//go:build !windows

package cloudflared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManagedProcessLifecycleAndEncryptedToken(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("fixture.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	const token = "isolated-managed-tunnel-secret"
	if err := SaveToken(token); err != nil {
		t.Fatal(err)
	}
	raw, err := config.GetAs[string](config.CloudflareTunnelTokenKey)
	if err != nil || raw == token || strings.Contains(raw, token) {
		t.Fatal("token stored in plaintext")
	}
	loaded, err := LoadToken()
	if err != nil || loaded != token {
		t.Fatal("token roundtrip failed")
	}
	path := filepath.Join(t.TempDir(), "cloudflared-fixture")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'token=%s\\n' \"$TUNNEL_TOKEN\"\ntrap 'exit 0' TERM\nwhile :; do sleep 0.1; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KOMARI_CLOUDFLARED_BIN", path)
	m := &manager{maxLogLines: 80}
	if err := m.start(token); err != nil {
		t.Fatal(err)
	}
	defer m.stop()
	if err := m.start(token); err == nil {
		t.Fatal("duplicate process started")
	}
	for i := 0; i < 50; i++ {
		if strings.Contains(strings.Join(m.status().Logs, " "), "[redacted]") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := m.status()
	if !status.Running || strings.Contains(strings.Join(status.Logs, " "), token) {
		t.Fatal("managed process status invalid")
	}
	if err := m.stop(); err != nil || m.status().Running {
		t.Fatalf("process did not stop: %v", err)
	}
	if err := m.start(token); err != nil {
		t.Fatalf("process did not restart: %v", err)
	}
}
