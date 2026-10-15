package api

import (
	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPrivateCookiesCannotDisableWebSocketOriginProtection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "origins.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	_ = config.SetMany(map[string]any{config.PrivateSiteKey: true, config.WsOriginCheckEnabledKey: false, config.WsAllowedOriginsKey: "*"})
	t.Setenv("KOMARI_WS_DISABLE_ORIGIN", "true")
	for _, cookie := range []string{"temp_key=fixture", "session_token=fixture", "admin_entrance_token=fixture"} {
		for _, origin := range []string{"https://foreign.invalid", "null", ""} {
			r := httptest.NewRequest("GET", "http://panel.invalid/api/rpc2", nil)
			r.Header.Set("Origin", origin)
			r.Header.Set("Cookie", cookie)
			if CheckWebSocketOrigin(r) {
				t.Fatal("private cookie bypassed socket origin validation")
			}
		}
		r := httptest.NewRequest("GET", "http://panel.invalid/api/rpc2", nil)
		r.Header.Set("Origin", "http://panel.invalid")
		r.Header.Set("Cookie", cookie)
		if !CheckWebSocketOrigin(r) {
			t.Fatal("same-origin private socket was denied")
		}
	}
}
