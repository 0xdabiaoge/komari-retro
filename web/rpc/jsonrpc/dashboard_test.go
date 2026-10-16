package jsonrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

func checkDashboardContract(t *testing.T) {
	db := dbcore.GetDBInstance()
	jsonrpc.ResetDashboardCachesForTest()
	for _, method := range []string{"admin:getDashboardMetrics", "admin:getDashboardPing", "admin:getDatabaseStorage"} {
		for _, role := range []string{rpc.RoleGuest, rpc.RoleClient} {
			r := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: role, TempShareValid: true}, &rpc.JsonRpcRequest{ID: 1, Method: method})
			if r.Error == nil {
				t.Fatal("dashboard exposed to", role, method)
			}
		}
	}
	defer func() {
		for _, table := range []string{"records", "ping_records", "clients"} {
			key := "client"
			if table == "clients" {
				key = "uuid"
			}
			db.Exec("DELETE FROM " + table + " WHERE " + key + " LIKE 'dash-fixture-%'")
		}
		db.Exec("DELETE FROM ping_tasks WHERE id BETWEEN 880000 AND 880035")
		jsonrpc.ResetDashboardCachesForTest()
	}()
	now := time.Now().Add(-time.Minute)
	for i := 0; i < 120; i++ {
		id := fmt.Sprintf("dash-fixture-%03d", i)
		if err := db.Create(&models.Client{UUID: id, Token: id, Hidden: i == 119}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Record{Client: id, Time: models.FromTime(now), Cpu: 25, Ram: 256, RamTotal: 512, NetIn: 100, NetOut: 200, TrafficUp: 11, TrafficDown: 22}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for task := 0; task < 36; task++ {
		id := uint(880000 + task)
		if err := db.Create(&models.PingTask{Id: id, Name: "dashboard fixture"}).Error; err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 120; i++ {
			for n, value := range []int{10, -1, 30} {
				if task == 35 {
					value = -1
				}
				if err := db.Create(&models.PingRecord{Client: fmt.Sprintf("dash-fixture-%03d", i), TaskId: id, Time: models.FromTime(now.Add(time.Duration(n) * time.Second)), Value: value}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	call := func(method string, params any) *rpc.JsonRpcResponse {
		return jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{ID: 1, Method: method, Params: params})
	}
	r := call("admin:getDashboardMetrics", nil)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b, _ := json.Marshal(r.Result)
	var metrics struct {
		CPU, Memory []struct{ Value float64 }
		Traffic     struct {
			TotalUp, TotalDown float64
			Points             []any
			NodeTotals         []any
		}
	}
	if err := json.Unmarshal(b, &metrics); err != nil {
		t.Fatal(err)
	}
	if len(b) > 150000 || len(metrics.CPU) != 100 || len(metrics.Memory) != 100 || len(metrics.Traffic.Points) != 120 || len(metrics.Traffic.NodeTotals) != 100 || metrics.Traffic.TotalUp != 1320 || metrics.Traffic.TotalDown != 2640 || metrics.CPU[0].Value != 25 || metrics.Memory[0].Value != 50 {
		t.Fatalf("invalid compact aggregate: %s", b)
	}
	r = call("admin:getDashboardPing", nil)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b, _ = json.Marshal(r.Result)
	var ranks struct {
		Count                  int
		Stable, Unstable, Loss []struct {
			Valid int
			Loss  float64
			P95   *float64
			Ratio *float64 `json:"p99_p50_ratio"`
		}
	}
	json.Unmarshal(b, &ranks)
	if ranks.Count < 4320 || len(ranks.Stable) != 100 || len(ranks.Unstable) != 100 || len(ranks.Loss) != 100 || ranks.Loss[0].Valid != 0 || ranks.Loss[0].Loss != 100 || ranks.Loss[0].P95 != nil {
		t.Fatal("bad bounded ranking", ranks.Count, len(ranks.Stable), len(ranks.Loss))
	}
	found := false
	for _, item := range ranks.Stable {
		if item.P95 != nil && math.Abs(*item.P95-29) < .00001 && item.Ratio != nil && math.Abs(*item.Ratio-.49) < .00001 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected exact P95/volatility missing")
	}
	// Equal timestamps must use receipt order even when the last value is a
	// loss. A loss must never become a zero-ms latency or a stable ranking.
	for _, value := range []int{0, 10, 30, -1} {
		if err := db.Create(&models.PingRecord{Client: "dash-fixture-000", TaskId: 880000, Time: models.FromTime(now.Add(10 * time.Second)), Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	r = call("public:getPingMetricStats", map[string]any{"entity_id": "dash-fixture-000", "hours": 1})
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b, _ = json.Marshal(r.Result)
	var exact struct {
		Stats []struct {
			TaskID string `json:"task_id"`
			Total  int
			Valid  int
			Latest *float64
			P50    *float64
			P95    *float64
			P99    *float64
		}
	}
	json.Unmarshal(b, &exact)
	for _, stat := range exact.Stats {
		if stat.TaskID == "880000" {
			if stat.Total != 7 || stat.Valid != 5 || stat.Latest != nil || stat.P50 == nil || *stat.P50 != 10 || stat.P95 == nil || *stat.P95 != 30 || stat.P99 == nil || *stat.P99 != 30 {
				t.Fatal("incorrect tied latest, exact counts or percentiles", stat)
			}
		}
	}

	// A cached administrator aggregate must still reject a shared guest.
	if jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleGuest, TempShareValid: true}, &rpc.JsonRpcRequest{ID: 1, Method: "admin:getDashboardPing"}).Error == nil {
		t.Fatal("cached dashboard leaked")
	}
	r = call("public:queryMetrics", map[string]any{"entity_id": "dash-fixture-000", "hours": 1, "metric_keys": []string{"ping.latency_ms"}, "tags": map[string]string{"task_id": "880000"}})
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b, _ = json.Marshal(r.Result)
	var curves struct{ Series []jsonrpc.MetricSeriesItem }
	json.Unmarshal(b, &curves)
	if len(curves.Series) != 1 || curves.Series[0].Tags["task_id"] != "880000" {
		t.Fatal("selected task filter ignored")
	}
	// WAL reads must not occupy the single writer connection.
	rows, err := dbcore.GetReadDB().Raw("SELECT * FROM ping_records WHERE client='dash-fixture-000'").Rows()
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("missing reader fixture")
	}
	started := time.Now()
	err = db.Exec("UPDATE clients SET name='reader-write-check' WHERE uuid='dash-fixture-000'").Error
	rows.Close()
	if err != nil || time.Since(started) > time.Second {
		t.Fatal("reader blocked writer", err)
	}
	if dbcore.GetReadDB().Exec("DELETE FROM clients").Error == nil {
		t.Fatal("history reader is writable")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			r := call("admin:getDashboardMetrics", nil)
			if r.Error != nil {
				t.Errorf("concurrent dashboard: %v", r.Error)
			}
		})
	}
	wg.Wait()
}
