package jsonrpc

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/gorm"
)

func init() {
	reg("getDashboardMetrics", adminDashboardMetrics, "Get bounded 24-hour dashboard aggregates")
	reg("getDashboardPing", adminDashboardPing, "Get top 100 latency rankings")
}

// Only administrator aggregates are cached. Authorization happens in Dispatch
// before every cache read; public/share requests never use this cache.
type dashboardCache struct {
	mu      sync.Mutex
	db      *gorm.DB
	value   any
	expires time.Time
	running chan struct{}
}

func (c *dashboardCache) get(ctx context.Context, ttl time.Duration, build func(context.Context) (any, error)) (any, *rpc.JsonRpcError) {
	for {
		c.mu.Lock()
		if c.db == dbcore.GetDBInstance() && c.value != nil && time.Now().Before(c.expires) {
			value := c.value
			c.mu.Unlock()
			return value, nil
		}
		if ready := c.running; ready != nil {
			c.mu.Unlock()
			select {
			case <-ready:
				continue
			case <-ctx.Done():
				return nil, rpc.MakeError(rpc.Unavailable, "Dashboard query canceled", nil)
			}
		}
		c.running = make(chan struct{})
		c.mu.Unlock()
		value, err := build(ctx)
		c.mu.Lock()
		if err == nil {
			c.value, c.db, c.expires = value, dbcore.GetDBInstance(), time.Now().Add(ttl)
		}
		close(c.running)
		c.running = nil
		c.mu.Unlock()
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Unable to load dashboard statistics; retry later", nil)
		}
		return value, nil
	}
}

var dashboardMetricsCache, dashboardPingCache dashboardCache

type dashboardRank struct {
	UUID     string  `json:"uuid"`
	Value    float64 `json:"value"`
	Peak     float64 `json:"peak"`
	PeakTime int64   `json:"peakTime"`
}
type dashboardTrafficPoint struct {
	Time     int64   `json:"time"`
	UpRate   float64 `json:"upRate"`
	DownRate float64 `json:"downRate"`
	UpCum    float64 `json:"upCum"`
	DownCum  float64 `json:"downCum"`
}
type dashboardTrafficNode struct {
	UUID     string  `json:"uuid"`
	Up       float64 `json:"up"`
	Down     float64 `json:"down"`
	Total    float64 `json:"total"`
	PeakRate float64 `json:"peakRate"`
	PeakTime int64   `json:"peakTime"`
}
type dashboardTraffic struct {
	Points     []dashboardTrafficPoint `json:"points"`
	NodeTotals []dashboardTrafficNode  `json:"nodeTotals"`
	TotalUp    float64                 `json:"totalUp"`
	TotalDown  float64                 `json:"totalDown"`
}
type dashboardMetrics struct {
	Traffic dashboardTraffic `json:"traffic"`
	CPU     []dashboardRank  `json:"cpu"`
	Memory  []dashboardRank  `json:"memory"`
	Start   time.Time        `json:"start"`
	End     time.Time        `json:"end"`
}

func adminDashboardMetrics(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return dashboardMetricsCache.get(ctx, 30*time.Second, func(ctx context.Context) (any, error) { return buildDashboardMetrics(ctx, time.Now()) })
}

// Read history once, rather than sorting it eight times and returning a curve
// per node/metric. The wire response is 120 global buckets plus top 100 lists.
func buildDashboardMetrics(ctx context.Context, end time.Time) (*dashboardMetrics, error) {
	start := end.Add(-24 * time.Hour)
	boundary := end.Add(-4*time.Hour - time.Minute)
	columns := "r.client,r.time,COALESCE(r.cpu,0),COALESCE(r.ram,0),COALESCE(r.ram_total,0),COALESCE(r.net_in,0),COALESCE(r.net_out,0),COALESCE(r.traffic_up,0),COALESCE(r.traffic_down,0)"
	query := "SELECT " + columns + " FROM records r JOIN clients c ON c.uuid=r.client WHERE r.time>=? AND r.time<? AND r.time>=? UNION ALL SELECT " + columns + " FROM records_long_term r JOIN clients c ON c.uuid=r.client WHERE r.time>=? AND r.time<? AND r.time<? ORDER BY r.client,r.time"
	rows, err := dbcore.GetReadDB().WithContext(ctx).Raw(query, models.FromTime(start), models.FromTime(end), models.FromTime(boundary), models.FromTime(start), models.FromTime(end), models.FromTime(boundary)).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type bucket struct {
		count              int
		up, down, cpu, mem float64
	}
	type node struct {
		count            int
		cpuPeak, memPeak float64
		cpuTime, memTime int64
		buckets          [120]bucket
		traffic          dashboardTrafficNode
	}
	nodes := map[string]*node{}
	points := make([]dashboardTrafficPoint, 120)
	for i := range points {
		points[i].Time = start.Add(time.Duration(i) * 12 * time.Minute).UnixMilli()
	}
	for rows.Next() {
		var id string
		var at models.LocalTime
		var cpu float64
		var ram, total, in, out, up, down int64
		if err := rows.Scan(&id, &at, &cpu, &ram, &total, &in, &out, &up, &down); err != nil {
			return nil, err
		}
		b := int(at.ToTime().Sub(start) / (12 * time.Minute))
		if b < 0 || b >= 120 {
			continue
		}
		n := nodes[id]
		if n == nil {
			n = &node{traffic: dashboardTrafficNode{UUID: id}}
			nodes[id] = n
		}
		mem := 0.0
		if total > 0 {
			mem = 100 * float64(ram) / float64(total)
		}
		n.count++

		if n.count == 1 || cpu > n.cpuPeak {
			n.cpuPeak = cpu
			n.cpuTime = at.ToTime().UnixMilli()
		}
		if n.count == 1 || mem > n.memPeak {
			n.memPeak = mem
			n.memTime = at.ToTime().UnixMilli()
		}
		n.buckets[b].count++
		n.buckets[b].cpu += cpu
		n.buckets[b].mem += mem
		n.buckets[b].up += float64(out)
		n.buckets[b].down += float64(in)
		// TrafficUp/Down are observed deltas, never cumulative interface counters.
		u, d := math.Max(0, float64(up)), math.Max(0, float64(down))
		points[b].UpCum += u
		points[b].DownCum += d
		n.traffic.Up += u
		n.traffic.Down += d
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := &dashboardMetrics{Start: start, End: end, CPU: []dashboardRank{}, Memory: []dashboardRank{}, Traffic: dashboardTraffic{Points: []dashboardTrafficPoint{}, NodeTotals: []dashboardTrafficNode{}}}
	for id, n := range nodes {
		var cpuAvg, memAvg float64
		buckets := 0
		for _, b := range n.buckets {
			if b.count > 0 {
				cpuAvg += b.cpu / float64(b.count)
				memAvg += b.mem / float64(b.count)
				buckets++
			}
		}
		result.CPU = append(result.CPU, dashboardRank{id, cpuAvg / float64(buckets), n.cpuPeak, n.cpuTime})
		result.Memory = append(result.Memory, dashboardRank{id, memAvg / float64(buckets), n.memPeak, n.memTime})
		for i, b := range n.buckets {
			if b.count == 0 {
				continue
			}
			u, d := b.up/float64(b.count), b.down/float64(b.count)
			points[i].UpRate += u
			points[i].DownRate += d
			if u+d > n.traffic.PeakRate {
				n.traffic.PeakRate = u + d
				n.traffic.PeakTime = points[i].Time
			}
		}
		n.traffic.Total = n.traffic.Up + n.traffic.Down
		result.Traffic.NodeTotals = append(result.Traffic.NodeTotals, n.traffic)
	}
	for i := range points {
		result.Traffic.TotalUp += points[i].UpCum
		result.Traffic.TotalDown += points[i].DownCum
		points[i].UpCum = result.Traffic.TotalUp
		points[i].DownCum = result.Traffic.TotalDown
	}
	if len(nodes) > 0 {
		result.Traffic.Points = points
	}
	for _, list := range [][]dashboardRank{result.CPU, result.Memory} {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Value == list[j].Value {
				return list[i].UUID < list[j].UUID
			}
			return list[i].Value > list[j].Value
		})
	}
	sort.Slice(result.Traffic.NodeTotals, func(i, j int) bool {
		a, b := result.Traffic.NodeTotals[i], result.Traffic.NodeTotals[j]
		if a.Total == b.Total {
			return a.UUID < b.UUID
		}
		return a.Total > b.Total
	})
	if len(result.CPU) > 100 {
		result.CPU = result.CPU[:100]
	}
	if len(result.Memory) > 100 {
		result.Memory = result.Memory[:100]
	}
	if len(result.Traffic.NodeTotals) > 100 {
		result.Traffic.NodeTotals = result.Traffic.NodeTotals[:100]
	}
	return result, nil
}

func adminDashboardPing(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return dashboardPingCache.get(ctx, 2*time.Minute, func(ctx context.Context) (any, error) {
		end := time.Now()
		stats, err := queryPingStats(ctx, "", end.Add(-24*time.Hour), end, true, false)
		if err != nil {
			return nil, err
		}
		stable, unstable, loss := []pingMetricStat{}, []pingMetricStat{}, append([]pingMetricStat{}, stats...)
		for _, s := range stats {
			if s.Valid > 0 {
				stable = append(stable, s)
				unstable = append(unstable, s)
			}
		}
		sort.SliceStable(stable, func(i, j int) bool { return *stable[i].Ratio < *stable[j].Ratio })
		sort.SliceStable(unstable, func(i, j int) bool { return *unstable[i].Ratio > *unstable[j].Ratio })
		sort.SliceStable(loss, func(i, j int) bool { return loss[i].Loss > loss[j].Loss })
		if len(stable) > 100 {
			stable = stable[:100]
		}
		if len(unstable) > 100 {
			unstable = unstable[:100]
		}
		if len(loss) > 100 {
			loss = loss[:100]
		}
		return map[string]any{"stable": stable, "unstable": unstable, "loss": loss, "count": len(stats), "start": end.Add(-24 * time.Hour), "end": end}, nil
	})
}
