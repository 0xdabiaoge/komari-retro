package api

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/share"
	"net/http"
	"sync"
	"time"
)

var viewerSocketPool = struct {
	sync.Mutex
	total   int
	sources map[string]int
	grants  map[string]int
}{sources: map[string]int{}, grants: map[string]int{}}

// Viewer admission is separate from Agent and administrator transports.
// Counts are removed at handler exit; rejected random keys cannot grow the maps.
func AdmitViewerSocket(c *gin.Context) (func(), bool) {
	if GetRole(c) != RoleGuest {
		return func() {}, true
	}
	grant := share.FromRequest(c.Request)
	private, err := config.GetAs[bool](config.PrivateSiteKey, false)
	if (private || err != nil) && grant == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return nil, false
	}
	source := c.ClientIP()
	key := "public"
	limit := 8
	if grant != nil {
		key = grant.ID
		limit = grant.MaxConnections
	}
	viewerSocketPool.Lock()
	if viewerSocketPool.total >= 128 || viewerSocketPool.sources[source] >= 8 || viewerSocketPool.grants[key] >= limit && grant != nil {
		viewerSocketPool.Unlock()
		c.Header("Retry-After", "10")
		c.AbortWithStatus(http.StatusTooManyRequests)
		return nil, false
	}
	viewerSocketPool.total++
	viewerSocketPool.sources[source]++
	viewerSocketPool.grants[key]++
	viewerSocketPool.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			viewerSocketPool.Lock()
			defer viewerSocketPool.Unlock()
			viewerSocketPool.total--
			viewerSocketPool.sources[source]--
			if viewerSocketPool.sources[source] == 0 {
				delete(viewerSocketPool.sources, source)
			}
			viewerSocketPool.grants[key]--
			if viewerSocketPool.grants[key] == 0 {
				delete(viewerSocketPool.grants, key)
			}
		})
	}, true
}

const ViewerSocketIdleTimeout = 90 * time.Second
