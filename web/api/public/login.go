package public

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/api"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TwoFa    string `json:"2fa_code"`
}

const sessionCookieMaxAge = 2592000

type loginAttemptInfo struct {
	count       int
	lockedUntil time.Time
	lastAttempt time.Time
}

var (
	loginAttemptsMu sync.Mutex
	loginAttempts   = make(map[string]*loginAttemptInfo)
	lastLoginPrune  time.Time
)

func checkLoginRateLimit(ip string) error {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	pruneLoginAttemptsLocked(time.Now())

	info, exists := loginAttempts[ip]
	if !exists {
		if len(loginAttempts) >= 10000 {
			return fmt.Errorf("Login rate-limit capacity exceeded; try again later")
		}
		return nil
	}
	if time.Now().Before(info.lockedUntil) {
		remainSec := int(time.Until(info.lockedUntil).Seconds())
		return fmt.Errorf("Too many failed login attempts. Please try again after %d seconds", remainSec)
	}
	if time.Since(info.lastAttempt) > 15*time.Minute {
		delete(loginAttempts, ip)
	}
	return nil
}

func recordLoginFailure(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	pruneLoginAttemptsLocked(time.Now())

	info, exists := loginAttempts[ip]
	if !exists {
		if len(loginAttempts) >= 10000 {
			return
		}
		info = &loginAttemptInfo{}
		loginAttempts[ip] = info
	}
	info.count++
	info.lastAttempt = time.Now()
	if info.count >= 5 {
		info.lockedUntil = time.Now().Add(15 * time.Minute)
	}
}

func recordLoginSuccess(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	delete(loginAttempts, ip)
}

func pruneLoginAttemptsLocked(now time.Time) {
	if now.Sub(lastLoginPrune) <= time.Minute {
		return
	}
	for key, entry := range loginAttempts {
		if now.Sub(entry.lastAttempt) > 15*time.Minute && now.After(entry.lockedUntil) {
			delete(loginAttempts, key)
		}
	}
	lastLoginPrune = now
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session_token",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   utils.GetScheme(c) == "https",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func Login(c *gin.Context) {
	clientIP := c.ClientIP()
	if err := checkLoginRateLimit(clientIP); err != nil {
		api.RespondError(c, http.StatusTooManyRequests, err.Error())
		return
	}

	DisablePasswordLogin, _ := config.GetAs[bool](config.DisablePasswordLoginKey, false)
	if DisablePasswordLogin {
		api.RespondError(c, http.StatusForbidden, "Password login is disabled")
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	var data LoginRequest
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if data.Username == "" || data.Password == "" {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: Username and password are required")
		return
	}
	if len(data.Username) > 255 || len(data.Password) > 4096 || len(data.TwoFa) > 64 {
		api.RespondError(c, 400, "Invalid credential length")
		return
	}
	accountHash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(data.Username))))
	accountKey := fmt.Sprintf("account:%x", accountHash)
	if err := checkLoginRateLimit(accountKey); err != nil {
		api.RespondError(c, 429, err.Error())
		return
	}

	uuid, success := accounts.CheckPassword(data.Username, data.Password)
	if !success {
		recordLoginFailure(clientIP)
		recordLoginFailure(accountKey)
		api.RespondError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	// 2FA
	user, _ := accounts.GetUserByUUID(uuid)
	if user.TwoFactor != "" { // 开启了2FA
		if data.TwoFa == "" {
			recordLoginFailure(clientIP)
			recordLoginFailure(accountKey)
			api.RespondError(c, http.StatusUnauthorized, "2FA code is required")
			return
		}
		if ok, err := accounts.Verify2Fa(uuid, data.TwoFa); err != nil || !ok {
			recordLoginFailure(clientIP)
			recordLoginFailure(accountKey)
			api.RespondError(c, http.StatusUnauthorized, "Invalid 2FA code")
			return
		}
	}
	recordLoginSuccess(clientIP)
	recordLoginSuccess(accountKey)
	// Create session
	session, err := accounts.CreateSession(uuid, sessionCookieMaxAge, c.Request.UserAgent(), c.ClientIP(), "password")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}
	setSessionCookie(c, session, sessionCookieMaxAge)
	auditlog.Log(c.ClientIP(), uuid, "logged in (password)", "login")
	api.RespondSuccess(c, gin.H{"set-cookie": gin.H{"session_token": session}})
}
func Logout(c *gin.Context) {
	session, _ := c.Cookie("session_token")
	accounts.DeleteSession(session)
	setSessionCookie(c, "", -1)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	c.Redirect(302, "/")
}
