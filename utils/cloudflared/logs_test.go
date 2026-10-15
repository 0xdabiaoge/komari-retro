package cloudflared

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestChildProcessLogsAreBoundedAndRedactToken(t *testing.T) {
	m := &manager{maxLogLines: 3}
	const token = "isolated-sensitive-tunnel-token"
	m.scanPipe(io.NopCloser(strings.NewReader(strings.Repeat("error token="+token+"\n", 10))), true, token)
	if len(m.logs) != 3 {
		t.Fatalf("unbounded log history: %d", len(m.logs))
	}
	data, err := json.Marshal(m.status())
	if err != nil || strings.Contains(string(data), token) || !strings.Contains(string(data), "[redacted]") {
		t.Fatalf("token leaked in status: %s %v", data, err)
	}
	m.scanPipe(io.NopCloser(strings.NewReader(strings.Repeat("x", 5000)+"\n")), false, token)
	if len(m.logs[len(m.logs)-1]) > 4096 {
		t.Fatal("unbounded child-process log line")
	}
}
