package api

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type observedBody struct{ reads int }

func (b *observedBody) Read(p []byte) (int, error) { b.reads++; return 0, io.EOF }
func (*observedBody) Close() error                 { return nil }

func TestClientIdentityDoesNotBufferUnrelatedRequestBodies(t *testing.T) {
	for _, path := range []string{"/api/login", "/api/rpc2", "/api/admin/upload/backup", "/api/admin/theme/upload", "/api/clients/transfer/id", "/api/admin/client/id/file/upload?operation=chunk"} {
		t.Run(path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", path, nil)
			body := &observedBody{}
			c.Request.Body = body
			c.Request.Header.Set("Content-Type", "application/json")
			if token := extractClientToken(c); token != "" || body.reads != 0 {
				t.Fatalf("identity middleware read a non-report body: reads=%d", body.reads)
			}
		})
	}
}

func TestLegacyAgentBodyTokenStillWorks(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/clients/report", strings.NewReader(`{"token":"fixture-token","cpu":1}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if extractClientToken(c) != "fixture-token" {
		t.Fatal("legacy report token was lost")
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || !strings.Contains(string(body), `"cpu":1`) {
		t.Fatal("report body was not restored")
	}
}
