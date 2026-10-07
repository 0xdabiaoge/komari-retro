package terminal

import (
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/clients"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/utils"
	agentRuntime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
	"net/http"
	"time"
)

func RequestTerminal(c *gin.Context) {
	uuid := c.Param("uuid")
	if _, err := clients.GetClientByUUID(uuid); err != nil {
		c.JSON(404, gin.H{"error": "Client not found"})
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
	id := utils.GenerateRandomString(32)
	raw.SetReadLimit(1 << 20)
	raw.SetCloseHandler(func(int, string) error { closeTerminal(id); return nil })
	browser := connection.NewSafeConn(raw)
	session := &TerminalSession{UUID: uuid, UserUUID: c.GetString("uuid"), Browser: browser, RequesterIp: c.ClientIP()}
	TerminalSessionsMutex.Lock()
	TerminalSessions[id] = session
	TerminalSessionsMutex.Unlock()
	if !agentRuntime.IsAgentOnline(uuid) {
		browser.WriteMessage(1, []byte("Client offline\n"))
		closeTerminal(id)
		return
	}
	browser.WriteMessage(1, []byte("等待被控端连接 waiting for agent...\n"))
	dispatched := false
	client := agentRuntime.GetConnectedClients()[uuid]
	if client != nil && !agentRuntime.IsV2Client(uuid) {
		dispatched = client.WriteJSON(gin.H{"message": "terminal", "request_id": id}) == nil
	} else {
		dispatched = agentRuntime.DispatchV2Event(uuid, v2.MethodAgentTerminal, map[string]string{"request_id": id})
	}
	if !dispatched {
		browser.WriteMessage(1, []byte("Failed to dispatch terminal request\n"))
		closeTerminal(id)
		return
	}
	time.AfterFunc(30*time.Second, func() {
		TerminalSessionsMutex.Lock()
		waiting := TerminalSessions[id] == session && session.Agent == nil
		if waiting {
			delete(TerminalSessions, id)
		}
		TerminalSessionsMutex.Unlock()
		if waiting {
			browser.WriteMessage(1, []byte("被控端连接超时 timeout\n"))
			browser.Close()
		}
	})
}
