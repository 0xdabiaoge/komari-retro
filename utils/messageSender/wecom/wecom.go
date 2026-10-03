package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type WeComSender struct {
	Addition
}

func (w *WeComSender) GetName() string {
	return "wecom"
}

func (w *WeComSender) GetConfiguration() factory.Configuration {
	return &w.Addition
}

func (w *WeComSender) Init() error {
	if w.Addition.WebhookURL == "" {
		return fmt.Errorf("wecom webhook_url is required")
	}
	if _, err := url.ParseRequestURI(w.Addition.WebhookURL); err != nil {
		return fmt.Errorf("invalid wecom webhook_url: %w", err)
	}
	return nil
}

func (w *WeComSender) Destroy() error {
	return nil
}

func (w *WeComSender) SendTextMessage(message, title string) error {
	if w.Addition.WebhookURL == "" {
		return fmt.Errorf("wecom webhook_url is not configured")
	}
	if message == "" && title == "" {
		return fmt.Errorf("message content is empty")
	}

	msgType := strings.ToLower(strings.TrimSpace(w.Addition.MsgType))
	if msgType == "" {
		msgType = "markdown"
	}

	var payload map[string]interface{}
	if msgType == "markdown" {
		var content strings.Builder
		if title != "" {
			content.WriteString("### ")
			content.WriteString(title)
			content.WriteString("\n\n")
		}
		content.WriteString(message)

		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": content.String(),
			},
		}
	} else {
		var content strings.Builder
		if title != "" {
			content.WriteString("【")
			content.WriteString(title)
			content.WriteString("】\n\n")
		}
		content.WriteString(message)

		payload = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content.String(),
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal wecom payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(w.Addition.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to post to wecom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom returned HTTP status: %d", resp.StatusCode)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode wecom response: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("wecom error code %d: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &WeComSender{}
	})
}

var _ factory.IMessageSender = (*WeComSender)(nil)
