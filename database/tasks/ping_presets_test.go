package tasks

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/pingpresets"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func presetDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "presets.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Client{}, &models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	for _, id := range []string{"node-a", "node-b"} {
		if err := db.Create(&models.Client{UUID: id, Token: id, Name: id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPingPresetImportReplayAndMerge(t *testing.T) {
	db := presetDB(t)
	ids := []string{"global-cloudflare-ipv4", "global-google-ipv4", "global-google-ipv4"}
	first, err := importPingPresets(db, ids, []string{"node-a", "node-a"}, false, 60)
	if err != nil || first.Created != 2 {
		t.Fatalf("first import: %+v %v", first, err)
	}
	var task models.PingTask
	db.First(&task, first.TaskIDs[0])
	db.Model(&task).Updates(map[string]any{"name": "已有自定义名称", "interval": 120})
	again, err := importPingPresets(db, ids, []string{"node-a"}, false, 60)
	if err != nil || again.Created != 0 || again.Unchanged != 2 {
		t.Fatalf("replay: %+v %v", again, err)
	}
	merged, err := importPingPresets(db, ids, []string{"node-b"}, true, 30)
	if err != nil || merged.Updated != 2 {
		t.Fatalf("merge: %+v %v", merged, err)
	}
	db.First(&task, first.TaskIDs[0])
	if task.Name != "已有自定义名称" || task.Interval != 120 || !task.DefaultOn || len(task.Clients) != 2 {
		t.Fatalf("custom settings lost: %+v", task)
	}
	_, err = importPingPresets(db, ids, []string{"node-a"}, false, 60)
	db.First(&task, first.TaskIDs[0])
	if err != nil || !task.DefaultOn {
		t.Fatal("replay disabled an existing default")
	}
}

func TestPingPresetImportValidationAndRollback(t *testing.T) {
	db := presetDB(t)
	for _, tc := range []struct {
		ids, clients []string
		defaultOn    bool
		interval     int
	}{
		{nil, []string{"node-a"}, false, 60},
		{[]string{"global-google-ipv4", "unknown"}, []string{"node-a"}, false, 60},
		{[]string{"global-google-ipv4"}, []string{"missing-node"}, false, 60},
		{[]string{"global-google-ipv4"}, nil, false, 60},
		{[]string{"global-google-ipv4"}, nil, true, 1},
	} {
		if _, err := importPingPresets(db, tc.ids, tc.clients, tc.defaultOn, tc.interval); err == nil {
			t.Fatal("invalid import accepted")
		}
	}
	var count int64
	db.Model(&models.PingTask{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid selection partially persisted")
	}
	if err := db.Callback().Create().Before("gorm:create").Register("preset-test-failure", func(tx *gorm.DB) {
		if task, ok := tx.Statement.Dest.(*models.PingTask); ok && task.Target == "8.8.8.8" {
			tx.AddError(errors.New("isolated injected write failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	res, err := importPingPresets(db, []string{"global-cloudflare-ipv4", "global-google-ipv4"}, []string{"node-a"}, false, 60)
	db.Model(&models.PingTask{}).Count(&count)
	if err == nil || count != 0 || res.Created != 0 {
		t.Fatalf("transaction did not roll back: %+v %v count=%d", res, err, count)
	}
}

func TestPingPresetFullCatalogCanBeSavedWithoutActiveClients(t *testing.T) {
	db := presetDB(t)
	ids := []string{}
	for _, p := range pingpresets.Catalog.Entries {
		ids = append(ids, p.ID)
	}
	res, err := importPingPresets(db, ids, nil, true, 60)
	if err != nil || res.Created != len(ids) {
		t.Fatalf("full catalog: %+v %v", res, err)
	}
	var rows []models.PingTask
	db.Find(&rows)
	for _, row := range rows {
		if row.Clients == nil || !row.DefaultOn {
			t.Fatal("future-only task must retain [] and default_on")
		}
	}
}
