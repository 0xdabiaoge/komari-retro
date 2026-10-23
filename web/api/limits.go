package api

import (
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/pkg/config"
	"net/http"
	"strings"
	"time"
)

var requestSlots = make(chan struct{}, 16)
var adminRequestSlots = make(chan struct{}, 8)
var agentRequestSlots = make(chan struct{}, 32)
var agentRequestWaiters = make(chan struct{}, 64)
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
			// A reporting burst must not consume the administrator's capacity.
			// The path selects a bounded admission pool, never authentication.
			role := capacityRole(c)
			agentReport := role == RoleClient
			slots := requestSlots
			if role == RoleAdmin {
				slots = adminRequestSlots
			}
			if agentReport {
				slots = agentRequestSlots
			}
			select {
			case slots <- struct{}{}:
			default:
				if agentReport {
					select {
					case agentRequestWaiters <- struct{}{}:
					default:
						rejectCapacity(c)
						return
					}
					timer := time.NewTimer(time.Second)
					acquired := false
					select {
					case slots <- struct{}{}:
						acquired = true
					case <-c.Request.Context().Done():
					case <-timer.C:
					}
					timer.Stop()
					<-agentRequestWaiters
					if !acquired {
						rejectCapacity(c)
						return
					}
				} else {
					c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "request capacity exceeded"})
					return
				}
			}
			defer func() { <-slots }()
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

// Verify only header/query credentials here: legacy JSON tokens must still be
// read inside the bounded guest pool. Never trust a cookie's mere presence.
func capacityRole(c *gin.Context) string {
	if GetRole(c) == RoleClient {
		return RoleClient
	}
	token := c.Query("token")
	if token == "" {
		token = c.Query("Authorization")
	}
	if token == "" {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if token == "" {
		token = c.GetHeader("X-Client-Token")
	}
	clientValid := false
	if token != "" {
		_, err := clients.GetClientUUIDByToken(token)
		clientValid = err == nil
	}
	// Protocol submissions may also carry a browser's owner cookie (for example
	// an authenticated workbench or acceptance client). A verified Agent token
	// on the dedicated Agent transport selects Agent capacity, never admin rights.
	if strings.HasPrefix(c.Request.URL.Path, "/api/clients/") && clientValid {
		return RoleClient
	}
	if GetRole(c) == RoleAdmin {
		return RoleAdmin
	}
	if header := c.GetHeader("Authorization"); header != "" && isApiKeyValid(header) {
		return RoleAdmin
	}
	if token, _ := c.Cookie("session_token"); token != "" {
		if _, err := accounts.GetUserBySession(token); err == nil {
			return RoleAdmin
		}
	}
	if token, _ := c.Cookie("admin_entrance_token"); token != "" {
		entry, _ := config.GetAs[string](config.AdminPathKey)
		if entry != "" && subtle.ConstantTimeCompare([]byte(token), []byte("/"+strings.Trim(entry, "/"))) == 1 {
			return RoleAdmin
		}
	}
	if clientValid {
		return RoleClient
	}
	return RoleGuest
}

func rejectCapacity(c *gin.Context) {
	c.Header("Retry-After", "1")
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "request capacity exceeded"})
}

func allowAuthenticatedStream(c *gin.Context, role string) {
	p := c.Request.URL.Path
	if (role == RoleAdmin && (p == "/api/admin/upload/backup" || strings.HasPrefix(p, "/api/admin/theme/") || strings.Contains(p, "/file/upload"))) ||
		(role == RoleClient && strings.HasPrefix(p, "/api/clients/transfer/")) {
		_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
	}
}
