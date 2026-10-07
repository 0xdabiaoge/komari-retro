package dbcore

import (
	"fmt"
	"github.com/gofrs/flock"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var databaseLock *flock.Flock

func lockDatabase(filename string) error {
	if filename == ":memory:" || strings.Contains(filename, "mode=memory") {
		return nil
	}
	filename = strings.TrimPrefix(strings.SplitN(filename, "?", 2)[0], "file:")
	if value, err := url.PathUnescape(filename); err == nil {
		filename = value
	}
	path, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	parent := filepath.Dir(path)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	lockDir := filepath.Join(parent, ".komari-runtime")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return err
	}
	databaseLock = flock.New(filepath.Join(lockDir, filepath.Base(path)+".lock"))
	locked, err := databaseLock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("SQLite database is already used by another Komari process")
	}
	return nil
}
