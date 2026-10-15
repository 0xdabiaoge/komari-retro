package filetransfer

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPreviewTokenCapacityUnderConcurrentIssuance(t *testing.T) {
	previewTokensMu.Lock()
	previous := previewTokens
	previewTokens = make(map[string]previewTokenEntry)
	for i := 0; i < 1020; i++ {
		previewTokens[randomHex(24)] = previewTokenEntry{}
	}
	previewTokensMu.Unlock()
	t.Cleanup(func() { previewTokensMu.Lock(); previewTokens = previous; previewTokensMu.Unlock() })
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?path=/tmp/fixture", nil)
			c.Params = gin.Params{{Key: "uuid", Value: "fixture"}}
			HandleFilePreviewToken(c)
		}()
	}
	wg.Wait()
	previewTokensMu.Lock()
	count := len(previewTokens)
	previewTokensMu.Unlock()
	if count != 1024 {
		t.Fatalf("preview token quota violated: %d", count)
	}
}
