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
				if r.Method != http.MethodPost || r.URL.Path != "/bottest-token/sendMessage" {
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

func TestTelegramOfficialURLAndProxyPrefixes(t *testing.T) {
	// Examples follow the Bot API contract, rather than copying the URL builder.
	for _, endpoint := range []string{"", "https://api.telegram.org", "https://api.telegram.org/", "https://api.telegram.org/bot", "https://api.telegram.org/bot/"} {
		got, err := telegramMethodURL(endpoint, "123456:ABC-DEF", "sendMessage")
		if err != nil || got != "https://api.telegram.org/bot123456:ABC-DEF/sendMessage" {
			t.Fatalf("endpoint %q: URL=%q error=%v", endpoint, got, err)
		}
	}
	for _, prefix := range []string{"/bot", "/bot/", "", "/", "/relay/bot", "/relay/bot/", "/relay"} {
		t.Run(prefix, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expected := "/bot123456:ABC-DEF/sendMessage"
				if strings.HasPrefix(prefix, "/relay") {
					expected = "/relay" + expected
				}
				if r.URL.Path != expected {
					t.Errorf("request path=%q, want %q", r.URL.Path, expected)
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			sender := &TelegramSender{Addition: Addition{Endpoint: server.URL + prefix, BotToken: "123456:ABC-DEF", ChatID: "test-chat"}}
			if err := sender.SendTextMessage("probe", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTelegramErrorsExplainFailureWithoutLeakingToken(t *testing.T) {
	const token = "123456:fixture-secret"
	for _, test := range []struct {
		status     int
		body, want string
	}{
		{400, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`, "chat not found"},
		{401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`, "Unauthorized"},
		{403, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`, "bot was blocked"},
		{429, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":12}}`, "retry after 12 seconds"},
		{400, `{"ok":false,"description":"rejected 123456:fixture-secret and 123456%3Afixture-secret"}`, "[redacted]"},
		{502, `<html>gateway failure</html>`, "HTTP 502"},
		{200, strings.Repeat("x", (64<<10)+1), "exceeds 64 KiB"},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(test.status)
			_, _ = w.Write([]byte(test.body))
		}))
		sender := &TelegramSender{Addition: Addition{Endpoint: server.URL + "/bot", BotToken: token, ChatID: "test-chat"}}
		err := sender.SendTextMessage("probe", "")
		server.Close()
		if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "123456%3Afixture-secret") {
			t.Fatalf("expected safe descriptive error %q, got %v", test.want, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	err := (&TelegramSender{Addition: Addition{Endpoint: server.URL, BotToken: token, ChatID: "test-chat"}}).SendTextMessage("probe", "")
	if err == nil || !strings.Contains(err.Error(), "failed to send message") || strings.Contains(err.Error(), token) {
		t.Fatalf("unsafe transport error: %v", err)
	}
}

func TestTelegramRejectsUnsafeURLConfiguration(t *testing.T) {
	for _, endpoint := range []string{"file:///tmp/bot", "https://user:secret@host/bot", "https://host/bot?key=secret", "https://host/bot#fragment", "://invalid"} {
		if _, err := telegramMethodURL(endpoint, "test-token", "sendMessage"); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
	if _, err := telegramMethodURL("https://api.telegram.org/bot", "bad/token", "sendMessage"); err == nil {
		t.Fatal("path injection accepted")
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
