package terminal

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
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

func dispatchTerminal(id string, s *TerminalSession) {
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] == s && s.Agent == nil && !s.agentDeadline.IsZero() && time.Now().After(s.agentDeadline) {
		s.expiring = true
		TerminalSessionsMutex.Unlock()
		revokeTerminal(id, s)
		return
	}
	if TerminalSessions[id] != s || s.Agent != nil || s.shellExited || s.expiring || time.Since(s.lastDispatch) < 5*time.Second {
		TerminalSessionsMutex.Unlock()
		return
	}
	s.lastDispatch = time.Now()
	TerminalSessionsMutex.Unlock()
	if !agentRuntime.IsAgentOnline(s.UUID) {
		return
	}
	client := agentRuntime.GetConnectedClients()[s.UUID]
	if client != nil && !agentRuntime.IsV2Client(s.UUID) {
		_ = client.WriteJSON(gin.H{"message": "terminal", "request_id": id})
	} else {
		agentRuntime.DispatchV2Event(s.UUID, v2.MethodAgentTerminal, map[string]string{"request_id": id})
	}
}
func RequestTerminal(c *gin.Context) {
	uuid := c.Param("uuid")
	if _, err := clients.GetClientByUUID(uuid); err != nil {
		c.Status(404)
		return
	}
	if !api.IsWebSocketUpgrade(c) {
		c.Status(http.StatusBadRequest)
		return
	}
	token, key := c.GetString("session"), c.GetString("api_key")
	authorized := func() bool {
		if key != "" {
			current, _ := config.GetAs[string](config.ApiKeyKey, "")
			scope, _ := config.GetAs[string](config.ApiKeyScopeKey, "full")
			return scope == "full" && subtle.ConstantTimeCompare([]byte(current), []byte(key)) == 1
		}
		_, err := accounts.GetUserBySession(token)
		return token != "" && err == nil
	}
	if !authorized() {
		c.Status(http.StatusUnauthorized)
		return
	}
	owner := fmt.Sprintf("%x", sha256.Sum256([]byte(token+"\x00"+key)))
	id := c.Query("request_id")
	resuming := id != ""
	TerminalSessionsMutex.Lock()
	var s *TerminalSession
	if resuming {
		s = TerminalSessions[id]
		if !canResume(s, uuid, c.GetString("uuid"), owner, c.Query("flow") == "1") {
			TerminalSessionsMutex.Unlock()
			c.Status(http.StatusNotFound)
			return
		}
	} else {
		owned := 0
		for _, v := range TerminalSessions {
			if v != nil && v.UserUUID == c.GetString("uuid") {
				owned++
			}
		}
		if len(TerminalSessions) >= 32 || owned >= 8 {
			TerminalSessionsMutex.Unlock()
			c.Status(http.StatusTooManyRequests)
			return
		}
		id = utils.GenerateRandomString(32)
		s = &TerminalSession{UUID: uuid, UserUUID: c.GetString("uuid"), OwnerToken: owner, RequesterIp: c.ClientIP(), Done: make(chan struct{}), Authorized: authorized, flow: c.Query("flow") == "1", changed: make(chan struct{}), agentDeadline: time.Now().Add(5 * time.Minute)}
		started := time.Now()
		s.OnClose = func() {
			auditlog.Log(s.RequesterIp, s.UserUUID, "terminal closed, duration:"+time.Since(started).String(), "terminal")
		}
		TerminalSessions[id] = s
	}
	TerminalSessionsMutex.Unlock()
	raw, err := api.UpgradeWebSocket(c)
	if err != nil {
		if !resuming {
			closeTerminal(id)
		}
		return
	}
	raw.SetReadLimit(1 << 20)
	b := connection.NewSafeConn(raw)
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != s {
		TerminalSessionsMutex.Unlock()
		_ = b.Close()
		return
	}
	old := s.Browser
	s.Browser = b
	s.sentSeq = s.ackedSeq
	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	signalLocked(s)
	ready := s.Agent != nil
	TerminalSessionsMutex.Unlock()
	if old != nil {
		_ = old.Close()
	}
	if err := b.WriteJSON(gin.H{"type": "session", "request_id": id, "resumed": resuming, "ready": ready}); err != nil {
		detachBrowser(id, s, b)
		return
	}
	if !resuming {
		go watchAuthorization(id, s, 2*time.Second)
		auditlog.Log(s.RequesterIp, s.UserUUID, "open terminal", "terminal")
	} else {
		auditlog.Log(c.ClientIP(), s.UserUUID, "resume terminal", "terminal")
	}
	dispatchTerminal(id, s)
	go writeBrowser(id, s, b)
	readBrowser(id, s, b)
}

func canResume(s *TerminalSession, uuid, user, owner string, flow bool) bool {
	return s != nil && !s.expiring && s.UUID == uuid && s.UserUUID == user && s.OwnerToken == owner && s.flow == flow
}
