package messageSender

import (
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/notification"
	"github.com/komari-monitor/komari/utils/messageSender/telegram"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotificationTemplatesAndWireDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "templates.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := config.Set(config.SitenameKey, "测试站点 & 运维"); err != nil {
		t.Fatal(err)
	}
	field, _ := reflect.TypeOf(config.Settings{}).FieldByName("NotificationTemplate")
	if field.Tag.Get("default") != notification.DefaultTemplate {
		t.Fatal("initial default differs from shared preset catalog")
	}
	event := models.EventMessage{Event: "offline", Message: `磁盘 < 10% & 节点 "异常"；保留 {{time}}`, Time: time.Date(2026, 10, 16, 9, 41, 0, 0, time.FixedZone("CST", 8*3600)), Clients: []models.Client{
		{Name: `HK <node> & "quoted"`, IPv4: "192.0.2.18", IPv6: "2001:db8::18", Region: "中国香港", OS: "Debian 12", Arch: "amd64", Group: "生产组"},
		{Name: "Tokyo", IPv6: "2001:db8::24", Region: "日本东京", OS: "Ubuntu 24.04", Arch: "arm64", Group: "生产组"},
	}}
	var placeholders []string
	for _, v := range notification.Catalog.Variables {
		placeholders = append(placeholders, "{{"+v.Key+"}}")
	}
	all := parseTemplate(strings.Join(placeholders, "|"), event)
	for _, want := range []string{"|2|", "UTC+08:00", "amd64, arm64", "• HK &lt;node&gt;", "2001:db8::24", "{{time}}"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in rendered catalog", want)
		}
	}
	if strings.Contains(all, `<node>`) || strings.Contains(all, `"quoted"`) {
		t.Fatal("node data was not escaped")
	}
	if got := parseTemplate("{{message}}|{{event}}|{{ event }}|{{unknown}}", event); got != html.EscapeString(event.Message)+"|offline|offline|{{unknown}}" {
		t.Fatalf("substitution must be atomic: %s", got)
	}
	for _, preset := range notification.Catalog.Presets {
		t.Run(preset.ID, func(t *testing.T) {
			message := parseTemplate(preset.Template, event)
			if !strings.Contains(message, html.EscapeString(event.Message)) || !strings.Contains(message, "2026-10-16 09:41:00") {
				t.Fatal("preset dropped event details or time")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/bottest/sendMessage" {
					t.Errorf("unexpected request path %s", r.URL.Path)
				}
				_ = r.ParseForm()
				if r.Form.Get("text") != message || r.Form.Get("parse_mode") != "HTML" {
					t.Error("rendered template changed during delivery")
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			sender := telegram.TelegramSender{Addition: telegram.Addition{Endpoint: server.URL + "/bot", BotToken: "test", ChatID: "fixture"}}
			if err := sender.SendTextMessage(message, ""); err != nil {
				t.Fatal(err)
			}
		})
	}
	empty := models.EventMessage{Event: "renewal", Time: event.Time}
	got := parseTemplate("{{client_count}}|{{client_list}}|{{ipv4}}|{{arch}}|{{status}}|{{client_name}}|{{client_ip}}|{{raw_time}}", empty)
	if !strings.Contains(got, "0|无关联节点|未提供|未提供|服务器续费提醒") || strings.Contains(got, "{{") {
		t.Fatalf("empty event fallback: %s", got)
	}
	// Use the event names emitted by real notifiers, not only preview aliases.
	for kind, status := range map[string]string{
		messageevent.Alert: "系统高负载预警", messageevent.Renew: "服务器自动续期",
		messageevent.DReport: "每日流量报告", messageevent.WReport: "每周流量报告", messageevent.MReport: "每月流量报告",
	} {
		result := parseTemplate("{{status}}|{{emoji}}", models.EventMessage{Event: kind, Time: event.Time})
		if !strings.Contains(result, status) || strings.Contains(result, "🔔") {
			t.Errorf("production event %s not localized: %s", kind, result)
		}
	}
	before := time.Now().Add(-time.Second)
	zeroTime := parseTemplate("{{raw_time}}", models.EventMessage{Event: "Test"})
	parsed, err := time.Parse(time.RFC3339, zeroTime)
	if err != nil || parsed.Before(before) || parsed.After(time.Now().Add(time.Second)) {
		t.Fatalf("missing event time did not use current time: %s", zeroTime)
	}
}
