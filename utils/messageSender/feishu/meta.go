package feishu

type Addition struct {
	WebhookURL string `json:"webhook_url" required:"true" help:"Feishu / Lark Bot Webhook URL (e.g. https://open.feishu.cn/open-apis/bot/v2/hook/xxx)"`
	Secret     string `json:"secret" help:"Optional webhook signature verification secret"`
}
