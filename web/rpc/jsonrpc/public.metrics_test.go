package jsonrpc_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

func checkMetricQueryContract(t *testing.T) {
	db := dbcore.GetDBInstance()
	end := time.Now().Add(-time.Minute).Truncate(time.Second)
	start := end.Add(-time.Minute)
	for _, node := range []models.Client{{UUID: "metric-fixture", Token: "metric-fixture-token", Name: "metric-fixture"}, {UUID: "metric-other", Token: "metric-other-token", Name: "metric-other", Hidden: true}} {
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		db.Where("id = ?", 123456).Delete(&models.PingTask{})
		db.Where("client IN ?", []string{"metric-fixture", "metric-other"}).Delete(&models.Record{})
		db.Where("client = ?", "metric-fixture").Delete(&models.GPURecord{})
		db.Where("client = ?", "metric-fixture").Delete(&models.PingRecord{})
		db.Where("uuid IN ?", []string{"metric-fixture", "metric-other"}).Delete(&models.Client{})
	})
	for i, value := range []float32{10, 30} {
		if err := db.Create(&models.Record{Client: "metric-fixture", Time: models.FromTime(end.Add(time.Duration(-20+i*10) * time.Second)), Cpu: value, Ram: 256, RamTotal: 512, Load: 2, Disk: 123, DiskTotal: 456, Swap: 10, SwapTotal: 20, NetIn: 111, NetOut: 222, Process: 7, Connections: 8, ConnectionsUdp: 9}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.Record{Client: "metric-other", Time: models.FromTime(end.Add(-10 * time.Second)), Cpu: 999}).Error; err != nil {
		t.Fatal(err)
	}
	for i, value := range []float32{7, 11} {
		if err := db.Create(&models.GPURecord{Client: "metric-fixture", Time: models.FromTime(end.Add(-10 * time.Second)), DeviceIndex: i, DeviceName: "fixture-gpu", Utilization: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []int{10, -1, 30} {
		if err := db.Create(&models.PingRecord{Client: "metric-fixture", TaskId: 123456, Time: models.FromTime(end.Add(-10 * time.Second)), Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.PingTask{Id: 123456, Name: "metric-fixture-ping", Clients: models.StringArray{"metric-fixture"}, Interval: 10}).Error; err != nil {
		t.Fatal(err)
	}
	query := func(params map[string]any, role string) *rpc.JsonRpcResponse {
		return jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: role}, &rpc.JsonRpcRequest{Method: "public:queryMetrics", ID: 1, Params: params})
	}
	params := map[string]any{"entity_id": "metric-fixture", "start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "metric_keys": []string{"cpu.usage"}, "max_points": 1}
	for aggregation, want := range map[string]float64{"avg": 20, "min": 10, "max": 30, "first": 10, "last": 30, "sum": 40, "stddev": 10, "p70": 24, "p95": 29, "p99": 29.8} {
		params["aggregation"] = aggregation
		response := query(params, rpc.RoleAdmin)
		if response.Error != nil {
			t.Fatal(response.Error)
		}
		var result struct {
			Series []jsonrpc.MetricSeriesItem `json:"series"`
		}
		payload, _ := json.Marshal(response.Result)
		if err := json.Unmarshal(payload, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Series) != 1 || result.Series[0].EntityID != "metric-fixture" || len(result.Series[0].Points) != 1 || result.Series[0].Points[0].Value == nil || math.Abs(*result.Series[0].Points[0].Value-want) > 0.00001 {
			t.Fatalf("%s: want %v, got %s", aggregation, want, payload)
		}
	}
	params["aggregation"] = "avg"
	// Offset timestamps describe the same instants and must not shift the SQL window.
	zone := time.FixedZone("test-offset", 8*60*60)
	params["start"] = start.In(zone).Format(time.RFC3339)
	params["end"] = end.In(zone).Format(time.RFC3339)
	responseOffset := query(params, rpc.RoleAdmin)
	payloadOffset, _ := json.Marshal(responseOffset.Result)
	var offsetResult struct {
		Series []jsonrpc.MetricSeriesItem `json:"series"`
	}
	_ = json.Unmarshal(payloadOffset, &offsetResult)
	if responseOffset.Error != nil || len(offsetResult.Series) != 1 || *offsetResult.Series[0].Points[0].Value != 20 {
		t.Fatalf("offset timestamp query: %s %v", payloadOffset, responseOffset.Error)
	}
	params["start"] = start.Format(time.RFC3339)
	params["end"] = end.Format(time.RFC3339)
	params["aggregation_by_metric"] = map[string]string{"cpu.usage": "sum"}
	response := query(params, rpc.RoleAdmin)
	payload, _ := json.Marshal(response.Result)
	var override struct {
		Series []jsonrpc.MetricSeriesItem `json:"series"`
	}
	_ = json.Unmarshal(payload, &override)
	if response.Error != nil || len(override.Series) != 1 || *override.Series[0].Points[0].Value != 40 {
		t.Fatalf("aggregation override: %s", payload)
	}
	delete(params, "aggregation_by_metric")
	params["max_points"] = 2
	params["fill_empty"] = true
	response = query(params, rpc.RoleAdmin)
	payload, _ = json.Marshal(response.Result)
	var result struct {
		Series []jsonrpc.MetricSeriesItem `json:"series"`
	}
	_ = json.Unmarshal(payload, &result)
	if response.Error != nil || len(result.Series) != 1 || len(result.Series[0].Points) != 2 || result.Series[0].Points[0].Value != nil {
		t.Fatalf("empty buckets/max points: %s, %v", payload, response.Error)
	}
	params["fill_empty"] = false
	params["max_points"] = 1
	params["metric_keys"] = []string{"gpu.device.usage", "ping.latency_ms", "memory.total", "load.average", "net.in.rate", "disk.used", "swap.used", "connections.udp"}
	response = query(params, rpc.RoleAdmin)
	payload, _ = json.Marshal(response.Result)
	_ = json.Unmarshal(payload, &result)
	if response.Error != nil || len(result.Series) != 9 {
		t.Fatalf("metric catalog projections: %s, %v", payload, response.Error)
	}
	for _, series := range result.Series {
		if series.EntityID != "metric-fixture" {
			t.Fatal("cross-node series")
		}
		if series.MetricKey == "ping.latency_ms" && (series.Tags["task_id"] != "123456" || *series.Points[0].Value != 20) {
			t.Fatalf("ping loss/tags: %+v", series)
		}
	}
	params["entity_id"] = "metric-other"
	response = query(params, rpc.RoleGuest)
	payload, _ = json.Marshal(response.Result)
	_ = json.Unmarshal(payload, &result)
	if response.Error != nil || len(result.Series) != 0 {
		t.Fatalf("hidden node leaked: %s", payload)
	}
	params = map[string]any{"entity_id": "metric-fixture", "hours": 10.0 / 60, "metric_keys": []string{"cpu.usage"}, "max_points": 7}
	response = query(params, rpc.RoleAdmin)
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	for _, bad := range []map[string]any{{"hours": -1}, {"hours": 745}, {"start": "invalid"}, {"max_points": 10001}, {"aggregation": "injected"}, {"metric_keys": []string{"injected"}}} {
		if query(bad, rpc.RoleAdmin).Error == nil {
			t.Fatalf("invalid query accepted: %v", bad)
		}
	}
	stats := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{Method: "public:getPingMetricStats", ID: 1, Params: map[string]any{"entity_id": "metric-fixture", "start": start.In(zone).Format(time.RFC3339), "end": end.In(zone).Format(time.RFC3339), "hours": 10.0 / 60}})
	payload, _ = json.Marshal(stats.Result)
	var statsResult struct {
		Stats []struct {
			EntityID string `json:"entity_id"`
			Total    int    `json:"total"`
			Valid    int    `json:"valid"`
		} `json:"stats"`
	}
	_ = json.Unmarshal(payload, &statsResult)
	if stats.Error != nil || len(statsResult.Stats) != 1 || statsResult.Stats[0].EntityID != "metric-fixture" || statsResult.Stats[0].Total != 3 || statsResult.Stats[0].Valid != 2 {
		t.Fatalf("scoped ping statistics: %s %v", payload, stats.Error)
	}
}
