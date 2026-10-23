package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/komari-monitor/komari/pkg/rpc"
	"math"
	"sort"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

type pingMetricStat struct {
	EntityID string   `json:"entity_id"`
	TaskID   string   `json:"task_id"`
	Name     string   `json:"name"`
	Total    int64    `json:"total"`
	Valid    int64    `json:"valid"`
	Loss     float64  `json:"loss"`
	Avg      *float64 `json:"avg"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Latest   *float64 `json:"latest"`
	P50      *float64 `json:"p50"`
	P95      *float64 `json:"p95"`
	P99      *float64 `json:"p99"`
	Stddev   *float64 `json:"stddev"`
	Ratio    *float64 `json:"p99_p50_ratio"`
}

type pingBin struct {
	value float64
	count int64
}

// Interpolated percentiles over exact value frequencies. Losses are excluded.
// Memory is bounded by distinct integer latencies in one node/task, not raw rows.
func pingPercentile(bins []pingBin, total int64, p float64) float64 {
	position := float64(total-1) * p
	lo, hi := int64(math.Floor(position)), int64(math.Ceil(position))
	var seen int64
	var low, high float64
	for _, b := range bins {
		if seen <= lo && lo < seen+b.count {
			low = b.value
		}
		if seen <= hi && hi < seen+b.count {
			high = b.value
			break
		}
		seen += b.count
	}
	return low + (high-low)*(position-float64(lo))
}

func queryPingStats(ctx context.Context, entity string, start, end time.Time, admin bool, includeLatest bool) ([]pingMetricStat, error) {
	if admin && entity == "" && !includeLatest {
		return queryDashboardPingStats(ctx, start, end)
	}
	filter := "time>=? AND time<?"
	args := []any{models.FromTime(start), models.FromTime(end)}
	if entity != "" {
		filter += " AND client=?"
		args = append(args, entity)
	}
	visibility := ""
	if !admin {
		visibility = " WHERE c.hidden=0"
	}
	if meta := rpc.MetaFromContext(ctx); !admin && meta != nil && meta.Share != nil {
		clause, values := meta.Share.SQLPredicate("client")
		filter += clause
		args = append(args, values...)
	}
	// Aggregate before joining metadata. The covering index streams each value
	// frequency in order; no global raw-row sort or repeated metadata lookup.
	latestExpr := "''"
	if includeLatest {
		latestExpr = "MAX(time)"
	}
	query := `SELECT h.client,h.task_id,t.name,h.value,h.samples,h.latest
 FROM (SELECT client,task_id,value,COUNT(*) AS samples,` + latestExpr + ` AS latest
 FROM ping_records INDEXED BY idx_ping_stats_cover WHERE ` + filter + ` GROUP BY client,task_id,value) h
 JOIN clients c ON c.uuid=h.client JOIN ping_tasks t ON t.id=h.task_id` + visibility + ` ORDER BY h.client,h.task_id,h.value`
	rows, err := dbcore.GetReadDB().WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []pingMetricStat{}
	var current *pingMetricStat
	bins := []pingBin{}
	var sum, squares float64
	var latestKey string
	var tied bool
	var ambiguous []int
	finish := func() {
		if current == nil {
			return
		}
		if includeLatest && tied {
			ambiguous = append(ambiguous, len(result)-1)
		}
		current.Loss = 100 * float64(current.Total-current.Valid) / float64(current.Total)
		if current.Valid > 0 {
			avg := sum / float64(current.Valid)
			stddev := math.Sqrt(math.Max(0, squares/float64(current.Valid)-avg*avg))
			p50, p95, p99 := pingPercentile(bins, current.Valid, .5), pingPercentile(bins, current.Valid, .95), pingPercentile(bins, current.Valid, .99)
			ratio := pingVolatility(p50, p99)
			current.Avg = &avg
			current.Stddev = &stddev
			current.P50 = &p50
			current.P95 = &p95
			current.P99 = &p99
			current.Ratio = &ratio
		}
	}
	for rows.Next() {
		var client, name, key string
		var task, count int64
		var value float64
		if err := rows.Scan(&client, &task, &name, &value, &count, &key); err != nil {
			return nil, err
		}
		id := fmt.Sprint(task)
		if current == nil || current.EntityID != client || current.TaskID != id {
			finish()
			result = append(result, pingMetricStat{EntityID: client, TaskID: id, Name: name})
			current = &result[len(result)-1]
			bins = bins[:0]
			sum = 0
			squares = 0
			latestKey = ""
			tied = false
		}
		current.Total += count
		if key == latestKey {
			tied = true
		}
		if key > latestKey {
			tied = false
			latestKey = key
			current.Latest = nil
			if value >= 0 {
				v := value
				current.Latest = &v
			}
		}
		if value < 0 {
			continue
		}
		current.Valid += count
		sum += value * float64(count)
		squares += value * value * float64(count)
		bins = append(bins, pingBin{value, count})
		if current.Min == nil {
			v := value
			current.Min = &v
		}
		v := value
		current.Max = &v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	finish()
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// Resolve only equal-timestamp values, using rowid as the receipt-order tie
	// break. Most groups need no extra lookup; all percentiles remain exact.
	for _, i := range ambiguous {
		s := &result[i]
		var value float64
		if err := dbcore.GetReadDB().WithContext(ctx).Raw("SELECT value FROM ping_records INDEXED BY idx_ping_record_client_time WHERE client=? AND task_id=? AND time>=? AND time<? ORDER BY time DESC,rowid DESC LIMIT 1", s.EntityID, s.TaskID, models.FromTime(start), models.FromTime(end)).Row().Scan(&value); err != nil {
			return nil, err
		}
		s.Latest = nil
		if value >= 0 {
			v := value
			s.Latest = &v
		}
	}
	return result, nil
}

// Transfer one compact frequency vector per node/task instead of repeating its
// metadata for every latency value. This avoids a million Go/SQLite row scans
// in a large dashboard while retaining exact counts and quantiles.
func queryDashboardPingStats(ctx context.Context, start, end time.Time) ([]pingMetricStat, error) {
	query := `WITH frequencies AS (
 SELECT client,task_id,value,COUNT(*) AS samples FROM ping_records INDEXED BY idx_ping_stats_cover
 WHERE time>=? AND time<? GROUP BY client,task_id,value
 ) SELECT f.client,f.task_id,t.name,json_group_array(json_array(f.value,f.samples))
 FROM frequencies f JOIN clients c ON c.uuid=f.client JOIN ping_tasks t ON t.id=f.task_id
 GROUP BY f.client,f.task_id ORDER BY f.client,f.task_id`
	rows, err := dbcore.GetReadDB().WithContext(ctx).Raw(query, models.FromTime(start), models.FromTime(end)).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []pingMetricStat{}
	for rows.Next() {
		var s pingMetricStat
		var task int64
		var encoded []byte
		if err := rows.Scan(&s.EntityID, &task, &s.Name, &encoded); err != nil {
			return nil, err
		}
		s.TaskID = fmt.Sprint(task)
		var frequencies [][2]float64
		if err := json.Unmarshal(encoded, &frequencies); err != nil {
			return nil, err
		}
		bins := make([]pingBin, 0, len(frequencies))
		var sum, squares float64
		for _, f := range frequencies {
			n := int64(f[1])
			s.Total += n
			if f[0] < 0 {
				continue
			}
			s.Valid += n
			sum += f[0] * f[1]
			squares += f[0] * f[0] * f[1]
			bins = append(bins, pingBin{f[0], n})
		}
		s.Loss = 100 * float64(s.Total-s.Valid) / float64(s.Total)
		if s.Valid > 0 {
			sort.Slice(bins, func(i, j int) bool { return bins[i].value < bins[j].value })
			min, max := bins[0].value, bins[len(bins)-1].value
			avg := sum / float64(s.Valid)
			stddev := math.Sqrt(math.Max(0, squares/float64(s.Valid)-avg*avg))
			p50, p95, p99 := pingPercentile(bins, s.Valid, .5), pingPercentile(bins, s.Valid, .95), pingPercentile(bins, s.Valid, .99)
			ratio := pingVolatility(p50, p99)
			s.Min = &min
			s.Max = &max
			s.Avg = &avg
			s.Stddev = &stddev
			s.P50 = &p50
			s.P95 = &p95
			s.P99 = &p99
			s.Ratio = &ratio
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func pingVolatility(p50, p99 float64) float64 {
	if p50 <= 0 {
		return p99
	}
	return (p99 - p50) / p50
}
