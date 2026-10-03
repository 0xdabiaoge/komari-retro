package messageSender

import (
	"testing"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

func Test(t *testing.T) {
	senders := factory.GetAllMessageSenders()
	if len(senders) == 0 {
		t.Error("No message senders found")
		return
	}
	cfg := factory.GetSenderConfigs()
	if len(cfg) == 0 {
		t.Error("No sender configs found")
		return
	}
	LoadProvider("telegram", `{"bot_token":"123456:ABC-DEF","chat_id":"12345678"}`)
	cp := CurrentProvider
	if cp() == nil {
		t.Error("Current provider is nil")
		return
	}
}
