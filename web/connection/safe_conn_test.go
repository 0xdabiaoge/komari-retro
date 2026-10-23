package connection

import (
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCloseInterruptsEvenWhenWriterOwnsMutex(t *testing.T) {
	ch := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e == nil {
			ch <- c
		}
	}))
	defer server.Close()
	client, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	raw := <-ch
	defer raw.Close()
	c := NewSafeConn(raw)
	c.mu.Lock()
	defer c.mu.Unlock()
	done := make(chan struct{})
	go func() { c.TryWriteJSON(map[string]string{"type": "closed"}); _ = c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("revocation blocked behind writer mutex")
	}
}
