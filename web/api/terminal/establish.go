package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
)

func EstablishConnection(c *gin.Context) {
	id := c.Query("id")
	TerminalSessionsMutex.Lock()
	session := TerminalSessions[id]
	valid := session != nil && session.Browser != nil && session.Agent == nil && c.GetString("client_uuid") == session.UUID
	TerminalSessionsMutex.Unlock()
	if !valid {
		c.JSON(http.StatusConflict, gin.H{"error": "Terminal session unavailable or client mismatch"})
		return
	}
	if !api.IsWebSocketUpgrade(c) {
		c.Status(http.StatusBadRequest)
		return
	}
	raw, err := api.UpgradeWebSocket(c)
	if err != nil {
		return
	}
	raw.SetReadLimit(1 << 20)
	raw.SetCloseHandler(func(int, string) error { closeTerminal(id); return nil })
	agent := connection.NewSafeConn(raw)
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != session || session.Agent != nil {
		TerminalSessionsMutex.Unlock()
		agent.Close()
		return
	}
	session.Agent = agent
	TerminalSessionsMutex.Unlock()
	go ForwardTerminal(id)
}
