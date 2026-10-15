package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelegramDeliveryAndFailures(t *testing.T) {
	for _, tt := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"delivered", 200, `{"ok":true}`, false},
		{"invalid chat", 200, `{"ok":false,"description":"chat not found"}`, true},
		{"unauthorized", 401, `{"ok":false}`, true},
		{"rate limited", 429, `{"ok":false}`, true},
		{"gateway failure", 502, `bad gateway`, true},
		{"malformed response", 200, `not json`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var called atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				if r.Method != http.MethodPost || r.URL.Path != "/bot/test-token/sendMessage" {
					t.Errorf("unexpected Telegram request: %s %s", r.Method, r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("chat_id") != "test-chat" || r.Form.Get("message_thread_id") != "42" || r.Form.Get("parse_mode") != "HTML" || r.Form.Get("text") != "<b>测试标题</b>\n节点已离线" {
					t.Errorf("incorrect Telegram form: %v", r.Form)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			sender := &TelegramSender{Addition: Addition{Endpoint: server.URL + "/bot", BotToken: " test-token ", ChatID: " test-chat ", MessageThreadID: " 42 "}}
			err := sender.SendTextMessage("节点已离线", "测试标题")
			if !called.Load() || (err != nil) != tt.wantError {
				t.Fatalf("called=%v error=%v, expected error=%v", called.Load(), err, tt.wantError)
			}
		})
	}
}

func TestTelegramRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		select {
		case <-r.Context().Done():
		case <-time.After(20 * time.Second):
		}
	}))
	defer server.Close()
	sender := &TelegramSender{Addition: Addition{Endpoint: server.URL + "/bot", BotToken: "test-token", ChatID: "test-chat"}}
	if err := sender.SendTextMessage("timeout test", ""); err == nil || !strings.Contains(err.Error(), "failed to send message") {
		t.Fatalf("unresponsive Telegram endpoint was not rejected: %v", err)
	}
}

func TestTelegramRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []Addition{{}, {BotToken: "token"}, {ChatID: "chat"}} {
		sender := &TelegramSender{Addition: config}
		if err := sender.SendTextMessage("test", ""); err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("missing credentials were accepted: %v", err)
		}
	}
	if err := (&TelegramSender{}).SendTextMessage("", ""); err == nil {
		t.Fatal("empty message was accepted")
	}
}
