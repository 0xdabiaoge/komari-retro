package security

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCapabilitiesDoNotLeakViaReferrerAndCrossSiteLogoutIsDenied(t *testing.T) {
	r := gin.New()
	r.Use(ResponseHeaders())
	r.GET("/api/logout", func(c *gin.Context) { c.Status(204) })
	for _, tt := range []struct {
		site   string
		status int
	}{{"cross-site", 403}, {"same-origin", 204}, {"none", 204}} {
		req := httptest.NewRequest("GET", "/api/logout", nil)
		req.Header.Set("Sec-Fetch-Site", tt.site)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tt.status || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("unexpected response for %s: %d %v", tt.site, w.Code, w.Header())
		}
	}
}
