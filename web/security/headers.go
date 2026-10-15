package security

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ResponseHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Secure entry/share capabilities occur in URLs. Never send those paths
		// to third-party images, fonts, previews or links through Referer.
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		// Cookie authority is ambient. Cross-origin mutations are never made
		// safe by disabling CORS or adding a frontend origin to its allowlist.
		if isAPIRequestPath(c.Request.URL.Path) && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			_, sessionErr := c.Cookie("session_token")
			_, entryErr := c.Cookie("admin_entrance_token")
			_, shareErr := c.Cookie("temp_key")
			if origin := c.GetHeader("Origin"); origin != "" && (sessionErr == nil || entryErr == nil || shareErr == nil) && !OriginMatchesHost(origin, c.Request.Host) {
				abortOrigin(c)
				return
			}
		}
		if c.GetHeader("Sec-Fetch-Site") == "cross-site" && c.Request.URL.Path == "/api/logout" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func abortOrigin(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Body != nil {
		c.Request.Close = true
		c.Header("Connection", "close")
	}
	c.AbortWithStatus(http.StatusForbidden)
}
