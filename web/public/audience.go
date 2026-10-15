package public

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"html"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/config"
)

func managementAudience(c *gin.Context) bool {
	if token, _ := c.Cookie("session_token"); token != "" {
		if _, err := accounts.GetUserBySession(token); err == nil {
			return true
		}
	}
	entry, _ := config.GetAs[string](config.AdminPathKey, "")
	entry = strings.TrimSpace(entry)
	if entry != "" && !strings.HasPrefix(entry, "/") {
		entry = "/" + entry
	}
	cookie, _ := c.Cookie("admin_entrance_token")
	return entry != "" && subtle.ConstantTimeCompare([]byte(cookie), []byte(entry)) == 1
}

func privateSite() bool {
	value, err := config.GetAs[bool](config.PrivateSiteKey, false)
	return err != nil || value
}

func audienceHeaders(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Vary", "Cookie")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
}

func guestThemeID(theme string) string {
	if strings.EqualFold(theme, RetroTheme) {
		return RetroTheme
	}
	return DefaultTheme
}

// Only read separately compiled viewer files. Never fall back to privileged FS.
func ReadGuestThemeFile(theme, relative string) ([]byte, bool) {
	if !IsBuiltinTheme(theme) || !fs.ValidPath(relative) || strings.Contains(relative, "\\") {
		return nil, false
	}
	content, err := fs.ReadFile(GuestFS, path.Join("guest", theme, "dist", relative))
	return content, err == nil
}

func serveGuestIndex(c *gin.Context, theme string) {
	audienceHeaders(c)
	content, ok := ReadGuestThemeFile(guestThemeID(theme), "index.html")
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	// Viewer documents never execute site custom_head/custom_body or plugin code.
	name, _ := config.GetAs[string](config.SitenameKey, "Komari")
	text := strings.ReplaceAll(string(content), "<title>Komari Monitor</title>", "<title>"+html.EscapeString(name)+"</title>")
	text = strings.ReplaceAll(text, "<title>Komari Retro</title>", "<title>"+html.EscapeString(name)+"</title>")
	text = strings.Replace(text, "</head>", `<script defer src="/viewer-bootstrap.js"></script></head>`, 1)
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	nonce := base64.RawStdEncoding.EncodeToString(random[:])
	text = regexp.MustCompile(`<script\b`).ReplaceAllString(text, `<script nonce="`+nonce+`"`)
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'nonce-"+nonce+"' 'strict-dynamic'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	if language, err := c.Cookie(LanguageCookieName); err == nil {
		text = replaceHTMLLanguage(text, language)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(text))
}

func registerAudienceAssets(r *gin.RouterGroup) {
	r.GET("/share-assets/:theme/*file", func(c *gin.Context) {
		audienceHeaders(c)
		if privateSite() && !hasValidTempKey(c) && !managementAudience(c) {
			c.Status(http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(c.Param("file"), "/")
		if strings.HasSuffix(rel, ".html") {
			c.Status(http.StatusNotFound)
			return
		}
		content, ok := ReadGuestThemeFile(c.Param("theme"), rel)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, getFallbackMimeType(rel), content)
	})
	r.GET("/private-assets/:theme/*file", func(c *gin.Context) {
		audienceHeaders(c)
		if privateSite() && !managementAudience(c) {
			c.Status(http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(c.Param("file"), "/")
		if !IsBuiltinTheme(c.Param("theme")) || !fs.ValidPath(rel) || strings.Contains(rel, "\\") || strings.HasSuffix(rel, ".map") {
			c.Status(http.StatusNotFound)
			return
		}
		content, ok := ReadBuiltinThemeFile(c.Param("theme"), path.Join("dist", rel))
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, getFallbackMimeType(rel), content)
	})
	// Replace old origin-wide precaching workers with a self-retiring worker.
	r.GET("/sw.js", func(c *gin.Context) {
		audienceHeaders(c)
		c.Header("Service-Worker-Allowed", "/")
		c.Data(200, "application/javascript; charset=utf-8", []byte(`self.addEventListener('install',()=>self.skipWaiting());self.addEventListener('activate',event=>event.waitUntil((async()=>{for(const key of await caches.keys())if(key.startsWith('workbox-'))await caches.delete(key);await self.registration.unregister();await self.clients.claim();})()));`))
	})
	r.GET("/viewer-bootstrap.js", func(c *gin.Context) {
		audienceHeaders(c)
		c.Data(200, "application/javascript; charset=utf-8", []byte(`if('serviceWorker'in navigator)navigator.serviceWorker.getRegistrations().then(list=>Promise.all(list.filter(item=>item.scope===location.origin+'/').map(item=>item.unregister()))).catch(()=>{});if('caches'in window)caches.keys().then(list=>Promise.all(list.filter(key=>key.startsWith('workbox-')).map(key=>caches.delete(key)))).catch(()=>{});`))
	})
}

func issueViewerCookie(c *gin.Context, token string) {
	permanent, _ := config.GetAs[string](config.PermanentShareTokenKey, "")
	if token != "" && token == permanent {
		setAccessCookie(c, "temp_key", token, 30*86400, true)
		return
	}
	expires, _ := config.GetAs[int64](config.TemporyShareTokenExpireAtKey, 0)
	if remaining := int(expires - time.Now().Unix()); remaining > 0 {
		setAccessCookie(c, "temp_key", token, remaining, true)
	}
}
