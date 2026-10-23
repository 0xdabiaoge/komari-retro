package terminal

import (
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type retainedTestPTY struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	inputs chan string
	once   sync.Once
}

func (p *retainedTestPTY) Read(b []byte) (int, error)  { return p.reader.Read(b) }
func (p *retainedTestPTY) Write(b []byte) (int, error) { p.inputs <- string(b); return len(b), nil }
func (p *retainedTestPTY) Resize(int, int) error       { return nil }
func (p *retainedTestPTY) Wait() error                 { return nil }
func (p *retainedTestPTY) Close() error {
	p.once.Do(func() { _ = p.reader.Close(); _ = p.writer.Close() })
	return nil
}

func TestRetainedPTYWaitsForReattachmentInsteadOfDiscardingOutput(t *testing.T) {
	r, w := io.Pipe()
	pty := &retainedTestPTY{reader: r, writer: w, inputs: make(chan string, 32)}
	id := t.Name()
	s := &terminalSession{id: id, term: pty, changed: make(chan struct{})}
	sessionsMu.Lock()
	sessions[id] = s
	sessionsMu.Unlock()
	defer closeSession(id, s)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e == nil {
			StartTerminal(c, id)
		}
	}))
	defer srv.Close()
	go s.readOutput()
	dial := func() *websocket.Conn {
		c, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if e != nil {
			t.Fatal(e)
		}
		_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
		var hello struct {
			Type           string
			TerminalResume bool
		}
		if err := c.ReadJSON(&hello); err != nil || hello.Type != "capabilities" || !hello.TerminalResume {
			t.Fatalf("capability handshake: %+v %v", hello, err)
		}
		return c
	}
	first := dial()
	_ = first.WriteMessage(websocket.BinaryMessage, []byte("first input"))
	if got := <-pty.inputs; got != "first input" {
		t.Fatal(got)
	}
	_ = first.Close()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		detached := s.conn == nil
		s.mu.Unlock()
		if detached {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not detached")
		}
		time.Sleep(time.Millisecond)
	}
	if _, e := w.Write([]byte("output while disconnected")); e != nil {
		t.Fatal(e)
	}
	second := dial()
	defer second.Close()
	_, data, e := second.ReadMessage()
	if e != nil || string(data) != "output while disconnected" {
		t.Fatalf("retained output %q: %v", data, e)
	}
	_ = second.WriteMessage(websocket.BinaryMessage, []byte("same PTY"))
	if got := <-pty.inputs; got != "same PTY" {
		t.Fatal(got)
	}
	_ = second.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "permission revoked"), time.Now().Add(time.Second))
	for end := time.Now().Add(time.Second); ; {
		sessionsMu.Lock()
		remaining := sessions[id] != nil
		sessionsMu.Unlock()
		if !remaining {
			break
		}
		if time.Now().After(end) {
			t.Fatal("revocation close frame left PTY alive")
		}
		time.Sleep(time.Millisecond)
	}
}
