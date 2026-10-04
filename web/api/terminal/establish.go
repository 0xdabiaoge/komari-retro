package terminal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/web/api"
)

func EstablishConnection(c *gin.Context) {
	session_id := c.Query("id")
	TerminalSessionsMutex.Lock()
	session, exists := TerminalSessions[session_id]
	TerminalSessionsMutex.Unlock()
	if !exists || session == nil || session.Browser == nil {
		c.JSON(404, gin.H{"status": "error", "error": "Session not found"})
		return
	}

	// 验证被控端客户端身份，必须与申请建立终端的主机 UUID 完全一致 (防冒充劫持)
	if clientUUID, ok := c.Get("client_uuid"); ok {
		if uuidStr, ok := clientUUID.(string); ok && uuidStr != "" && uuidStr != session.UUID {
			c.JSON(http.StatusForbidden, gin.H{"status": "error", "error": "Client identity does not match terminal session"})
			return
		}
	}

	// Upgrade the connection to WebSocket
	if !api.IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	conn, err := api.UpgradeWebSocket(c)
	if err != nil {
		TerminalSessionsMutex.Lock()
		if session.Browser != nil {
			session.Browser.Close()
		}
		delete(TerminalSessions, session_id)
		TerminalSessionsMutex.Unlock()
		return
	}

	TerminalSessionsMutex.Lock()
	session.Agent = conn
	TerminalSessionsMutex.Unlock()

	conn.SetCloseHandler(func(code int, text string) error {
		TerminalSessionsMutex.Lock()
		delete(TerminalSessions, session_id)
		TerminalSessionsMutex.Unlock()
		// 通知 Browser 关闭终端连接
		if session.Browser != nil {
			session.Browser.Close()
		}
		return nil
	})
	go ForwardTerminal(session_id)
}
