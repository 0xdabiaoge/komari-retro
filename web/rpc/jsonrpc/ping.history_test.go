package jsonrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/pkg/share"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

// A full day at the real production task count crosses the old 50,000-row cap.
func checkFullDayPingHistory(t *testing.T) {
	db := dbcore.GetDBInstance()
	ids := []string{"full-day-visible", "full-day-hidden", "full-day-outside"}
	for i, id := range ids {
		if err := db.Create(&models.Client{UUID: id, Token: id, Hidden: i == 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		db.Exec("DELETE FROM ping_records WHERE client IN ?", ids)
		db.Exec("DELETE FROM ping_tasks WHERE id BETWEEN 970000 AND 970035")
		db.Exec("DELETE FROM clients WHERE uuid IN ?", ids)
	}()
	for i := 0; i < 36; i++ {
		if err := db.Create(&models.PingTask{Id: uint(970000 + i), Name: fmt.Sprintf("full-day-task-%02d", i), Clients: models.StringArray(ids), Type: "icmp", Target: "PRIVATE_TARGET", Interval: 60}).Error; err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now().Add(-24*time.Hour + 30*time.Second)
	if err := db.Exec(`WITH RECURSIVE minutes(n) AS (SELECT 0 UNION ALL SELECT n+1 FROM minutes WHERE n<1439)
 INSERT INTO ping_records(client,task_id,time,value)
 SELECT c.uuid,t.id,strftime('%Y-%m-%d %H:%M:%f',?,printf('+%d seconds',n*60)),
 CASE n%4 WHEN 0 THEN 0 WHEN 1 THEN 20 WHEN 2 THEN -1 ELSE 40 END
 FROM minutes CROSS JOIN clients c CROSS JOIN ping_tasks t
 WHERE c.uuid IN ? AND t.id BETWEEN 970000 AND 970035`, models.FromTime(start), ids).Error; err != nil {
		t.Fatal(err)
	}
	type result struct {
		Count   int
		Records []struct {
			Client string
			TaskID uint `json:"task_id"`
			Time   time.Time
			Value  int
		}
		Tasks []struct {
			ID                              uint
			Total                           int
			Loss                            float64
			Avg, Min, Max, Latest, P50, P99 int
		}
	}
	check := func(method string, params map[string]any, meta *rpc.ContextMeta, budget int) result {
		t.Helper()
		r := jsonrpc.Dispatch(context.Background(), meta, &rpc.JsonRpcRequest{ID: 1, Method: method, Params: params})
		if r.Error != nil {
			t.Fatalf("%s: %v", method, r.Error)
		}
		b, _ := json.Marshal(r.Result)
		var v result
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		if v.Count != len(v.Records) || v.Count > budget || v.Count == 0 || len(b) > 4<<20 || strings.Contains(string(b), "PRIVATE_TARGET") {
			t.Fatalf("unbounded or empty history: records=%d bytes=%d", v.Count, len(b))
		}
		return v
	}
	admin := &rpc.ContextMeta{Permission: rpc.RoleAdmin}
	for _, budget := range []int{4000, 10000, 36} {
		v := check("common:getRecords", map[string]any{"uuid": ids[0], "type": "ping", "hours": 24, "maxCount": budget}, admin, budget)
		if len(v.Tasks) != 36 {
			t.Fatal("task lost", len(v.Tasks))
		}
		seen := map[uint]bool{}
		for _, r := range v.Records {
			if r.Client != ids[0] {
				t.Fatal("cross-node sample")
			}
			seen[r.TaskID] = true
		}
		if len(seen) != 36 {
			t.Fatal("sampling starved a task", len(seen))
		}
		for _, s := range v.Tasks {
			if s.Total != 1440 || s.Loss != 25 || s.Avg != 20 || s.Min != 0 || s.Max != 40 || s.P50 != 20 || s.P99 != 40 {
				t.Fatalf("sample biased full-window statistics: %+v", s)
			}
		}
		if budget > 36 && (v.Records[0].Time.Sub(start) > time.Second || v.Records[len(v.Records)-1].Time.Sub(start) < 23*time.Hour+58*time.Minute) {
			t.Fatal("history endpoints were truncated")
		}
	}
	check("public:getPingRecords", map[string]any{"uuid": ids[0], "hours": "24"}, admin, 4000)
	// Match the theme's two concurrent chart requests against full-day data.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			r := jsonrpc.Dispatch(context.Background(), admin, &rpc.JsonRpcRequest{ID: 1, Method: "common:getRecords", Params: map[string]any{"uuid": ids[0], "type": "ping", "hours": 24, "maxCount": 1000}})
			if r.Error != nil {
				t.Errorf("concurrent full-day query: %v", r.Error)
			}
		})
	}
	wg.Wait()
	for _, method := range []string{"public:getPingMetricStats", "public:queryMetrics"} {
		r := jsonrpc.Dispatch(context.Background(), admin, &rpc.JsonRpcRequest{ID: 1, Method: method, Params: map[string]any{"entity_id": ids[0], "hours": 24, "metric_keys": []string{"ping.latency_ms"}, "max_points": 120}})
		if r.Error != nil {
			t.Fatalf("full-day metric endpoint %s: %v", method, r.Error)
		}
		encoded, _ := json.Marshal(r.Result)
		var v struct {
			Stats []struct {
				Total, Valid int
				Loss         float64
			}
			Series []any
		}
		if err := json.Unmarshal(encoded, &v); err != nil {
			t.Fatal(err)
		}
		if method == "public:getPingMetricStats" {
			if len(v.Stats) != 36 || v.Stats[0].Total != 1440 || v.Stats[0].Valid != 1080 || v.Stats[0].Loss != 25 {
				t.Fatal("full-day metric statistics lost observations")
			}
		} else if len(v.Series) != 36 {
			t.Fatal("full-day metric curves lost tasks")
		}
	}
	// Filtering must happen before both statistics and sampling budgets.
	guest := &rpc.ContextMeta{Permission: rpc.RoleGuest, TempShareValid: true, Share: &share.Grant{Nodes: []string{ids[0]}, HistoryHours: 24}}
	v := check("common:getRecords", map[string]any{"type": "ping", "task_id": 970000, "hours": 24}, guest, 4000)
	if len(v.Tasks) != 1 || v.Tasks[0].Total != 1440 {
		t.Fatal("hidden/outside records polluted statistics", v.Tasks)
	}
	for _, r := range v.Records {
		if r.Client != ids[0] {
			t.Fatal("share isolation violated")
		}
	}
	v = check("public:getPingRecords", map[string]any{"task_id": "970000", "hours": "24"}, guest, 4000)
	if v.Tasks[0].Total != 1440 {
		t.Fatal("legacy statistics leaked other nodes")
	}
	guest.Share.HistoryHours = 1
	v = check("common:getRecords", map[string]any{"uuid": ids[0], "type": "ping", "hours": 1}, guest, 4000)
	if len(v.Tasks) != 36 || v.Tasks[0].Total != 60 || v.Tasks[0].Loss != 25 {
		t.Fatal("one-hour share did not clip full-day data")
	}
	if jsonrpc.Dispatch(context.Background(), guest, &rpc.JsonRpcRequest{ID: 1, Method: "common:getRecords", Params: map[string]any{"uuid": ids[0], "type": "ping", "hours": 24}}).Error == nil {
		t.Fatal("one-hour share acquired full-day history")
	}
	// A realtime share must still be denied, even with a full database behind it.
	guest.Share.HistoryHours = 0
	if jsonrpc.Dispatch(context.Background(), guest, &rpc.JsonRpcRequest{ID: 1, Method: "common:getRecords", Params: map[string]any{"uuid": ids[0], "type": "ping", "hours": 24}}).Error == nil {
		t.Fatal("realtime share gained history")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if jsonrpc.Dispatch(ctx, admin, &rpc.JsonRpcRequest{ID: 1, Method: "common:getRecords", Params: map[string]any{"uuid": ids[0], "type": "ping", "hours": 24}}).Error == nil {
		t.Fatal("cancelled query continued")
	}
}
