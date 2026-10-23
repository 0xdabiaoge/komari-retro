package public_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/web/api"
	public "github.com/komari-monitor/komari/web/public"
	"github.com/komari-monitor/komari/web/router"
	"github.com/komari-monitor/komari/web/security"
)

func TestPrivateAudienceIsolation(t *testing.T) {
	t.Chdir(t.TempDir())
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = filepath.Join(t.TempDir(), "audiences.db")
	dbcore.GetDBInstance()
	t.Cleanup(func() { _ = dbcore.Close() })
	user, err := accounts.CreateAccount("audience-owner", "FixtureOwnerPassword12")
	if err != nil {
		t.Fatal(err)
	}
	session, err := accounts.CreateSession(user.UUID, 3600, "fixture", "127.0.0.1", "password")
	if err != nil {
		t.Fatal(err)
	}
	const entry = "/entry-audience-private-fixture"
	const share = "share-audience-private-fixture"
	const limited = "temporary-audience-private-fixture"
	const apiKey = "full-api-key-audience-private-fixture"
	const canary = "PRIVATE_SCRIPT_CANARY"
	if err := config.SetMany(map[string]any{"private_site": true, "admin_path": entry, "admin_view_path": "/view-audience-private-fixture", "permanent_share_token": share, "tempory_share_token": limited, "tempory_share_token_expire_at": time.Now().Add(time.Hour).Unix(), "api_key": apiKey, "api_key_scope": "full", "custom_head": "<script>" + canary + "()</script>", "custom_body": "<button>" + canary + "</button>"}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(security.ResponseHeaders(), api.RequestLimits(), api.IdentityMiddleware(), api.PrivateSiteMiddleware())
	router.Register(r)
	handler := api.ViewerHandler(r)
	request := func(method, path, cookie, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	viewer := "temp_key=" + share
	owner := "session_token=" + session
	entryCookie := "admin_entrance_token=" + entry
	refs := regexp.MustCompile(`(?:src|href)="([^" ]+)"`)
	blocked := regexp.MustCompile(`(?i)/api/(login|logout|me|admin)\b|/admin[/"'\x60?]|session_token|logged_in|LoginDialog|loginRedirect|登录|密码|登入|密碼`)
	for _, theme := range []string{"default", "retro"} {
		t.Run(theme, func(t *testing.T) {
			_ = config.Set(config.ThemeKey, theme)
			exchange := request("GET", "/s/"+share, "", "")
			if exchange.Code != 303 || exchange.Header().Get("Location") != "/shared" {
				t.Fatal("share URL was not cleaned")
			}
			page := request("GET", "/shared", viewer, "")
			if page.Code != 200 || !strings.Contains(page.Body.String(), "/share-assets/"+theme+"/") || strings.Contains(page.Body.String(), canary) {
				t.Fatalf("viewer document: %d", page.Code)
			}
			cookies := exchange.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Name != "temp_key" || !cookies[0].HttpOnly || cookies[0].MaxAge > 30*86400 {
				t.Fatal("viewer cookie flags")
			}
			if !strings.Contains(page.Header().Get("Content-Security-Policy"), "'strict-dynamic'") || !strings.Contains(page.Body.String(), "<script nonce=") {
				t.Fatal("viewer script policy missing")
			}
			for _, ref := range refs.FindAllStringSubmatch(page.Body.String(), -1) {
				if !strings.HasPrefix(ref[1], "/share-assets/") {
					continue
				}
				asset := request("GET", ref[1], viewer, "")
				if asset.Code != 200 || blocked.Match(asset.Body.Bytes()) || !strings.Contains(asset.Header().Get("Cache-Control"), "no-store") {
					t.Fatalf("viewer resource %s: %d", ref[1], asset.Code)
				}
				for _, invalid := range []string{"", "temp_key=forged"} {
					if request("GET", ref[1], invalid, "").Code != 404 {
						t.Fatal("viewer asset accepted without a valid share")
					}
				}
			}
			for _, cookie := range []string{"", viewer, "temp_key=forged", "admin_entrance_token=forged"} {
				for _, endpoint := range []string{"/admin", "/admin/login", "/admin/account", "/terminal", "/manage", "/index.html", "/themes/" + theme + "/dist/index.html", "/private-assets/" + theme + "/index.html", "/share-assets/" + theme + "/index.html", "/api/me", "/viewer-api/me"} {
					w := request("GET", endpoint, cookie, "")
					if w.Code != 404 {
						t.Fatalf("non-owner fetched %s: %d", endpoint, w.Code)
					}
				}
			}
			privateFiles := 0
			root := map[string]string{"default": "defaultTheme", "retro": "retroTheme"}[theme] + "/dist/"
			_ = fs.WalkDir(public.PublicFS, strings.TrimSuffix(root, "/"), func(name string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() || !(strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".txt")) {
					return nil
				}
				rel := strings.TrimPrefix(name, root)
				for _, prefix := range []string{"/private-assets/" + theme + "/", "/themes/" + theme + "/dist/", "/"} {
					w := request("GET", prefix+rel, viewer, "")
					if prefix == "/" && rel == "manifest.json" {
						expected, ok := public.ReadGuestThemeFile(theme, rel)
						if !ok || w.Code != 200 || !bytes.Equal(expected, w.Body.Bytes()) {
							t.Fatal("root manifest did not use viewer export")
						}
						continue
					}
					if w.Code != 404 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
						t.Fatalf("private resource leaked: %s (%d)", rel, w.Code)
					}
				}
				privateFiles++
				return nil
			})
			if privateFiles < 5 {
				t.Fatal("private resource enumeration empty")
			}
			for _, cookie := range []string{owner, entryCookie} {
				page := request("GET", "/admin/login", cookie, "")
				if page.Code != 200 || !strings.Contains(page.Body.String(), "/private-assets/default/") {
					t.Fatal("owner document failed")
				}
				for _, ref := range refs.FindAllStringSubmatch(page.Body.String(), -1) {
					if strings.HasPrefix(ref[1], "/private-assets/") && request("GET", ref[1], cookie, "").Code != 200 {
						t.Fatal("owner resource failed")
					}
				}
			}
			t.Logf("denied every %s private text resource: %d files x 3 paths", theme, privateFiles)
		})
	}
	for _, auth := range []string{viewer, viewer + "; " + owner + "; " + entryCookie} {
		for _, method := range []string{"admin:getSettings", "admin:exec", "admin:getClientToken", "public:getMe", "common:getMe"} {
			body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{}}`
			req := httptest.NewRequest("POST", "/viewer-api/rpc2?token=forged&Authorization=forged", bytes.NewBufferString(body))
			req.Header.Set("Cookie", auth)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			var response map[string]json.RawMessage
			_ = json.Unmarshal(w.Body.Bytes(), &response)
			if response["error"] == nil || response["result"] != nil {
				t.Fatalf("viewer inherited privileged identity: %s", method)
			}
		}
		for _, endpoint := range []string{"/viewer-api/public", "/api/public"} {
			w := request("GET", endpoint, viewer, "")
			if w.Code != 200 || strings.Contains(w.Body.String(), canary) || strings.Contains(w.Body.String(), entry) || strings.Contains(w.Body.String(), apiKey) {
				t.Fatal("public configuration leaked")
			}
		}
	}
	for _, endpoint := range []string{"/viewer-api/admin/settings", "/viewer-api/login", "/share-assets/retro/../default/../../private-assets/default/index.html", "/share-assets/retro/%2e%2e/retroTheme/dist/index.html"} {
		if w := request("GET", endpoint, viewer, ""); w.Code != 404 {
			t.Fatalf("alternate path accepted: %s %d", endpoint, w.Code)
		}
	}
	_ = config.Set(config.TemporyShareTokenExpireAtKey, time.Now().Add(-time.Minute).Unix())
	for _, endpoint := range []string{"/s/" + limited, "/viewer-api/public"} {
		if request("GET", endpoint, "temp_key="+limited, "").Code != 404 {
			t.Fatal("expired share accepted")
		}
	}
	_ = config.Set(config.PermanentShareTokenKey, "rotated")
	if request("GET", "/viewer-api/public", viewer, "").Code != 404 {
		t.Fatal("rotated share accepted")
	}
	_ = config.Set(config.PrivateSiteKey, "invalid-private-flag")
	for _, endpoint := range []string{"/", "/admin/login", "/api/public", "/private-assets/default/index.html"} {
		if request("GET", endpoint, "", "").Code != 404 {
			t.Fatal("invalid privacy configuration failed open")
		}
	}
}
