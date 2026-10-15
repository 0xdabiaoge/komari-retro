package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

var requestSlots = make(chan struct{}, 16)
var largeUploadSlots = make(chan struct{}, 2)

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
		if strings.HasPrefix(p, "/api/") && p != "/api/version" && !IsWebSocketUpgrade(c) {
			select {
			case requestSlots <- struct{}{}:
				defer func() { <-requestSlots }()
			default:
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "request capacity exceeded"})
				return
			}
			if limit > 2<<20 {
				select {
				case largeUploadSlots <- struct{}{}:
					defer func() { <-largeUploadSlots }()
				default:
					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "upload capacity exceeded"})
					return
				}
			}
		}
		// JSON/control messages cannot hold request slots indefinitely. Streaming
		// uploads keep their existing transfer cancellation/timeouts.
		if c.Request.Method != http.MethodGet && c.Request.Body != nil {
			controller := http.NewResponseController(c.Writer)
			_ = controller.SetReadDeadline(time.Now().Add(15 * time.Second))
			// Keep the bound through net/http's unread-body drain. The server
			// resets it for the next request; authorized streams clear it below.
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func allowAuthenticatedStream(c *gin.Context, role string) {
	p := c.Request.URL.Path
	if (role == RoleAdmin && (p == "/api/admin/upload/backup" || strings.HasPrefix(p, "/api/admin/theme/") || strings.Contains(p, "/file/upload"))) ||
		(role == RoleClient && strings.HasPrefix(p, "/api/clients/transfer/")) {
		_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
	}
}
