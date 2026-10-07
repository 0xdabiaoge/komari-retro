package filetransfer

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type countingResponse struct {
	headers http.Header
	status  int
	bytes   int64
}

func (w *countingResponse) Header() http.Header  { return w.headers }
func (w *countingResponse) WriteHeader(code int) { w.status = code }
func (w *countingResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	w.bytes += int64(len(p))
	return len(p), nil
}

func TestDownloadStreamsLargeEmptyAndRanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := executeFileOperation
	defer func() { executeFileOperation = original }()
	for _, tc := range []struct {
		name        string
		size        int64
		rangeHeader string
		want        int64
		status      int
	}{
		{"large", 256 << 20, "", 256 << 20, 200}, {"empty", 0, "", 0, 200}, {"range", 256 << 20, "bytes=128-255", 128, 206},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := make(chan bool, 1)
			executeFileOperation = func(ctx context.Context, uuid, op string, args map[string]interface{}) (json.RawMessage, error) {
				if op == "stat" {
					return json.Marshal(remoteFileInfo{Name: "file.bin", Path: "/file.bin", Size: tc.size})
				}
				if op != "download_stream" {
					t.Errorf("unexpected operation %s", op)
					return nil, nil
				}
				called <- true
				bridge := getBridge(args["transfer_id"].(string))
				bridge.DataStream <- io.NopCloser(io.LimitReader(zeroReader{}, args["length"].(int64)))
				select {
				case <-bridge.Done:
				case <-ctx.Done():
				}
				return json.RawMessage(`{}`), nil
			}
			response := &countingResponse{headers: make(http.Header)}
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequest("GET", "/download", nil)
			c.Request.Header.Set("Range", tc.rangeHeader)
			streamDownload(c, "node", "/file.bin", false, "")
			c.Writer.WriteHeaderNow()
			if response.status != tc.status || response.bytes != tc.want {
				t.Fatalf("status=%d bytes=%d", response.status, response.bytes)
			}
			if tc.size == 0 {
				select {
				case <-called:
					t.Fatal("empty file dispatched stream")
				default:
				}
			}
			bridgesMu.RLock()
			remaining := len(transferBridges)
			bridgesMu.RUnlock()
			if remaining != 0 {
				t.Fatal("bridge leaked")
			}
		})
	}
}
