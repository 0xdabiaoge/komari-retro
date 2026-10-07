// Package archiveutil validates archives in a fresh staging directory before activation.
package archiveutil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Limits struct {
	Files       int
	Total, File uint64
}

var BackupLimits = Limits{20000, 8 << 30, 4 << 30}
var ThemeLimits = Limits{10000, 256 << 20, 64 << 20}

func Extract(filename, dest string, limits Limits) error {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer archive.Close()
	if len(archive.File) > limits.Files {
		return fmt.Errorf("too many archive entries")
	}
	seen := make(map[string]bool)
	var total uint64
	for _, entry := range archive.File {
		name := filepath.FromSlash(strings.ReplaceAll(entry.Name, "\\", "/"))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe archive path")
		}
		key := strings.ToLower(filepath.Clean(name))
		if seen[key] {
			return fmt.Errorf("duplicate archive path")
		}
		seen[key] = true
		if entry.Mode()&os.ModeSymlink != 0 || (!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
			return fmt.Errorf("unsupported archive entry type")
		}
		if entry.UncompressedSize64 > limits.File || entry.UncompressedSize64 > limits.Total-total {
			return fmt.Errorf("archive expansion limit exceeded")
		}
		total += entry.UncompressedSize64
		path := filepath.Join(dest, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			reader.Close()
			return err
		}
		n, copyErr := io.Copy(file, io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
		closeErr := file.Close()
		reader.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if uint64(n) != entry.UncompressedSize64 {
			return fmt.Errorf("archive size mismatch")
		}
	}
	return nil
}

// Activate keeps the previous directory until the caller has verified startup.
func Activate(staged, target string) (string, error) {
	previous := filepath.Join(filepath.Dir(target), fmt.Sprintf(".%s.previous-%d", filepath.Base(target), time.Now().UnixNano()))
	exists := false
	if _, err := os.Stat(target); err == nil {
		exists = true
		if err = os.Rename(target, previous); err != nil {
			return "", err
		}
	}
	if err := os.Rename(staged, target); err != nil {
		if exists {
			if rollbackErr := os.Rename(previous, target); rollbackErr != nil {
				return previous, fmt.Errorf("activation failed: %v; rollback failed: %w", err, rollbackErr)
			}
		}
		return "", err
	}
	if exists {
		return previous, nil
	}
	return "", nil
}
