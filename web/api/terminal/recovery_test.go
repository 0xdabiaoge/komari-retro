package terminal

import (
	"bytes"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func socketPair(t *testing.T) (*connection.SafeConn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e == nil {
			accepted <- c
		}
	}))
	c, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if e != nil {
		t.Fatal(e)
	}
	server := <-accepted
	t.Cleanup(func() { _ = c.Close(); _ = server.Close(); srv.Close() })
	_ = c.SetReadDeadline(time.Now().Add(8 * time.Second))
	return connection.NewSafeConn(server), c
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); {
		TerminalSessionsMutex.Lock()
		ok := f()
		TerminalSessionsMutex.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("session transition timed out")
}
func fixtureSession(t *testing.T) (string, *TerminalSession, *websocket.Conn, *websocket.Conn) {
	t.Helper()
	b, bc := socketPair(t)
	a, ac := socketPair(t)
	id := t.Name()
	s := &TerminalSession{UUID: "node", UserUUID: "admin", OwnerToken: "credential", flow: true, Browser: b, Agent: a, Done: make(chan struct{}), changed: make(chan struct{})}
	TerminalSessionsMutex.Lock()
	TerminalSessions[id] = s
	TerminalSessionsMutex.Unlock()
	t.Cleanup(func() { closeTerminal(id) })
	go writeBrowser(id, s, b)
	go readBrowser(id, s, b)
	go ForwardTerminal(id)
	return id, s, bc, ac
}
func TestBrowserReconnectReplaysUnacknowledgedOutputAndKeepsAgent(t *testing.T) {
	id, s, b, a := fixtureSession(t)
	payload := []byte("你好\x1b[32mPTY state\x1b[0m")
	if e := a.WriteMessage(websocket.BinaryMessage, payload); e != nil {
		t.Fatal(e)
	}
	var first outputChunk
	if e := b.ReadJSON(&first); e != nil {
		t.Fatal(e)
	}
	if first.Seq != 1 || !bytes.Equal(first.Data, payload) {
		t.Fatal("output changed")
	}
	oldAgent := s.Agent
	_ = b.Close()
	eventually(t, func() bool { return s.Browser == nil })
	next, client := socketPair(t)
	TerminalSessionsMutex.Lock()
	if s.Agent != oldAgent {
		t.Fatal("browser disconnect killed Agent")
	}
	s.expiry.Stop()
	s.expiry = nil
	s.Browser = next
	s.sentSeq = s.ackedSeq
	signalLocked(s)
	TerminalSessionsMutex.Unlock()
	go readBrowser(id, s, next)
	go writeBrowser(id, s, next)
	var replay outputChunk
	if e := client.ReadJSON(&replay); e != nil {
		t.Fatal(e)
	}
	if replay.Seq != first.Seq || !bytes.Equal(replay.Data, payload) {
		t.Fatal("unacknowledged output not replayed")
	}
	_ = client.WriteJSON(map[string]any{"type": "ack", "seq": replay.Seq})
	eventually(t, func() bool { return s.outputBytes == 0 })
	_ = client.WriteMessage(websocket.BinaryMessage, []byte("echo preserved\r"))
	_, data, e := a.ReadMessage()
	if e != nil || string(data) != "echo preserved\r" {
		t.Fatalf("input forwarding: %q %v", data, e)
	}
	_ = client.WriteJSON(map[string]string{"type": "close"})
	select {
	case <-s.Done:
	case <-time.After(time.Second):
		t.Fatal("explicit close retained shell")
	}
}
func TestSlowBrowserUsesBoundedBackpressureWithoutDroppingOutput(t *testing.T) {
	_, s, b, a := fixtureSession(t)
	payload := bytes.Repeat([]byte("abcd"), 256*1024)
	sent := make(chan error, 1)
	go func() { sent <- a.WriteMessage(websocket.BinaryMessage, payload) }()
	time.Sleep(40 * time.Millisecond)
	TerminalSessionsMutex.Lock()
	n := s.outputBytes
	TerminalSessionsMutex.Unlock()
	if n > outputLimit {
		t.Fatal("unbounded queue")
	}
	var received []byte
	var seq uint64
	for len(received) < len(payload) {
		var c outputChunk
		if e := b.ReadJSON(&c); e != nil {
			t.Fatal(e)
		}
		if c.Seq != seq+1 {
			t.Fatal("output sequence gap")
		}
		seq = c.Seq
		received = append(received, c.Data...)
		_ = b.WriteJSON(map[string]any{"type": "ack", "seq": seq})
	}
	if !bytes.Equal(received, payload) {
		t.Fatal("output lost or reordered")
	}
	if e := <-sent; e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { return s.outputBytes == 0 })
}
func TestShellExitDrainsFinalOutputInsteadOfStartingAnotherShell(t *testing.T) {
	_, s, b, a := fixtureSession(t)
	_ = a.WriteMessage(websocket.BinaryMessage, []byte("last line"))
	_ = a.WriteJSON(map[string]string{"type": "exit"})
	var c outputChunk
	if e := b.ReadJSON(&c); e != nil {
		t.Fatal(e)
	}
	if string(c.Data) != "last line" {
		t.Fatal("last output lost")
	}
	_ = b.WriteJSON(map[string]any{"type": "ack", "seq": c.Seq})
	var closed struct{ Type string }
	if e := b.ReadJSON(&closed); e != nil {
		t.Fatal(e)
	}
	if closed.Type != "closed" {
		t.Fatal("missing exit state")
	}
	select {
	case <-s.Done:
	case <-time.After(time.Second):
		t.Fatal("exited session retained")
	}
}
func TestFutureAcknowledgementRevokesSession(t *testing.T) {
	_, s, b, _ := fixtureSession(t)
	_ = b.WriteJSON(map[string]any{"type": "ack", "seq": 1000})
	select {
	case <-s.Done:
	case <-time.After(time.Second):
		t.Fatal("forged acknowledgement accepted")
	}
}
func TestResumeIsBoundToNodeUserCredentialAndProtocol(t *testing.T) {
	s := &TerminalSession{UUID: "n", UserUUID: "u", OwnerToken: "secret", flow: true}
	if !canResume(s, "n", "u", "secret", true) {
		t.Fatal("legitimate resume rejected")
	}
	for _, c := range []struct {
		node, user, key string
		flow            bool
	}{{"other", "u", "secret", true}, {"n", "other", "secret", true}, {"n", "u", "other", true}, {"n", "u", "secret", false}} {
		if canResume(s, c.node, c.user, c.key, c.flow) {
			t.Fatal("cross-owner/protocol resume accepted")
		}
	}
	s.expiring = true
	if canResume(s, "n", "u", "secret", true) || canResume(nil, "n", "u", "secret", true) {
		t.Fatal("expired resume accepted")
	}
}
func TestRevocationUnblocksSaturatedOutputProducer(t *testing.T) {
	id := t.Name()
	s := &TerminalSession{Done: make(chan struct{}), changed: make(chan struct{})}
	TerminalSessionsMutex.Lock()
	TerminalSessions[id] = s
	TerminalSessionsMutex.Unlock()
	done := make(chan bool, 1)
	go func() { done <- enqueueOutput(id, s, make([]byte, outputLimit*2)) }()
	eventually(t, func() bool { return s.outputBytes == outputLimit })
	closeTerminal(id)
	select {
	case ok := <-done:
		if ok {
			t.Fatal("producer ignored revocation")
		}
	case <-time.After(time.Second):
		t.Fatal("producer leaked after revocation")
	}
}

func TestLegacyAgentDisconnectEndsSessionWithoutAutomaticNewShell(t *testing.T) {
	_, s, _, a := fixtureSession(t)
	_ = a.Close()
	select {
	case <-s.Done:
	case <-time.After(time.Second):
		t.Fatal("legacy Agent disconnect would auto-create a shell")
	}
}

func TestNewAgentDisconnectRetainsRecoverableSession(t *testing.T) {
	id, s, _, a := fixtureSession(t)
	_ = a.WriteJSON(map[string]interface{}{"type": "capabilities", "terminalResume": true})
	eventually(t, func() bool { return s.resumeCapable })
	_ = a.Close()
	eventually(t, func() bool { return s.Agent == nil })
	TerminalSessionsMutex.Lock()
	defer TerminalSessionsMutex.Unlock()
	if TerminalSessions[id] != s || s.shellExited {
		t.Fatal("recoverable Agent session lost")
	}
}
