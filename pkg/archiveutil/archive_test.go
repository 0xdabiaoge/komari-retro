package archiveutil

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func archiveFixture(t *testing.T, names []string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "fixture.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("payload"))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return name
}
func TestRejectUnsafeArchives(t *testing.T) {
	for _, names := range [][]string{{"../escape"}, {"..\\escape"}, {"/absolute"}, {"same", "SAME"}, {"file"}} {
		t.Run(names[0], func(t *testing.T) {
			limits := ThemeLimits
			if names[0] == "file" {
				limits.File = 1
			}
			if err := Extract(archiveFixture(t, names), t.TempDir(), limits); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
func TestFailedActivationRetainsOriginal(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "theme")
	os.Mkdir(target, 0700)
	os.WriteFile(filepath.Join(target, "index.html"), []byte("original"), 0600)
	if _, err := Activate(filepath.Join(parent, "missing"), target); err == nil {
		t.Fatal("missing staged data accepted")
	}
	got, err := os.ReadFile(filepath.Join(target, "index.html"))
	if err != nil || string(got) != "original" {
		t.Fatal("original theme lost", err)
	}
}
