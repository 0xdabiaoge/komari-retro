package feishu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type FeishuSender struct {
	Addition
}

func (f *FeishuSender) GetName() string {
	return "feishu"
}

func (f *FeishuSender) GetConfiguration() factory.Configuration {
	return &f.Addition
}

func (f *FeishuSender) Init() error {
	if f.Addition.WebhookURL == "" {
		return fmt.Errorf("feishu webhook_url is required")
	}
	if _, err := url.ParseRequestURI(f.Addition.WebhookURL); err != nil {
		return fmt.Errorf("invalid feishu webhook_url: %w", err)
	}
	return nil
}

func (f *FeishuSender) Destroy() error {
	return nil
}

func genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write([]byte{})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func (f *FeishuSender) SendTextMessage(message, title string) error {
	if f.Addition.WebhookURL == "" {
		return fmt.Errorf("feishu webhook_url is not configured")
	}
	if message == "" && title == "" {
		return fmt.Errorf("message content is empty")
	}

	payload := map[string]interface{}{}

	if f.Addition.Secret != "" {
		timestamp := time.Now().Unix()
		sign, err := genSign(f.Addition.Secret, timestamp)
		if err != nil {
			return fmt.Errorf("failed to generate feishu signature: %w", err)
		}
		payload["timestamp"] = strconv.FormatInt(timestamp, 10)
		payload["sign"] = sign
	}

	if title != "" {
		payload["msg_type"] = "post"
		payload["content"] = map[string]interface{}{
			"post": map[string]interface{}{
				"zh_cn": map[string]interface{}{
					"title": title,
					"content": [][]map[string]string{
						{
							{
								"tag":  "text",
								"text": message,
							},
						},
					},
				},
			},
		}
	} else {
		payload["msg_type"] = "text"
		payload["content"] = map[string]interface{}{
			"text": message,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal feishu payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(f.Addition.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to post to feishu: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu returned HTTP status: %d", resp.StatusCode)
	}

	var result struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		StatusC int    `json:"StatusCode"`
		StatusM string `json:"StatusMessage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode feishu response: %w", err)
	}

	if result.Code != 0 && result.StatusC != 0 {
		msg := result.Msg
		if msg == "" {
			msg = result.StatusM
		}
		return fmt.Errorf("feishu error: %s (code: %d)", msg, result.Code)
	}

	return nil
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &FeishuSender{}
	})
}

var _ factory.IMessageSender = (*FeishuSender)(nil)
