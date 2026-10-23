package api

import (
	"github.com/komari-monitor/komari/pkg/share"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func GetClients(c *gin.Context) {
	release, ok := AdmitViewerSocket(c)
	if !ok {
		return
	}
	defer release()
	// 升级到ws
	if !IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := UpgradeWebSocket(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)

	// 初始化用户信息
	var (
		isLogin    = false
		hiddenMap  = map[string]bool{}
		session, _ = c.Cookie("session_token")
	)

	// 登录状态检查
	_, err = accounts.GetUserBySession(session)
	if err == nil {
		isLogin = true
	}

	// 私有站点：未登录且未持有有效临时分享许可，拒绝访问并关闭连接
	if !isLogin {
		if privateSite, _ := config.GetAs[bool](config.PrivateSiteKey, false); privateSite {
			if !hasTempAccess(c) {
				conn.WriteJSON(gin.H{"status": "error", "error": "Private site is enabled, please login first."})
				return
			}
		}
	}

	// 仅在未登录时需要 Hidden 信息做过滤
	if !isLogin {
		var hiddenClients []models.Client
		db := dbcore.GetDBInstance()
		_ = db.Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error
		for _, cli := range hiddenClients {
			hiddenMap[cli.UUID] = true
		}
	}

	// 请求
	for {
		var resp struct {
			Online []string       `json:"online"` // 已建立连接的客户端uuid列表
			Data   map[string]any `json:"data"`   // Explicit audience projection.
		}

		resp.Online = []string{}
		resp.Data = map[string]any{}

		_ = conn.SetReadDeadline(time.Now().Add(ViewerSocketIdleTimeout))
		_, data, err := conn.ReadMessage()
		if err != nil {
			//log.Println("Error reading message:", err)
			return
		}
		_, sessionErr := accounts.GetUserBySession(session)
		isLogin = sessionErr == nil
		if !isLogin {
			if privateSite, _ := config.GetAs[bool](config.PrivateSiteKey, false); privateSite && !hasTempAccess(c) {
				return
			}
			hiddenMap = map[string]bool{}
			var hiddenClients []models.Client
			if err := dbcore.GetDBInstance().Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error; err != nil {
				return
			}
			for _, cli := range hiddenClients {
				hiddenMap[cli.UUID] = true
			}
		}
		message := string(data)
		grant := share.FromRequest(c.Request)

		uuID := ""
		if message != "get" { // 非请求全部内容
			if strings.HasPrefix(message, "get ") {
				uuID = strings.TrimSpace(strings.TrimPrefix(message, "get "))
			} else {
				conn.WriteJSON(gin.H{"status": "error", "error": "Invalid message"})
				continue
			}
		}

		// 在线客户端uuid列表（WebSocket 与非 WebSocket）
		for _, key := range agent_runtime.GetAllOnlineUUIDs() {
			if !isLogin && !grant.Allows(key) {
				continue
			}
			if !isLogin && hiddenMap[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}
			resp.Online = append(resp.Online, key)
		}

		//过往节点数据信息
		for key, report := range agent_runtime.GetLatestReport() {
			if !isLogin && !grant.Allows(key) {
				continue
			}
			if !isLogin && hiddenMap[key] {
				continue
			}
			if uuID != "" && key != uuID {
				continue
			}

			if isLogin {
				resp.Data[key] = *report
			} else {
				public := v1.PublicReport(*report)
				public.UUID = ""
				resp.Data[key] = public
			}
		}

		err = conn.WriteJSON(gin.H{"status": "success", "data": resp})
		if err != nil {
			return
		}
	}
}
