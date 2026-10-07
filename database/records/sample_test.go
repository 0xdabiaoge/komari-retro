package records

import (
	"context"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"path/filepath"
	"testing"
	"time"
)

func TestSampleLoadAcrossRetentionBoundary(t *testing.T) {
	t.Chdir(t.TempDir())
	flags.DatabaseFile = filepath.Join(t.TempDir(), "sample.db")
	flags.DatabaseType = "sqlite"
	db := dbcore.GetDBInstance()
	t.Cleanup(func() { _ = dbcore.Close() })
	end := time.Now().Truncate(time.Second)
	for _, node := range []string{"a", "b"} {
		if err := db.Create(&models.Client{UUID: node, Name: node, Token: node}).Error; err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 720; i++ {
			at := end.Add(-time.Duration(i) * time.Minute)
			rec := models.Record{Client: node, Time: models.LocalTime(at), Cpu: float32(i), Gpu: 42}
			table := "records"
			if i > 240 {
				table = "records_long_term"
			}
			if err := db.Table(table).Create(&rec).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := SampleLoad(context.Background(), "a", end.Add(-12*time.Hour), end, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 30 || len(got) > 50 {
		t.Fatalf("unexpected sample count %d", len(got))
	}
	if time.Time(got[0].Time).After(end.Add(-8 * time.Hour)) {
		t.Fatal("long-term rows missing")
	}
	for _, rec := range got {
		if rec.Client != "a" || rec.Gpu != 42 {
			t.Fatalf("cross-node or damaged sample %+v", rec)
		}
	}
	got, err = SampleLoad(context.Background(), "", end.Add(-12*time.Hour), end, 100)
	if err != nil || len(got) > 100 {
		t.Fatal(len(got), err)
	}
	seen := map[string]bool{}
	for _, rec := range got {
		seen[rec.Client] = true
	}
	if len(seen) != 2 {
		t.Fatal("multi-node sampling lost a node")
	}
	if _, err = SampleLoad(context.Background(), "a", end.Add(-32*24*time.Hour), end, 100); err == nil {
		t.Fatal("unbounded window accepted")
	}
}
