package discord

type Addition struct {
	WebhookURL string `json:"webhook_url" required:"true" help:"Discord Webhook URL (e.g. https://discord.com/api/webhooks/xxx/yyy)"`
	Username   string `json:"username" default:"Komari Monitor" help:"Custom bot username (default: Komari Monitor)"`
	AvatarURL  string `json:"avatar_url" help:"Custom bot avatar image URL"`
}
