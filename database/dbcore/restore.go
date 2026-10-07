package dbcore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/pkg/archiveutil"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backups have a fixed komari.db destination. Refuse custom database paths
// rather than report a restore that the running process will never read.
func RestoreSupported() bool {
	path := strings.TrimPrefix(strings.SplitN(flags.DatabaseFile, "?", 2)[0], "file:")
	actual, err := filepath.Abs(path)
	standard, standardErr := filepath.Abs("./data/komari.db")
	return flags.IsSQLite() && err == nil && standardErr == nil && actual == standard
}

func ValidateBackup(filename string) error {
	staged, err := os.MkdirTemp(filepath.Dir(filename), "restore-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	return extractBackup(filename, staged)
}

func extractBackup(filename, staged string) error {
	if err := archiveutil.Extract(filename, staged, archiveutil.BackupLimits); err != nil {
		return err
	}
	for _, name := range []string{".runtime", ".komari-runtime", "backup.zip"} {
		if _, err := os.Lstat(filepath.Join(staged, name)); err == nil {
			return fmt.Errorf("reserved runtime path in backup")
		}
	}
	if _, err := os.Stat(filepath.Join(staged, "komari-backup-markup")); err != nil {
		return fmt.Errorf("backup marker missing")
	}
	dbPath := filepath.Join(staged, "komari.db")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("backup database missing")
	}
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil || result != "ok" {
		return fmt.Errorf("backup database integrity check failed: %v", err)
	}
	var table string
	if err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='configs'").Scan(&table); err != nil {
		return fmt.Errorf("backup is not a Komari database")
	}
	return nil
}

func restorePendingBackup(dataDir string) error {
	filename := filepath.Join(dataDir, "backup.zip")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	staged, err := os.MkdirTemp(filepath.Dir(dataDir), "restore-staged-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	if err = extractBackup(filename, staged); err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(staged, "komari-backup-markup")); err != nil {
		return err
	}
	return activateBackup(staged, dataDir)
}

// The data directory itself and its lock inode never move. A durable journal
// permits rollback after process termination between any of the renames.
type restoreJournal struct {
	Previous           string `json:"previous"`
	Phase              string `json:"phase"`
	Original, Incoming []string
}

func journalPath(dataDir string) string { return filepath.Join(dataDir, ".runtime", "restore.json") }
func saveJournal(dataDir string, j restoreJournal) error {
	content, err := json.Marshal(j)
	if err != nil {
		return err
	}
	filename := journalPath(dataDir)
	f, err := os.OpenFile(filename+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Rename replacement works on supported platforms; journal is never truncated.
	return os.Rename(filename+".tmp", filename)
}
func activateBackup(staged, dataDir string) error {
	runtimeDir := filepath.Join(dataDir, ".runtime")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return err
	}
	old, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}
	incoming, err := os.ReadDir(staged)
	if err != nil {
		return err
	}
	previous := "previous-" + fmt.Sprint(time.Now().UnixNano())
	previousPath := filepath.Join(runtimeDir, previous)
	if err := os.Mkdir(previousPath, 0700); err != nil {
		return err
	}
	j := restoreJournal{Previous: previous, Phase: "moving"}
	for _, e := range old {
		if e.Name() != ".runtime" && e.Name() != ".komari-runtime" {
			j.Original = append(j.Original, e.Name())
		}
	}
	for _, e := range incoming {
		j.Incoming = append(j.Incoming, e.Name())
	}
	if err := saveJournal(dataDir, j); err != nil {
		return err
	}
	for _, name := range j.Original {
		if err := os.Rename(filepath.Join(dataDir, name), filepath.Join(previousPath, name)); err != nil {
			return err
		}
	}
	j.Phase = "installing"
	if err := saveJournal(dataDir, j); err != nil {
		return err
	}
	for _, name := range j.Incoming {
		if err := os.Rename(filepath.Join(staged, name), filepath.Join(dataDir, name)); err != nil {
			return err
		}
	}
	return nil // Startup migrations must succeed before completeRestore clears the journal.
}
func completeRestore(dataDir string) error {
	err := os.Remove(journalPath(dataDir))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func recoverInterruptedRestore(dataDir string) error {
	content, err := os.ReadFile(journalPath(dataDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var j restoreJournal
	if err := json.Unmarshal(content, &j); err != nil {
		return err
	}
	if !filepath.IsLocal(j.Previous) || filepath.Base(j.Previous) != j.Previous {
		return fmt.Errorf("invalid restore journal")
	}
	previous := filepath.Join(dataDir, ".runtime", j.Previous)
	failed, err := os.MkdirTemp(filepath.Join(dataDir, ".runtime"), "failed-")
	if err != nil {
		return err
	}
	for _, name := range append(append([]string{}, j.Original...), j.Incoming...) {
		if !filepath.IsLocal(name) || filepath.Base(name) != name || name == ".runtime" {
			return fmt.Errorf("invalid restore journal entry")
		}
	}
	if j.Phase == "installing" {
		for _, name := range j.Incoming {
			alreadyRestored := false
			for _, oldName := range j.Original {
				if oldName == name {
					if _, err := os.Lstat(filepath.Join(previous, name)); os.IsNotExist(err) {
						alreadyRestored = true
					}
					break
				}
			}
			if alreadyRestored {
				continue
			}
			src := filepath.Join(dataDir, name)
			if _, err := os.Lstat(src); err == nil {
				if err := os.Rename(src, filepath.Join(failed, name)); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	} else if j.Phase != "moving" {
		return fmt.Errorf("invalid restore phase")
	}
	for _, name := range j.Original {
		src := filepath.Join(previous, name)
		if _, err := os.Lstat(src); err == nil {
			if err := os.Rename(src, filepath.Join(dataDir, name)); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Keep the submitted archive as evidence rather than repeating a failed migration.
	backup := filepath.Join(dataDir, "backup.zip")
	if _, err := os.Stat(backup); err == nil {
		if err := os.Rename(backup, filepath.Join(failed, "submitted-backup.zip")); err != nil {
			return err
		}
	}
	if err := completeRestore(dataDir); err != nil {
		return err
	}
	log.Printf("Interrupted backup restore rolled back; previous data retained")
	return nil
}
