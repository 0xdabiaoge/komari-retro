package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// Bound uploads before any authentication or JSON middleware reads their body.
func RequestLimits() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := int64(2 << 20)
		p := c.Request.URL.Path
		switch {
		case p == "/api/admin/upload/backup":
			limit = 1 << 30
		case strings.HasPrefix(p, "/api/admin/theme/"):
			limit = 50 << 20
		case strings.Contains(p, "/file/upload") && c.Query("operation") == "chunk":
			limit = 128 << 20
		case strings.HasPrefix(p, "/api/clients/transfer/"):
			limit = 1 << 40 // streamed; never read into memory
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(413, gin.H{"error": "request body too large"})
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
