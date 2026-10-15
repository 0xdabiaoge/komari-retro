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
		if c.GetHeader("Sec-Fetch-Site") == "cross-site" && c.Request.URL.Path == "/api/logout" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
