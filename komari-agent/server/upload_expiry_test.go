package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUploadExpiryPreservesFreshAndActiveFiles(t *testing.T) {
	resetUploadStreamState(t)
	now := time.Now()
	dir := t.TempDir()
	for _, id := range []string{"expired", "fresh", "active"} {
		path := filepath.Join(dir, id+".part")
		if err := os.WriteFile(path, []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		created := now.Add(-3 * time.Hour)
		if id == "fresh" {
			created = now
		}
		uploadChunksMu.Lock()
		uploadChunks[id] = uploadChunkState{TempPath: path, CreatedAt: created}
		uploadChunksMu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	end, err := beginUploadStream("active", "stream", cancel)
	if err != nil {
		t.Fatal(err)
	}
	cleanupExpiredUploads(now)
	if _, err := os.Stat(filepath.Join(dir, "expired.part")); !os.IsNotExist(err) {
		t.Fatal("expired file survived", err)
	}
	for _, id := range []string{"fresh", "active"} {
		if _, err := os.Stat(filepath.Join(dir, id+".part")); err != nil {
			t.Fatal("live file removed", id, err)
		}
	}
	if ctx.Err() != nil {
		t.Fatal("active upload was canceled")
	}
	end()
	cleanupExpiredUploads(now)
	if _, err := os.Stat(filepath.Join(dir, "active.part")); !os.IsNotExist(err) {
		t.Fatal("idle expired file survived", err)
	}
}
