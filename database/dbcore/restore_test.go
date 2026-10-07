package dbcore

import (
	"archive/zip"
	"database/sql"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
	"testing"
)

func TestValidBackupActivatesAfterValidation(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "komari.db"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "replacement.db")
	db, err := sql.Open("sqlite3", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE configs (key TEXT PRIMARY KEY,value TEXT); INSERT INTO configs VALUES ('theme','\"default\"')"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "backup.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("komari.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	w, err = z.Create("komari-backup-markup")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("test backup"))
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := ValidateBackup(filepath.Join(dir, "backup.zip")); err != nil {
		t.Fatal(err)
	}
	if err := restorePendingBackup(dir); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite3", filepath.Join(dir, "komari.db"))
	if err != nil {
		t.Fatal(err)
	}
	var theme string
	if err := db.QueryRow("SELECT value FROM configs WHERE key='theme'").Scan(&theme); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if theme != `"default"` {
		t.Fatal("restored database incorrect", theme)
	}
	if err := completeRestore(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(journalPath(dir)); !os.IsNotExist(err) {
		t.Fatal("completed restore journal was not cleared")
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("rollback snapshot was not retained")
	}
}

func TestInvalidRestorePreservesData(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "komari.db")
	os.WriteFile(original, []byte("original"), 0600)
	archive, err := os.Create(filepath.Join(dir, "backup.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(archive)
	w, _ := z.Create("../escape")
	w.Write([]byte("bad"))
	z.Close()
	archive.Close()
	if err := restorePendingBackup(dir); err == nil {
		t.Fatal("invalid archive accepted")
	}
	got, err := os.ReadFile(original)
	if err != nil || string(got) != "original" {
		t.Fatal("original data changed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup.zip")); err != nil {
		t.Fatal("submitted archive lost")
	}
}
func TestInterruptedRestoreRollbackAndLock(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".runtime"), 0700)
	lockFile := filepath.Join(dir, ".runtime", "instance.lock")
	first := flock.New(lockFile)
	locked, err := first.TryLock()
	if err != nil || !locked {
		t.Fatal(err)
	}
	defer first.Unlock()
	os.WriteFile(filepath.Join(dir, "komari.db"), []byte("original"), 0600)
	staged := t.TempDir()
	os.WriteFile(filepath.Join(staged, "komari.db"), []byte("replacement"), 0600)
	os.WriteFile(filepath.Join(staged, "new-file"), []byte("new"), 0600)
	if err := activateBackup(staged, dir); err != nil {
		t.Fatal(err)
	}
	second := flock.New(lockFile)
	if locked, err := second.TryLock(); err != nil || locked {
		if locked {
			second.Unlock()
		}
		t.Fatal("restore replaced the lock inode")
	}
	if err := recoverInterruptedRestore(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "komari.db"))
	if err != nil || string(got) != "original" {
		t.Fatal("rollback lost original", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new-file")); !os.IsNotExist(err) {
		t.Fatal("replacement files survived rollback")
	}
	if err := recoverInterruptedRestore(dir); err != nil {
		t.Fatal("recovery is not idempotent", err)
	}
}
