package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"sync"
	"testing"
)

func TestSnapshotInvalidationAndIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	SetDb(db)
	if err := Set("object", map[string]any{"value": "original"}); err != nil {
		t.Fatal(err)
	}
	object, err := GetAs[map[string]string]("object")
	if err != nil {
		t.Fatal(err)
	}
	object["value"] = "caller mutation"
	again, _ := GetAs[map[string]string]("object")
	if again["value"] != "original" {
		t.Fatal("caller mutated snapshot")
	}
	if err := SetMany(map[string]any{"object": map[string]any{"value": "changed"}}); err != nil {
		t.Fatal(err)
	}
	again, _ = GetAs[map[string]string]("object")
	if again["value"] != "changed" {
		t.Fatal("stale snapshot")
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := Get("object"); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
