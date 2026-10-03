package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type DiscordSender struct {
	Addition
}

func (d *DiscordSender) GetName() string {
	return "discord"
}

func (d *DiscordSender) GetConfiguration() factory.Configuration {
	return &d.Addition
}

func (d *DiscordSender) Init() error {
	if d.Addition.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url is required")
	}
	if _, err := url.ParseRequestURI(d.Addition.WebhookURL); err != nil {
		return fmt.Errorf("invalid discord webhook_url: %w", err)
	}
	return nil
}

func (d *DiscordSender) Destroy() error {
	return nil
}

func (d *DiscordSender) SendTextMessage(message, title string) error {
	if d.Addition.WebhookURL == "" {
		return fmt.Errorf("discord webhook_url is not configured")
	}
	if message == "" && title == "" {
		return fmt.Errorf("message content is empty")
	}

	payload := map[string]interface{}{}

	username := d.Addition.Username
	if username == "" {
		username = "Komari Monitor"
	}
	payload["username"] = username

	if d.Addition.AvatarURL != "" {
		payload["avatar_url"] = d.Addition.AvatarURL
	}

	embed := map[string]interface{}{
		"description": message,
		"color":       0x5865F2, // Discord Blurple
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if title != "" {
		embed["title"] = title
	}
	payload["embeds"] = []interface{}{embed}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(d.Addition.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to post to discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned non-success HTTP status: %d", resp.StatusCode)
	}

	return nil
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &DiscordSender{}
	})
}

var _ factory.IMessageSender = (*DiscordSender)(nil)
