package messageSender

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

var (
	currentProvider factory.IMessageSender
	mu              = sync.Mutex{}
	once            = sync.Once{}
)

func CurrentProvider() factory.IMessageSender {
	mu.Lock()
	defer mu.Unlock()
	return currentProvider
}

func Initialize() {
	go func() {
		once.Do(func() {
			all := factory.GetAllMessageSenders()
			for _, provider := range all {
				if _, err := database.GetMessageSenderConfigByName(provider.GetName()); err == nil {
					continue
				}
				// 如果数据库中没有该提供者的配置，则保存默认配置
				config := provider.GetConfiguration()
				configBytes, err := json.Marshal(config)
				if err != nil {
					log.Printf("Failed to marshal config for provider %s: %v", provider.GetName(), err)
					return
				}
				if err := database.SaveMessageSenderConfig(&models.MessageSenderProvider{
					Name:     provider.GetName(),
					Addition: string(configBytes),
				}); err != nil {
					log.Printf("Failed to save default config for provider %s: %v", provider.GetName(), err)
					return
				}
			}
		})
	}()
	NotificationMethod, _ := config.GetAs[string](config.NotificationMethodKey, "none")

	if NotificationMethod == "" || NotificationMethod == "none" {
		mu.Lock()
		if currentProvider != nil {
			currentProvider.Destroy()
			currentProvider = nil
		}
		mu.Unlock()
		return
	}

	// 尝试从数据库加载配置
	senderConfig, err := database.GetMessageSenderConfigByName(NotificationMethod)
	if err != nil {
		mu.Lock()
		if currentProvider != nil {
			currentProvider.Destroy()
			currentProvider = nil
		}
		mu.Unlock()
		return
	}
	_ = LoadProvider(NotificationMethod, senderConfig.Addition)
}

func SendTextMessage(message string, title string) error {
	if CurrentProvider() == nil {
		return fmt.Errorf("message sender provider is not initialized")
	}
	var err error
	NotificationEnabled, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil {
		return err
	}
	if !NotificationEnabled {
		return nil
	}
	for i := 0; i < 3; i++ {
		err = CurrentProvider().SendTextMessage(message, title)
		if err == nil {
			auditlog.Log("", "", "Message sent: "+title, "info")
			return nil
		}
	}
	auditlog.Log("", "", "Failed to send message after 3 attempts: "+err.Error()+","+title, "error")
	return err
}
func SendEvent(event models.EventMessage) error {
	if CurrentProvider() == nil {
		return fmt.Errorf("message sender provider is not initialized")
	}
	var err error
	cfg, err := config.GetMany(map[string]any{
		config.NotificationEnabledKey:  false,
		config.NotificationTemplateKey: "{{emoji}} <b>【Komari 监控告警】</b> {{emoji}}\n━━━━━━━━━━━━━━━\n📌 <b>告警事件</b>: <code>{{status}}</code>\n🖥 <b>监控节点</b>: <b>{{client}}</b>\n🌐 <b>节点网络</b>: <code>{{ip}}</code> ({{region}})\n📝 <b>告警详情</b>: {{message}}\n⏰ <b>告警时间</b>: <code>{{time}}</code>\n━━━━━━━━━━━━━━━\n🔔 <i>来自 {{site_name}} 监控平台</i>",
	})
	if err != nil {
		return err
	}
	if !cfg[config.NotificationEnabledKey].(bool) {
		return nil
	}

	// 检查提供者是否实现了 IEventMessageSender 接口
	if eventSender, ok := CurrentProvider().(factory.IEventMessageSender); ok {
		// 如果实现了,直接调用 SendEvent
		for i := 0; i < 3; i++ {
			err = eventSender.SendEvent(event)
			if err == nil || err.Error() == "short response: \x00\x00\x00\x1a\x00\x00\x00" {
				auditlog.Log("", "", "Event message sent: "+event.Event, "info")
				return nil
			}
		}
		auditlog.Log("", "", "Failed to send event message after 3 attempts: "+err.Error()+","+event.Event, "error")
		return err
	}

	// 如果没有实现,使用模板格式化为文本消息
	messageTemplate := cfg[config.NotificationTemplateKey].(string)

	messageTemplate = parseTemplate(messageTemplate, event)

	for i := 0; i < 3; i++ {
		err = CurrentProvider().SendTextMessage(messageTemplate, "")
		if err == nil || err.Error() == "short response: \x00\x00\x00\x1a\x00\x00\x00" { // QQ 会返回这个错误，但实际上消息是发送成功的
			auditlog.Log("", "", "Event message sent: "+event.Event, "info")
			return nil
		}
	}
	auditlog.Log("", "", "Failed to send event message after 3 attempts: "+err.Error()+","+event.Event, "error")
	return err
}

func escapeTelegramHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func parseTemplate(messageTemplate string, event models.EventMessage) string {
	clientNames := make([]string, 0, len(event.Clients))
	clientIPs := make([]string, 0, len(event.Clients))
	clientRegions := make([]string, 0, len(event.Clients))
	clientOSs := make([]string, 0, len(event.Clients))
	clientGroups := make([]string, 0, len(event.Clients))

	for _, c := range event.Clients {
		name := c.Name
		if strings.TrimSpace(name) == "" {
			name = c.UUID
		}
		clientNames = append(clientNames, name)

		ip := c.IPv4
		if ip == "" {
			ip = c.IPv6
		}
		if ip != "" {
			clientIPs = append(clientIPs, ip)
		}

		if c.Region != "" {
			clientRegions = append(clientRegions, c.Region)
		}
		if c.OS != "" {
			clientOSs = append(clientOSs, c.OS)
		}
		if c.Group != "" {
			clientGroups = append(clientGroups, c.Group)
		}
	}

	joinedClients := strings.Join(clientNames, ", ")
	if joinedClients == "" {
		joinedClients = "无关联节点"
	}
	joinedIPs := strings.Join(clientIPs, ", ")
	if joinedIPs == "" {
		joinedIPs = "未知"
	}
	joinedRegions := strings.Join(clientRegions, ", ")
	if joinedRegions == "" {
		joinedRegions = "未知地区"
	}
	joinedOSs := strings.Join(clientOSs, ", ")
	if joinedOSs == "" {
		joinedOSs = "未知系统"
	}
	joinedGroups := strings.Join(clientGroups, ", ")
	if joinedGroups == "" {
		joinedGroups = "默认分组"
	}

	// 智能 Emoji
	emoji := event.Emoji
	if emoji == "" {
		switch strings.ToLower(event.Event) {
		case "offline":
			emoji = "🔴"
		case "online":
			emoji = "🟢"
		case "load":
			emoji = "📈"
		case "traffic":
			emoji = "📊"
		case "traffic_report":
			emoji = "📑"
		case "expire", "renewal":
			emoji = "⏰"
		case "login":
			emoji = "🔐"
		case "test":
			emoji = "🚀"
		default:
			emoji = "🔔"
		}
	}

	// 友好状态描述
	statusText := event.Event
	switch strings.ToLower(event.Event) {
	case "offline":
		statusText = "节点离线 ⚠️"
	case "online":
		statusText = "节点恢复上线 ✅"
	case "load":
		statusText = "系统高负载预警 📈"
	case "traffic":
		statusText = "流量超额预警 📊"
	case "traffic_report":
		statusText = "月度流量报告 📑"
	case "expire":
		statusText = "服务器即将到期 ⏰"
	case "login":
		statusText = "后台登录提醒 🔐"
	case "test":
		statusText = "通知测试 🚀"
	}

	timeStr := event.Time.Format("2006-01-02 15:04:05")
	dateStr := event.Time.Format("2006-01-02")
	siteName, _ := config.GetAs[string](config.SitenameKey, "Komari Retro")
	if strings.TrimSpace(siteName) == "" {
		siteName = "Komari Retro"
	}

	replaceMap := map[string]string{
		"{{event}}":       escapeTelegramHTML(event.Event),
		"{{status}}":      escapeTelegramHTML(statusText),
		"{{status_text}}": escapeTelegramHTML(statusText),
		"{{client}}":      escapeTelegramHTML(joinedClients),
		"{{client_name}}": escapeTelegramHTML(joinedClients),
		"{{ip}}":          escapeTelegramHTML(joinedIPs),
		"{{client_ip}}":   escapeTelegramHTML(joinedIPs),
		"{{region}}":      escapeTelegramHTML(joinedRegions),
		"{{os}}":          escapeTelegramHTML(joinedOSs),
		"{{group}}":       escapeTelegramHTML(joinedGroups),
		"{{time}}":        timeStr,
		"{{date}}":        dateStr,
		"{{raw_time}}":    event.Time.Format(time.RFC3339),
		"{{message}}":     escapeTelegramHTML(event.Message),
		"{{emoji}}":       emoji,
		"{{site_name}}":   escapeTelegramHTML(siteName),
		"{{server}}":      escapeTelegramHTML(siteName),
	}
	result := messageTemplate
	for placeholder, value := range replaceMap {
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
