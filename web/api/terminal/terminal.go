package terminal

import (
	"sync"
	"time"

	"github.com/komari-monitor/komari/web/connection"
)

type TerminalSession struct {
	UUID        string
	UserUUID    string
	Browser     *connection.SafeConn
	Agent       *connection.SafeConn
	RequesterIp string
	Done        chan struct{}
	Authorized  func() bool
}

var TerminalSessionsMutex = &sync.Mutex{}
var TerminalSessions = make(map[string]*TerminalSession)

func closeTerminal(id string) {
	TerminalSessionsMutex.Lock()
	session := TerminalSessions[id]
	delete(TerminalSessions, id)
	var browser, agent *connection.SafeConn
	if session != nil {
		browser, agent = session.Browser, session.Agent
		if session.Done != nil {
			close(session.Done)
		}
	}
	TerminalSessionsMutex.Unlock()
	if browser != nil {
		_ = browser.Close()
	}
	if agent != nil {
		_ = agent.Close()
	}
}

func revokeTerminal(id string, session *TerminalSession) {
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != session {
		TerminalSessionsMutex.Unlock()
		return
	}
	agent := session.Agent
	TerminalSessionsMutex.Unlock()
	// Explicitly terminate the retained PTY instead of retaining it as though
	// this were only a transient network disconnect.
	if agent != nil {
		_ = agent.WriteJSON(map[string]string{"type": "close"})
	}
	closeTerminal(id)
}

func watchAuthorization(id string, session *TerminalSession, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-session.Done:
			return
		case <-ticker.C:
			if session.Authorized != nil && !session.Authorized() {
				revokeTerminal(id, session)
				return
			}
		}
	}
}
