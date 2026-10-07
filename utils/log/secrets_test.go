package log

import (
	"bytes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
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
