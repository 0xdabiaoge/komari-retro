package terminal

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/clients"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/utils"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
)

func RequestTerminal(c *gin.Context) {
	uuid := c.Param("uuid")
	user_uuid, _ := c.Get("uuid")
	_, err := clients.GetClientByUUID(uuid)
	if err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Client not found",
		})
		return
	}
	// 建立ws
	if !api.IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Require WebSocket upgrade"})
		return
	}
	conn, err := api.UpgradeWebSocket(c)
	if err != nil {
		return
	}
	// 新建一个终端连接
	id := utils.GenerateRandomString(32)
	session := &TerminalSession{
		UserUUID:    user_uuid.(string),
		UUID:        uuid,
		Browser:     conn,
		Agent:       nil,
		RequesterIp: c.ClientIP(),
	}

	TerminalSessionsMutex.Lock()
	TerminalSessions[id] = session
	TerminalSessionsMutex.Unlock()
	conn.SetCloseHandler(func(code int, text string) error {
		log.Println("Terminal connection closed:", code, text)
		TerminalSessionsMutex.Lock()
		delete(TerminalSessions, id)
		TerminalSessionsMutex.Unlock()
		// 通知 Agent 关闭终端连接
		if session.Agent != nil {
			session.Agent.Close()
		}
		return nil
	})

	if !agent_runtime.IsAgentOnline(uuid) {
		conn.WriteMessage(1, []byte("Client offline!\n被控端离线!\n"))
		conn.Close()
		TerminalSessionsMutex.Lock()
		delete(TerminalSessions, id)
		TerminalSessionsMutex.Unlock()
		return
	}

	// 1. Dispatch via V2 JSON-RPC (works for WebSocket and HTTP pull)
	dispatched := agent_runtime.DispatchV2Event(uuid, v2.MethodAgentTerminal, map[string]string{
		"request_id": id,
	})

	// 2. Also send legacy V1 format if client has direct WebSocket connection
	if clientConn := agent_runtime.GetConnectedClients()[uuid]; clientConn != nil {
		_ = clientConn.WriteJSON(gin.H{
			"message":    "terminal",
			"request_id": id,
		})
	}

	if !dispatched && agent_runtime.GetConnectedClients()[uuid] == nil {
		conn.WriteMessage(1, []byte("Failed to dispatch terminal request to agent.\n无法向被控端发送终端请求。\n"))
		conn.Close()
		TerminalSessionsMutex.Lock()
		delete(TerminalSessions, id)
		TerminalSessionsMutex.Unlock()
		return
	}

	conn.WriteMessage(1, []byte("等待被控端连接 waiting for agent...\n"))
	// 如果没有连接上，则关闭连接
	time.AfterFunc(30*time.Second, func() {
		TerminalSessionsMutex.Lock()
		if session.Agent == nil {
			if session.Browser != nil {
				session.Browser.WriteMessage(1, []byte("被控端连接超时 timeout\n"))
				session.Browser.Close()
			}
			conn.Close()
			delete(TerminalSessions, id)
		}
		TerminalSessionsMutex.Unlock()
	})
	//auditlog.Log(c.ClientIP(), user_uuid.(string), "request, terminal id:"+id+",client:"+session.UUID, "terminal")
}
