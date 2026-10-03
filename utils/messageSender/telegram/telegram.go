package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type TelegramSender struct {
	Addition
}

func (t *TelegramSender) GetName() string {
	return "telegram"
}

func (t *TelegramSender) GetConfiguration() factory.Configuration {
	return &t.Addition
}

func (t *TelegramSender) Init() error {
	// 初始化逻辑，如果需要的话
	return nil
}

func (t *TelegramSender) Destroy() error {
	// 清理逻辑，如果需要的话
	return nil
}

func (t *TelegramSender) SendTextMessage(message, title string) error {
	fullMessage := message
	if title != "" {
		fullMessage = fmt.Sprintf("<b>%s</b>\n%s", title, message)
	}

	if fullMessage == "" {
		return errors.New("message is empty")
	}

	if strings.TrimSpace(t.Addition.BotToken) == "" || strings.TrimSpace(t.Addition.ChatID) == "" {
		return errors.New("bot_token and chat_id are required")
	}

	endpoint := strings.TrimSpace(t.Addition.Endpoint)
	if endpoint == "" {
		endpoint = "https://api.telegram.org/bot"
	}
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	apiURL := endpoint + strings.TrimSpace(t.Addition.BotToken) + "/sendMessage"

	data := url.Values{}
	data.Set("chat_id", strings.TrimSpace(t.Addition.ChatID))
	data.Set("text", fullMessage)
	data.Set("parse_mode", "HTML")

	// Add message_thread_id if provided
	if t.Addition.MessageThreadID != "" {
		data.Set("message_thread_id", strings.TrimSpace(t.Addition.MessageThreadID))
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-OK status: %d", resp.StatusCode)
	}

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Ok {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}

	return nil
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &TelegramSender{}
	})
}

// 确保实现了 IMessageSender 接口
var _ factory.IMessageSender = (*TelegramSender)(nil)
