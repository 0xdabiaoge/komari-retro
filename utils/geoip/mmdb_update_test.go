package geoip

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedMMDBUpdatePreservesFileAndReleasesLocks(t *testing.T) {
	originalURL := GeoIpUrl
	t.Cleanup(func() { GeoIpUrl = originalURL })
	path := filepath.Join(t.TempDir(), "country.mmdb")
	original := []byte("existing database must survive failed updates")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	service := &MaxMindGeoIPService{dbFilePath: path}
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte("invalid database"))
		}))
		GeoIpUrl = server.URL
		if err := service.UpdateDatabase(); err == nil {
			t.Fatal("invalid download accepted")
		}
		server.Close()
		if !service.mu.TryRLock() {
			t.Fatal("failed update left lookup lock held")
		}
		service.mu.RUnlock()
		if !service.updateMu.TryLock() {
			t.Fatal("failed update left update lock held")
		}
		service.updateMu.Unlock()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != string(original) {
			t.Fatalf("old database changed after failure: %q %v", data, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	GeoIpUrl = server.URL
	server.Close()
	if err := service.UpdateDatabase(); err == nil {
		t.Fatal("transport failure accepted")
	}
	if !service.mu.TryRLock() {
		t.Fatal("transport failure left lookup lock held")
	}
	service.mu.RUnlock()
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".geoip-update-*"))
	if len(files) != 0 {
		t.Fatalf("staged files leaked: %v", files)
	}
}
