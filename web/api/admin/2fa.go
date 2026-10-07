package admin

import (
	"image/png"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/web/api"
	"github.com/pquerna/otp/totp"
)

type pendingFactor struct {
	Secret, Session, Previous string
	Expires                   time.Time
}

var factorMu sync.Mutex
var pendingFactors = make(map[string]pendingFactor)

func Generate2FA(c *gin.Context) {
	if c.GetString("session") == "" {
		api.RespondError(c, 401, "A user session is required")
		return
	}
	if err := api.VerifySensitive2FACode(c); err != nil {
		api.RespondError(c, 401, err.Error())
		return
	}
	secret, img, err := accounts.Generate2Fa()
	if err != nil {
		api.RespondError(c, 500, "Failed to generate 2FA: "+err.Error())
		return
	}
	factorMu.Lock()
	for uuid, entry := range pendingFactors {
		if time.Now().After(entry.Expires) {
			delete(pendingFactors, uuid)
		}
	}
	user, err := accounts.GetUserByUUID(c.GetString("uuid"))
	if err != nil {
		factorMu.Unlock()
		api.RespondError(c, 500, "Account unavailable")
		return
	}
	pendingFactors[c.GetString("uuid")] = pendingFactor{secret, c.GetString("session"), user.TwoFactor, time.Now().Add(10 * time.Minute)}
	factorMu.Unlock()
	c.Header("Content-Type", "image/png")
	c.Writer.WriteHeader(200)
	png.Encode(c.Writer, img)
}

func Enable2FA(c *gin.Context) {
	uuid := c.GetString("uuid")
	factorMu.Lock()
	defer factorMu.Unlock()
	pending, ok := pendingFactors[uuid]
	secret := pending.Secret
	code := c.Query("code")
	if !ok || time.Now().After(pending.Expires) || pending.Session != c.GetString("session") || secret == "" || uuid == "" || code == "" {
		api.RespondError(c, 400, "2FA secret or code not provided")
		return
	}
	if !totp.Validate(code, secret) {
		api.RespondError(c, 400, "Invalid 2FA code")
		return
	}
	// Generating a replacement was already authorized using the old factor.
	user, getErr := accounts.GetUserByUUID(uuid)
	if getErr != nil || user.TwoFactor != pending.Previous {
		api.RespondError(c, 400, "2FA configuration changed; generate a new secret")
		return
	}
	err := accounts.Enable2Fa(uuid, secret)
	if err != nil {
		api.RespondError(c, 500, "Failed to enable 2FA: "+err.Error())
		return
	}
	delete(pendingFactors, uuid)

	api.RespondSuccess(c, "2FA enabled successfully")
}

func Disable2FA(c *gin.Context) {
	if c.GetString("session") == "" {
		api.RespondError(c, 401, "A user session is required")
		return
	}
	if err := api.VerifySensitive2FACode(c); err != nil {
		api.RespondError(c, 401, err.Error())
		return
	}
	uuid, _ := c.Get("uuid")
	err := accounts.Disable2Fa(uuid.(string))
	if err != nil {
		api.RespondError(c, 500, "Failed to disable 2FA: "+err.Error())
		return
	}
	api.RespondSuccess(c, "")
}
