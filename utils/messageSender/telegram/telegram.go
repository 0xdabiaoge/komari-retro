package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	token := strings.TrimSpace(t.Addition.BotToken)
	apiURL, err := telegramMethodURL(t.Addition.Endpoint, token, "sendMessage")
	if err != nil {
		return err
	}

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
		return fmt.Errorf("failed to send message: %s", redactSecret(err.Error(), token))
	}
	defer resp.Body.Close()

	const maxResponse = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return fmt.Errorf("failed to read Telegram response: %s", redactSecret(err.Error(), token))
	}
	if len(body) > maxResponse {
		return errors.New("telegram API response exceeds 64 KiB")
	}

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
		ErrorCode   int    `json:"error_code"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("telegram API returned HTTP %d with an invalid JSON response", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK || !result.Ok {
		description := strings.TrimSpace(redactSecret(result.Description, token))
		if description == "" {
			description = "request rejected"
		}
		detail := fmt.Sprintf("telegram API error (HTTP %d, code %d): %s", resp.StatusCode, result.ErrorCode, description)
		if result.Parameters.RetryAfter > 0 {
			detail += fmt.Sprintf("; retry after %d seconds", result.Parameters.RetryAfter)
		}
		return errors.New(detail)
	}

	return nil
}

// Telegram appends the token directly to the literal "bot" prefix, including
// when using a self-hosted Bot API or a reverse proxy with a path prefix.
func telegramMethodURL(endpoint, token, method string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "https://api.telegram.org/bot"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid Telegram endpoint: use an HTTP(S) Bot API base URL without credentials, query or fragment")
	}
	if strings.ContainsAny(token, "/\\?# \t\r\n") {
		return "", errors.New("bot_token contains invalid characters")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/bot") {
		u.Path += "/bot"
	}
	u.Path += token + "/" + method
	u.RawPath = ""
	return u.String(), nil
}

func redactSecret(text, token string) string {
	for _, secret := range []string{token, url.QueryEscape(token), url.PathEscape(token)} {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return text
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &TelegramSender{}
	})
}

// 确保实现了 IMessageSender 接口
var _ factory.IMessageSender = (*TelegramSender)(nil)
