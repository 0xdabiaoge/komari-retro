package public

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessCookieSecurity(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		for _, admin := range []bool{false, true} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, scheme+"://fixture.invalid/", nil)
			setAccessCookie(c, "fixture_access", "synthetic", 600, admin)
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal("missing cookie")
			}
			cookie := cookies[0]
			if cookie.Secure != (scheme == "https") || cookie.HttpOnly != admin || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 600 {
				t.Fatalf("unexpected access cookie flags: %+v", cookie)
			}
		}
	}
}
