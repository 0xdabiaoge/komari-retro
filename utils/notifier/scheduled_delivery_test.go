package notifier

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/messageSender"
)

func TestScheduledExpiryAndTrafficCallbacksDeliverToIsolatedSink(t *testing.T) {
	t.Chdir(t.TempDir())
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = filepath.Join(t.TempDir(), "notifications.db")
	db := dbcore.GetDBInstance()
	t.Cleanup(func() { dbcore.Close() })
	events := make(chan string, 16)
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		events <- r.Form.Get("text")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer sink.Close()
	addition, _ := json.Marshal(map[string]string{"endpoint": sink.URL, "bot_token": "isolated-token", "chat_id": "isolated-chat"})
	if err := messageSender.LoadProvider("telegram", string(addition)); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"notification_enabled": true, "expire_notification_enabled": true, "expire_notification_lead_days": 7, "notification_template": "{{status}}|{{message}}"} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	for _, node := range []models.Client{
		{UUID: "near-expiry", Token: "near-expiry", Name: "NEAR_EXPIRY", ExpiredAt: models.FromTime(now.Add(48 * time.Hour))},
		{UUID: "outside-window", Token: "outside-window", Name: "OUTSIDE_WINDOW", ExpiredAt: models.FromTime(now.Add(30 * 24 * time.Hour))},
		{UUID: "expired", Token: "expired", Name: "ALREADY_EXPIRED", ExpiredAt: models.FromTime(now.Add(-24 * time.Hour))},
	} {
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
	}
	CheckExpireScheduledWork()
	select {
	case event := <-events:
		if !strings.Contains(event, "NEAR_EXPIRY") || strings.Contains(event, "OUTSIDE_WINDOW") || strings.Contains(event, "ALREADY_EXPIRED") {
			t.Fatalf("wrong expiry recipients: %s", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expiry event not delivered")
	}
	t.Run("disabled-expiry", func(t *testing.T) {
		config.Set("expire_notification_enabled", false)
		CheckExpireScheduledWork()
		select {
		case event := <-events:
			t.Fatalf("disabled expiry delivered: %s", event)
		default:
		}
	})
	for _, cadence := range []string{"daily", "weekly", "monthly"} {
		t.Run(cadence, func(t *testing.T) {
			node := models.Client{UUID: cadence, Token: cadence, Name: strings.ToUpper(cadence) + "_FIXTURE", TrafficLimitType: "sum"}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			notification := models.TrafficReportNotification{Client: cadence, Enable: true, Daily: cadence == "daily", Weekly: cadence == "weekly", Monthly: cadence == "monthly"}
			if err := db.Create(&notification).Error; err != nil {
				t.Fatal(err)
			}
			instant := now.AddDate(0, 0, -1)
			if cadence == "weekly" {
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				instant = now.AddDate(0, 0, -(weekday-1)-7)
			}
			if cadence == "monthly" {
				instant = time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
			}
			if err := db.Create(&models.Record{Client: cadence, Time: models.FromTime(instant), TrafficUp: 400, TrafficDown: 600}).Error; err != nil {
				t.Fatal(err)
			}
			sendTrafficReport(cadence == "daily", cadence == "weekly", cadence == "monthly")
			select {
			case event := <-events:
				if !strings.Contains(event, node.Name) || !strings.Contains(event, "1000 B") {
					t.Fatalf("wrong traffic report: %s", event)
				}
			case <-time.After(time.Second):
				t.Fatal("traffic report not delivered")
			}
		})
	}
}
