package dbcore

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRetentionAndOfflineReclamation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "retention.db")
	db, err := gorm.Open(sqlite.Open(file+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	if err = ConfigureReclamation(db); err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("CREATE TABLE ping_records (client TEXT,time TEXT,value INT,payload BLOB)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<12000) INSERT INTO ping_records SELECT 'fixture','2000-01-01 00:00:00',10,zeroblob(500) FROM n").Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO ping_records VALUES ('formal','2099-01-01 00:00:00',42,zeroblob(500))")
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	before, _ := os.Stat(file)
	if err = DeleteHistoryBefore(db, "ping_records", time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Raw("SELECT COUNT(*) FROM ping_records").Scan(&count)
	if count != 1 {
		t.Fatal("retention removed recent rows or left expired rows", count)
	}
	if err = DeleteHistoryBefore(db, "users", time.Now()); err == nil {
		t.Fatal("unsafe table accepted")
	}
	for i := 0; i < 300; i++ {
		if err = ReclaimIdlePages(db); err != nil {
			t.Fatal(err)
		}
	}
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	after, _ := os.Stat(file)
	if after.Size() >= before.Size()/2 {
		t.Fatalf("incremental reclaim ineffective: %d -> %d", before.Size(), after.Size())
	}
	pool, _ = db.DB()
	pool.Close()
	// A legacy database can be compacted only while its server lock is free.
	legacy := filepath.Join(t.TempDir(), "legacy.db")
	plain, _ := sql.Open("sqlite3", legacy)
	_, err = plain.Exec("CREATE TABLE users (name TEXT); INSERT INTO users VALUES ('formal'); CREATE TABLE history (payload BLOB); INSERT INTO history VALUES (zeroblob(5000000)); DELETE FROM history")
	if err != nil {
		t.Fatal(err)
	}
	plain.Close()
	info, _ := os.Stat(legacy)
	lockDir := filepath.Join(filepath.Dir(legacy), ".komari-runtime")
	os.MkdirAll(lockDir, 0700)
	l := flock.New(filepath.Join(lockDir, filepath.Base(legacy)+".lock"))
	if ok, e := l.TryLock(); !ok || e != nil {
		t.Fatal(e)
	}
	if err = CompactOffline(legacy); err == nil {
		t.Fatal("offline compact accepted active service lock")
	}
	l.Unlock()
	if err = CompactOffline(legacy); err != nil {
		t.Fatal(err)
	}
	newInfo, _ := os.Stat(legacy)
	if newInfo.Size() >= info.Size()/2 {
		t.Fatal("legacy file did not shrink")
	}
	plain, _ = sql.Open("sqlite3", legacy)
	defer plain.Close()
	var name string
	var mode int
	plain.QueryRow("SELECT name FROM users").Scan(&name)
	plain.QueryRow("PRAGMA auto_vacuum").Scan(&mode)
	if name != "formal" || mode != 2 {
		t.Fatal("compact changed business data or failed to enable reclamation")
	}
}
