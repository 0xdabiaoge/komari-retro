package cmd

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/geoip"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGeoIPEnabledToggleReinitializesUnchangedProvider(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	config.SetDb(db)
	require.NoError(t, config.SetMany(map[string]any{"geo_ip_enabled": false, "geo_ip_provider": "ipinfo"}))
	geoip.InitGeoIp()
	config.Subscribe(refreshRuntimeSettings)
	waitFor := func(name string) {
		t.Helper()
		require.Eventually(t, func() bool { return geoip.ProviderName() == name }, time.Second*3, time.Millisecond*5)
	}
	// These constructors need no network: only the enabled key changes, and
	// each round verifies the actual subscribed runtime transition.
	for i := 0; i < 3; i++ {
		require.NoError(t, config.Set("geo_ip_enabled", true))
		waitFor("ipinfo.io")
		require.NoError(t, config.Set("geo_ip_enabled", false))
		waitFor("EmptyProvider")
	}
	require.NoError(t, config.SetMany(map[string]any{"geo_ip_enabled": true, "geo_ip_provider": "geojs"}))
	waitFor("geojs.io")
	require.NoError(t, config.Set("geo_ip_enabled", false))
	waitFor("EmptyProvider")
}
