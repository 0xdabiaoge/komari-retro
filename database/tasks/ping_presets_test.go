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

func TestPingPresetCustomEditImportAndReplay(t *testing.T) {
	db := presetDB(t)
	edit := PingPresetOverride{ID: "global-google-ipv4", Name: " 自定义地址 ", Type: "http", Target: " https://example.com/check "}
	ids := []string{edit.ID, "global-cloudflare-ipv4"}
	result, err := importPingPresets(db, ids, []string{"node-a"}, false, 60, edit)
	if err != nil || result.Created != 2 {
		t.Fatalf("custom import: %+v %v", result, err)
	}
	var task models.PingTask
	db.First(&task, result.TaskIDs[0])
	if task.Name != "自定义地址" || task.Type != "http" || task.Target != "https://example.com/check" {
		t.Fatalf("custom edit lost: %+v", task)
	}
	edit.Name = "重复导入不应覆盖现有任务"
	result, err = importPingPresets(db, ids, []string{"node-b"}, false, 30, edit)
	if err != nil || result.Created != 0 || result.Updated != 2 {
		t.Fatalf("custom replay: %+v %v", result, err)
	}
	db.First(&task, task.Id)
	if task.Name != "自定义地址" || task.Interval != 60 || len(task.Clients) != 2 {
		t.Fatalf("history/config overwritten: %+v", task)
	}
	original, _ := pingpresets.Find(edit.ID)
	if original.Target != "8.8.8.8" || original.Type != "icmp" {
		t.Fatal("custom import mutated the built-in catalog")
	}
}

func TestPingPresetCustomEditValidationIsAtomic(t *testing.T) {
	db := presetDB(t)
	valid := PingPresetOverride{ID: "global-google-ipv4", Name: "自定义", Type: "icmp", Target: "9.9.9.9"}
	for _, edits := range [][]PingPresetOverride{
		{valid, valid},
		{{ID: "global-cloudflare-ipv6", Name: "未勾选", Type: "icmp", Target: "::1"}},
		{{ID: valid.ID, Name: "", Type: "icmp", Target: "9.9.9.9"}},
		{{ID: valid.ID, Name: "错误端口", Type: "tcp", Target: "example.com:99999"}},
		{{ID: valid.ID, Name: "无端口", Type: "tcp", Target: "example.com"}},
		{{ID: valid.ID, Name: "不支持的协议", Type: "udp", Target: "example.com:80"}},
		{{ID: valid.ID, Name: "错误 ICMP", Type: "icmp", Target: "https://example.com"}},
		{{ID: valid.ID, Name: "错误 HTTP", Type: "http", Target: "ftp://example.com"}},
		{{ID: valid.ID, Name: "包含凭据", Type: "http", Target: "https://user:pass@example.com"}},
	} {
		if _, err := importPingPresets(db, []string{valid.ID, "global-cloudflare-ipv4"}, []string{"node-a"}, false, 60, edits...); err == nil {
			t.Fatalf("invalid edit accepted: %+v", edits)
		}
		var count int64
		db.Model(&models.PingTask{}).Count(&count)
		if count != 0 {
			t.Fatal("invalid edit partially persisted tasks")
		}
	}
	for _, edit := range []PingPresetOverride{
		{ID: valid.ID, Name: "IPv6 ICMP", Type: "icmp", Target: "2001:4860:4860::8888"},
		{ID: valid.ID, Name: "IPv6 TCP", Type: "tcp", Target: "[2001:db8::1]:443"},
		{ID: valid.ID, Name: "HTTP query", Type: "http", Target: "https://example.com/check?q=1"},
	} {
		if err := validatePingPresetOverride(edit); err != nil {
			t.Fatalf("valid edit rejected: %+v %v", edit, err)
		}
	}
}

func TestPingPresetCustomHTTPPathsAreNotMerged(t *testing.T) {
	db := presetDB(t)
	ids := []string{"global-google-ipv4", "global-cloudflare-ipv4"}
	edits := []PingPresetOverride{
		{ID: ids[0], Name: "大写路径", Type: "http", Target: "https://example.com/Status?key=A"},
		{ID: ids[1], Name: "小写路径", Type: "http", Target: "https://example.com/status?key=A"},
	}
	result, err := importPingPresets(db, ids, []string{"node-a"}, false, 60, edits...)
	if err != nil || result.Created != 2 {
		t.Fatalf("distinct HTTP paths merged: %+v %v", result, err)
	}
	edits[0].Target = "https://EXAMPLE.com/Status?key=A"
	result, err = importPingPresets(db, ids[:1], []string{"node-a"}, false, 60, edits[:1]...)
	if err != nil || result.Created != 0 || result.Unchanged != 1 {
		t.Fatalf("HTTP hostname case did not merge: %+v %v", result, err)
	}
}
