package log

import (
	"bytes"
	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

func TestSQLLogsDoNotExpandBoundSecrets(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(original)
	logger := NewGormLogger()
	logger.SlowThreshold = -1 // force tracing even when the platform clock reports zero elapsed time
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE secrets (token TEXT)")
	secret := "private-api-secret-must-not-log"
	db.Exec("INSERT INTO secrets(token) VALUES (?)", secret)
	if strings.Contains(output.String(), secret) {
		t.Fatal("SQL log contains credential")
	}
	if !strings.Contains(output.String(), "INSERT") {
		t.Fatal("test never exercised SQL logger")
	}
}

func TestPrivateCapabilityRoutesAreRedacted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.SetMany(map[string]any{config.AdminPathKey: "/entry-private-canary", config.AdminViewPathKey: "/view-private-canary"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/entry-private-canary", "/view-private-canary", "/entry-private-canary/nested", "/s/share-private-canary"} {
		if strings.Contains(safeLogPath(path), "canary") {
			t.Fatal("capability path was logged")
		}
	}
	query := safeLogQuery(url.Values{"token": {"agent-canary"}, "temp_key": {"share-canary"}, "preview_token": {"preview-canary"}, "page": {"2"}})
	if strings.Contains(query, "canary") || !strings.Contains(query, "page=2") {
		t.Fatal("query redaction failed")
	}
}
