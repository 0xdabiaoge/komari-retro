package jsonrpc_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/pkg/rpc"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

// Explicit workload: 500 nodes × 36 tasks × 1,440 minute samples = 25,920,000
// ping rows, plus 240 recent and 80 rollup load rows per node. Run separately
// from unit tests: go test ./web/rpc/jsonrpc -run '^$' -bench DashboardScale -benchtime=1x -timeout=20m
func BenchmarkDashboardScale(b *testing.B) {
	b.Chdir(b.TempDir())
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = filepath.Join(b.TempDir(), "scale.db")

	db := dbcore.GetDBInstance()
	defer dbcore.Close()
	start := time.Now()
	for _, sql := range []string{
		`WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<500) INSERT INTO clients(uuid,token,name,hidden) SELECT printf('%036d',i),printf('token-%d',i),printf('node-%d',i),i%10=0 FROM n`,
		`WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<36) INSERT INTO ping_tasks(id,name,type,target,interval,clients,all_clients,weight) SELECT i,printf('task-%d',i),'icmp','192.0.2.1',60,'[]',0,i FROM n`,
		`WITH RECURSIVE n(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM n WHERE i<239) INSERT INTO records(client,time,cpu,ram,ram_total,net_in,net_out,traffic_up,traffic_down) SELECT c.uuid,strftime('%Y-%m-%d %H:%M:%S','now',printf('-%d minutes',n.i)),20+n.i%60,512,1024,200,100,60,120 FROM clients c CROSS JOIN n`,
		`WITH RECURSIVE n(i) AS (VALUES(17) UNION ALL SELECT i+1 FROM n WHERE i<95) INSERT INTO records_long_term(client,time,cpu,ram,ram_total,net_in,net_out,traffic_up,traffic_down) SELECT c.uuid,strftime('%Y-%m-%d %H:%M:%S','now',printf('-%d minutes',n.i*15)),30,512,1024,200,100,900,1800 FROM clients c CROSS JOIN n`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			b.Fatal(err)
		}
	}
	// Chunked generation avoids a giant WAL transaction in the scale fixture.
	for hour := 0; hour < 24; hour++ {
		q := `WITH RECURSIVE n(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM n WHERE i<59) INSERT INTO ping_records(client,task_id,time,value) SELECT c.uuid,t.id,strftime('%Y-%m-%d %H:%M:%S','now',printf('-%d minutes',?+n.i)),CASE WHEN (n.i+t.id)%11=0 THEN -1 ELSE 10+(n.i+t.id)%100 END FROM clients c CROSS JOIN ping_tasks t CROSS JOIN n`
		if err := db.Exec(q, hour*60).Error; err != nil {
			b.Fatal(err)
		}
	}
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	var count int64
	db.Raw("SELECT COUNT(*) FROM ping_records").Scan(&count)
	if count != 25920000 {
		b.Fatal(count)
	}
	info, _ := os.Stat(flags.DatabaseFile)
	b.Logf("fixture: nodes=500 tasks=36 ping_rows=%d bytes=%d generation=%s GOMAXPROCS=%d", count, info.Size(), time.Since(start), runtime.GOMAXPROCS(0))
	call := func(method string) *rpc.JsonRpcResponse {
		return jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{ID: 1, Method: method})
	}
	b.ResetTimer()
	startReports := func(label string) func() {
		var once sync.Once
		stop := make(chan struct{})
		writeDone := make(chan time.Duration, 1)
		go func() {
			maxLatency := time.Duration(0)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					writeDone <- maxLatency
					return
				case <-ticker.C:
					at := time.Now()
					err := db.Exec("INSERT INTO records(client,time,cpu,ram,ram_total,net_in,net_out,traffic_up,traffic_down) VALUES (?,datetime('now'),42,512,1024,200,100,0,0)", "000000000000000000000000000000000001").Error
					if err != nil {
						b.Errorf("concurrent Agent report: %v", err)
					}
					if elapsed := time.Since(at); elapsed > maxLatency {
						maxLatency = elapsed
					}
				}
			}
		}()
		return func() {
			once.Do(func() {
				close(stop)
				maxLatency := <-writeDone
				b.Logf("max Agent report write during %s=%s", label, maxLatency)
				if maxLatency > time.Second {
					b.Error("background read/retention blocked report write")
				}
			})
		}
	}
	stopCold := startReports("cold history scans")
	defer stopCold()

	for _, method := range []string{"admin:getDashboardMetrics", "admin:getDashboardPing"} {
		started := time.Now()
		r := call(method)
		if r.Error != nil {
			b.Fatal(method, r.Error)
		}
		raw, _ := json.Marshal(r.Result)
		if len(raw) > 512000 {
			b.Fatal("unbounded response", method, len(raw))
		}
		b.Logf("%s cold=%s response_bytes=%d", method, time.Since(started), len(raw))
	}
	// Repeat cached reads while normal report writes continue.
	var wg sync.WaitGroup
	started := time.Now()
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			for _, method := range []string{"admin:getDashboardMetrics", "admin:getDashboardPing"} {
				r := call(method)
				if r.Error != nil {
					b.Errorf("concurrent request: %v", r.Error)
				}
			}
		})
	}
	writeStart := time.Now()
	if err := db.Exec("INSERT INTO records(client,time,cpu) VALUES (?,datetime('now'),42)", "000000000000000000000000000000000001").Error; err != nil {
		b.Fatal(err)
	}
	writeTime := time.Since(writeStart)
	wg.Wait()
	b.Logf("20 concurrent cached readers=%s report_write=%s", time.Since(started), writeTime)
	if writeTime > time.Second {
		b.Fatal("report write blocked by history reads")
	}
	stopCold()
	// Two minutes of expired probe traffic at this fleet size. Retention must
	// keep up with >=18,000 incoming rows/minute without deleting current data.
	if err := db.Exec(`INSERT INTO ping_records(client,task_id,time,value) SELECT c.uuid,t.id,'2000-01-01 00:00:00',10 FROM clients c CROSS JOIN ping_tasks t CROSS JOIN (SELECT 1 UNION ALL SELECT 2)`).Error; err != nil {
		b.Fatal(err)
	}
	stopCleanup := startReports("bounded retention")
	defer stopCleanup()
	cutoff := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	cleanupStart := time.Now()
	if err := dbcore.DeleteHistoryBefore(db, "ping_records", cutoff); err != nil {
		b.Fatal(err)
	}
	var remaining int64
	db.Raw("SELECT COUNT(*) FROM ping_records WHERE time<'2001-01-01'").Scan(&remaining)
	b.Logf("first bounded retention pass removed=%d elapsed=%s", 36000-remaining, time.Since(cleanupStart))
	if 36000-remaining < 18000 {
		b.Fatal("retention cannot keep up with fixture ingest rate")
	}
	for pass := 0; remaining > 0 && pass < 20; pass++ {
		if err := dbcore.DeleteHistoryBefore(db, "ping_records", cutoff); err != nil {
			b.Fatal(err)
		}
		db.Raw("SELECT COUNT(*) FROM ping_records WHERE time<'2001-01-01'").Scan(&remaining)
	}
	db.Raw("SELECT COUNT(*) FROM ping_records").Scan(&count)
	if remaining != 0 || count != 25920000 {
		b.Fatal("retention left expired data or removed retained history", remaining, count)
	}
	stopCleanup()
	b.StopTimer()
}
