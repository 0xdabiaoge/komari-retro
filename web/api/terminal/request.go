package terminal

import (
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/pkg/config"
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
	sessionToken, apiKey := c.GetString("session"), c.GetString("api_key")
	authorized := func() bool {
		if apiKey != "" {
			current, _ := config.GetAs[string](config.ApiKeyKey, "")
			scope, _ := config.GetAs[string](config.ApiKeyScopeKey, "full")
			return scope == "full" && subtle.ConstantTimeCompare([]byte(current), []byte(apiKey)) == 1
		}
		_, err := accounts.GetUserBySession(sessionToken)
		return sessionToken != "" && err == nil
	}
	session := &TerminalSession{UUID: uuid, UserUUID: c.GetString("uuid"), Browser: browser, RequesterIp: c.ClientIP(), Done: make(chan struct{}), Authorized: authorized}
	TerminalSessionsMutex.Lock()
	owned := 0
	for _, existing := range TerminalSessions {
		if existing != nil && existing.UserUUID == session.UserUUID {
			owned++
		}
	}
	if len(TerminalSessions) >= 32 || owned >= 8 {
		TerminalSessionsMutex.Unlock()
		_ = browser.WriteMessage(1, []byte("Terminal capacity exceeded\n"))
		_ = browser.Close()
		return
	}
	TerminalSessions[id] = session
	TerminalSessionsMutex.Unlock()
	go watchAuthorization(id, session, 2*time.Second)
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
		TerminalSessionsMutex.Unlock()
		if waiting {
			browser.WriteMessage(1, []byte("被控端连接超时 timeout\n"))
			closeTerminal(id)
		}
	})
}
