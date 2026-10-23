package terminal

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"

	"github.com/komari-monitor/komari/web/connection"
)

type TerminalSession struct {
	UUID          string
	UserUUID      string
	Browser       *connection.SafeConn
	Agent         *connection.SafeConn
	RequesterIp   string
	Done          chan struct{}
	Authorized    func() bool
	OwnerToken    string
	flow          bool
	changed       chan struct{}
	expiry        *time.Timer
	lastDispatch  time.Time
	output        []outputChunk
	outputBytes   int
	nextSeq       uint64
	sentSeq       uint64
	ackedSeq      uint64
	shellExited   bool
	expiring      bool
	agentDeadline time.Time
	resumeCapable bool
}

var TerminalSessionsMutex = &sync.Mutex{}
var TerminalSessions = make(map[string]*TerminalSession)

func closeTerminal(id string) {
	TerminalSessionsMutex.Lock()
	session := TerminalSessions[id]
	delete(TerminalSessions, id)
	var browser, agent *connection.SafeConn
	if session != nil {
		if session.expiry != nil {
			session.expiry.Stop()
		}
		signalLocked(session)
		browser, agent = session.Browser, session.Agent
		if session.Done != nil {
			close(session.Done)
		}
	}
	TerminalSessionsMutex.Unlock()
	if browser != nil {
		browser.TryWriteJSON(map[string]string{"type": "closed", "message": "终端会话已结束，请新建连接。"})
		_ = browser.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Terminal session ended"), time.Now().Add(time.Second))
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
		agent.TryWriteJSON(map[string]string{"type": "close"})
		_ = agent.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Terminal permission revoked"), time.Now().Add(time.Second))
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
