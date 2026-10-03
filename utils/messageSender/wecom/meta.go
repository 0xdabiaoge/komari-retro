package wecom

type Addition struct {
	WebhookURL string `json:"webhook_url" required:"true" help:"Enterprise WeChat Bot Webhook URL (e.g. https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx)"`
	MsgType    string `json:"msg_type" type:"option" default:"markdown" options:"markdown,text" help:"Message type: markdown or text"`
}
