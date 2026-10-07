package renewal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	agent "github.com/komari-monitor/komari/web/agent"
)

func TestRenewalHonorsHTTPPresenceAndOfflineExpiry(t *testing.T) {
	t.Chdir(t.TempDir())
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = filepath.Join(t.TempDir(), "renewal.db")
	db := dbcore.GetDBInstance()
	t.Cleanup(func() { dbcore.Close() })
	now := time.Now().Truncate(time.Second)
	for _, test := range []struct {
		name      string
		ttl       time.Duration
		enabled   bool
		cycle     int
		wantRenew bool
	}{
		{"http-online", time.Minute, true, 7, true},
		{"expired-presence", -time.Second, true, 7, false},
		{"disabled-auto-renewal", time.Minute, false, 7, false},
		{"no-billing-cycle", time.Minute, true, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := models.Client{UUID: test.name, Token: test.name, Name: test.name, AutoRenewal: test.enabled, BillingCycle: test.cycle, ExpiredAt: models.FromTime(now.AddDate(0, 0, -1))}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			agent.KeepAlivePresence(node.UUID, 1, test.ttl)
			defer agent.SetPresence(node.UUID, 1, false)
			CheckAndAutoRenewal(node)
			var saved models.Client
			if err := db.First(&saved, "uuid = ?", node.UUID).Error; err != nil {
				t.Fatal(err)
			}
			expected := node.ExpiredAt.ToTime()
			if test.wantRenew {
				expected = expected.AddDate(0, 0, test.cycle)
			}
			if !saved.ExpiredAt.ToTime().Equal(expected) {
				t.Fatalf("expire time: got %s want %s", saved.ExpiredAt.ToTime(), expected)
			}
		})
	}
}
