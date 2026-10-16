package dbcore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
)

// Small transactions let report/login writes run between retention batches.
// The budget bounds each maintenance pass; the next pass continues the backlog.
func DeleteHistoryBefore(db *gorm.DB, table string, before time.Time) error {
	switch table {
	case "records", "gpu_records", "records_long_term", "gpu_records_long_term", "ping_records":
	default:
		return fmt.Errorf("invalid history table")
	}
	deadline := time.Now().Add(time.Second)
	for batches := 0; batches < 100; batches++ {
		result := db.Exec("DELETE FROM "+table+" WHERE rowid IN (SELECT rowid FROM "+table+" WHERE time<? LIMIT 2000)", models.FromTime(before))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected < 2000 || time.Now().After(deadline) {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// New databases opt into incremental reclamation before schema creation.
// Existing NONE databases need an explicit offline VACUUM, never a surprise
// full rewrite while the service is running.
func ConfigureReclamation(db *gorm.DB) error {
	var tables int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tables).Error; err != nil {
		return err
	}
	if tables == 0 {
		if err := db.Exec("PRAGMA auto_vacuum=INCREMENTAL").Error; err != nil {
			return err
		}
		return db.Exec("VACUUM").Error
	}
	return nil
}

func ReclaimIdlePages(db *gorm.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	db = db.WithContext(ctx)
	var mode int
	if err := db.Raw("PRAGMA auto_vacuum").Scan(&mode).Error; err != nil {
		return err
	}
	if mode != 2 {
		return nil
	}
	// Bound relocation to at most 256 KiB per pass for 4 KiB pages.
	rows, err := db.Raw("PRAGMA incremental_vacuum(64)").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// CompactOffline shares the server's database lock and refuses a live database.
// It does not initialize accounts, settings, migrations, or background jobs.
func CompactOffline(filename string) error {
	if strings.ContainsAny(filename, "?\x00") || filename == ":memory:" || strings.HasPrefix(filename, "file:") {
		return fmt.Errorf("compact requires a plain SQLite file path")
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	if _, err = os.Stat(abs); err != nil {
		return err
	}
	if err = lockDatabase(abs); err != nil {
		return err
	}
	defer databaseLock.Unlock()
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(abs)+"?mode=rw&_busy_timeout=1000")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Integrity checks of multi-GB covering indexes otherwise thrash SQLite's
	// tiny default page cache. Bound the main page cache to 64 MiB.
	if _, err = db.Exec("PRAGMA cache_size=-65536; PRAGMA temp_store=FILE"); err != nil {
		return err
	}
	check := func() error {
		var result string
		if e := db.QueryRow("PRAGMA integrity_check").Scan(&result); e != nil {
			return e
		}
		if result != "ok" {
			return fmt.Errorf("database integrity check failed")
		}
		return nil
	}
	if err = check(); err != nil {
		return err
	}
	if _, err = db.Exec("PRAGMA auto_vacuum=INCREMENTAL"); err != nil {
		return err
	}
	if _, err = db.Exec("VACUUM"); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	var busy, frames, checkpointed int
	if err = db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return fmt.Errorf("database checkpoint is busy; close other SQLite readers and retry; retain WAL/SHM files")
	}
	return nil
}
