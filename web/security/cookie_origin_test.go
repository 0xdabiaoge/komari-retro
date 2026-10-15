package security

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCookieMutationOriginIndependentFromCORS(t *testing.T) {
	r := gin.New()
	r.Use(ResponseHeaders(), CorsMiddleware(false, "*"))
	r.POST("/api/mutate", func(c *gin.Context) { c.Status(200) })
	for _, test := range []struct {
		origin, cookie string
		code           int
	}{
		{"https://foreign.invalid", "session_token=synthetic", 403},
		{"https://foreign.invalid", "admin_entrance_token=synthetic", 403},
		{"https://foreign.invalid", "temp_key=synthetic", 403},
		{"null", "session_token=synthetic", 403},
		{"http://panel.invalid", "session_token=synthetic", 200},
		{"", "session_token=synthetic", 200},
		{"https://foreign.invalid", "", 200},
	} {
		req := httptest.NewRequest("POST", "http://panel.invalid/api/mutate", strings.NewReader("{}"))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Cookie", test.cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != test.code {
			t.Fatalf("origin=%s got=%d", test.origin, w.Code)
		}
		if test.code == 403 && (!req.Close || w.Header().Get("Connection") != "close") {
			t.Fatal("rejected body was left open")
		}
	}
}
