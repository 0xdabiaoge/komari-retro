package terminal

import (
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/web/connection"
	"time"
)

func ForwardTerminal(id string) {
	TerminalSessionsMutex.Lock()
	session := TerminalSessions[id]
	if session == nil || session.Agent == nil || session.Browser == nil {
		TerminalSessionsMutex.Unlock()
		return
	}
	browser, agent := session.Browser, session.Agent
	TerminalSessionsMutex.Unlock()
	auditlog.Log(session.RequesterIp, session.UserUUID, "established terminal", "terminal")
	started := time.Now()
	done := make(chan struct{}, 2)
	copyMessages := func(dst, src *connection.SafeConn) {
		defer func() { done <- struct{}{} }()
		for {
			kind, data, err := src.ReadMessage()
			if err != nil {
				return
			}
			if err = dst.WriteMessage(kind, data); err != nil {
				return
			}
		}
	}
	go copyMessages(agent, browser)
	go copyMessages(browser, agent)
	<-done
	closeTerminal(id)
	<-done
	auditlog.Log(session.RequesterIp, session.UserUUID, "terminal disconnected, duration:"+time.Since(started).String(), "terminal")
}
