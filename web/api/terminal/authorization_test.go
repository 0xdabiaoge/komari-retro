package terminal

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRevokedTerminalAuthorizationClosesSession(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	session := &TerminalSession{Done: make(chan struct{}), Authorized: allowed.Load}
	TerminalSessionsMutex.Lock()
	TerminalSessions["revocation-fixture"] = session
	TerminalSessionsMutex.Unlock()
	t.Cleanup(func() { closeTerminal("revocation-fixture") })
	go watchAuthorization("revocation-fixture", session, time.Millisecond)
	allowed.Store(false)
	select {
	case <-session.Done:
	case <-time.After(time.Second):
		t.Fatal("revoked terminal was retained")
	}
	TerminalSessionsMutex.Lock()
	_, exists := TerminalSessions["revocation-fixture"]
	TerminalSessionsMutex.Unlock()
	if exists {
		t.Fatal("revoked terminal remains registered")
	}
}
