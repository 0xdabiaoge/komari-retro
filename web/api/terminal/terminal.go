package terminal

import (
	"sync"

	"github.com/komari-monitor/komari/web/connection"
)

type TerminalSession struct {
	UUID        string
	UserUUID    string
	Browser     *connection.SafeConn
	Agent       *connection.SafeConn
	RequesterIp string
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
	}
	TerminalSessionsMutex.Unlock()
	if browser != nil {
		_ = browser.Close()
	}
	if agent != nil {
		_ = agent.Close()
	}
}
