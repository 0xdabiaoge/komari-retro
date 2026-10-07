package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
)

type metricProjection struct{ column, unit, source string }

// Expressions are code-owned, never interpolated from request parameters.
var metricProjections = map[string]metricProjection{
	"cpu.usage": {"cpu", "%", "load"}, "gpu.usage": {"gpu", "%", "load"},
	"memory.used": {"ram", "bytes", "load"}, "memory.total": {"ram_total", "bytes", "load"},
	"swap.used": {"swap", "bytes", "load"}, "swap.total": {"swap_total", "bytes", "load"},
	"load.average": {"load", "", "load"}, "temperature": {"temp", "degC", "load"},
	"disk.used": {"disk", "bytes", "load"}, "disk.total": {"disk_total", "bytes", "load"},
	"net.in.rate": {"net_in", "bytes/s", "load"}, "net.out.rate": {"net_out", "bytes/s", "load"},
	"net.in": {"net_in", "bytes/s", "load"}, "net.out": {"net_out", "bytes/s", "load"},
	"net.total.up": {"net_total_up", "bytes", "load"}, "net.total.down": {"net_total_down", "bytes", "load"},
	"traffic.up": {"traffic_up", "bytes", "load"}, "traffic.down": {"traffic_down", "bytes", "load"},
	"process.count": {"process", "count", "load"}, "connections.tcp": {"connections", "count", "load"},
	"connections.udp":  {"connections_udp", "count", "load"},
	"gpu.device.usage": {"utilization", "%", "gpu"}, "gpu.memory.used": {"mem_used", "bytes", "gpu"},
	"gpu.memory.total": {"mem_total", "bytes", "gpu"}, "gpu.temperature": {"temperature", "degC", "gpu"},
	"ping.latency_ms": {"value", "ms", "ping"}, "ping.latency": {"value", "ms", "ping"},
}

var metricAggregations = map[string]bool{"avg": true, "min": true, "max": true, "sum": true,
	"first": true, "last": true, "stddev": true, "p70": true, "p95": true, "p99": true}

func publicListMetricDefinitions(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	hours, _ := config.GetAs[int]("record_preserve_time", 720)
	days := math.Min(31, math.Max(0, float64(hours)/24))
	keys := make([]string, 0, len(metricProjections))
	for key := range metricProjections {
		if key != "net.in" && key != "net.out" && key != "ping.latency" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, map[string]any{"name": key, "unit": metricProjections[key].unit, "type": "gauge", "retention_days": days})
	}
	return result, nil
}

func publicQueryMetrics(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		MetricKeys          []string          `json:"metric_keys"`
		EntityID            string            `json:"entity_id"`
		Hours               *float64          `json:"hours"`
		Start               string            `json:"start"`
		End                 string            `json:"end"`
		MaxPoints           int               `json:"max_points"`
		Aggregation         string            `json:"aggregation"`
		AggregationByMetric map[string]string `json:"aggregation_by_metric"`
		FillEmpty           bool              `json:"fill_empty"`
	}
	invalid := func(message string) (any, *rpc.JsonRpcError) {
		return nil, rpc.MakeError(rpc.InvalidParams, message, nil)
	}
	if err := req.BindParams(&params); err != nil {
		return invalid("Invalid metric parameters")
	}
	if params.Hours != nil && (math.IsNaN(*params.Hours) || math.IsInf(*params.Hours, 0) || *params.Hours <= 0 || *params.Hours > 744) {
		return invalid("hours must be greater than 0 and at most 744")
	}
	if params.MaxPoints == 0 {
		params.MaxPoints = 120
	}
	if params.MaxPoints < 1 || params.MaxPoints > 10000 {
		return invalid("max_points must be 1..10000")
	}
	if params.Aggregation == "" {
		params.Aggregation = "avg"
	}
	if !metricAggregations[params.Aggregation] {
		return invalid("Unsupported metric aggregation")
	}
	for _, aggregation := range params.AggregationByMetric {
		if !metricAggregations[aggregation] {
			return invalid("Unsupported metric aggregation")
		}
	}
	if len(params.MetricKeys) == 0 {
		params.MetricKeys = []string{"net.in", "net.out", "cpu.usage", "memory.used"}
	}
	if len(params.MetricKeys) > 32 {
		return invalid("At most 32 metrics per query")
	}
	for _, key := range params.MetricKeys {
		if _, exists := metricProjections[key]; !exists {
			return invalid("Unknown metric: " + key)
		}
	}
	start, end, rangeErr := metricQueryRange(params.Hours, params.Start, params.End)
	if rangeErr != nil {
		return invalid(rangeErr.Error())
	}
	interval := int64(math.Ceil(end.Sub(start).Seconds() / float64(params.MaxPoints)))
	if interval < 1 {
		interval = 1
	}
	series := make([]MetricSeriesItem, 0)
	count := 0
	seen := map[string]bool{}
	for _, key := range params.MetricKeys {
		if seen[key] {
			continue
		}
		seen[key] = true
		aggregation := params.Aggregation
		if override := params.AggregationByMetric[key]; override != "" {
			aggregation = override
		}
		items, err := querySQLiteMetric(ctx, key, params.EntityID, start, end, interval, aggregation, params.FillEmpty, params.MaxPoints)
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Unable to query metric series", nil)
		}
		for _, item := range items {
			count += item.Count
		}
		if count > 100000 {
			return invalid("Metric response exceeds 100000 points; narrow the query")
		}
		series = append(series, items...)
	}
	return map[string]any{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "series": series, "count": count}, nil
}

func querySQLiteMetric(ctx context.Context, key, entity string, start, end time.Time, interval int64, aggregation string, fill bool, maxPoints int) ([]MetricSeriesItem, error) {
	projection := metricProjections[key]
	filter := ` r.time>=? AND r.time<? AND (?='' OR r.client=?) AND (? OR c.hidden=0)`
	args := []any{models.FromTime(start), models.FromTime(end), entity, entity, isLoginFromCtx(ctx)}
	var source string
	switch projection.source {
	case "gpu":
		source = `SELECT r.client,r.time,r.` + projection.column + ` AS value,json_object('device_index',CAST(r.device_index AS TEXT),'device_name',r.device_name) AS tags FROM gpu_records r JOIN clients c ON c.uuid=r.client WHERE` + filter
	case "ping":
		source = `SELECT r.client,r.time,r.value AS value,json_object('task_id',CAST(r.task_id AS TEXT)) AS tags FROM ping_records r JOIN clients c ON c.uuid=r.client WHERE` + filter + ` AND r.value>=0`
	default:
		// Keep the same raw/rollup boundary as the legacy history path. Compute
		// aggregates in SQLite instead of dropping rows before averaging peaks.
		boundary := time.Now().Add(-4*time.Hour - time.Minute)
		base := `SELECT r.client,r.time,r.` + projection.column + ` AS value,'' AS tags FROM %s r JOIN clients c ON c.uuid=r.client WHERE` + filter
		source = fmt.Sprintf(base, "records") + ` AND r.time>=? UNION ALL ` + fmt.Sprintf(base, "records_long_term") + ` AND r.time<?`
		args = append(args, models.FromTime(boundary))
		args = append(args, models.FromTime(start), models.FromTime(end), entity, entity, isLoginFromCtx(ctx), models.FromTime(boundary))
	}
	expression := map[string]string{"avg": "AVG(value)", "min": "MIN(value)", "max": "MAX(value)", "sum": "SUM(value)",
		"first": "MAX(CASE WHEN time_rank=1 THEN value END)", "last": "MAX(CASE WHEN reverse_rank=1 THEN value END)", "stddev": "AVG(value*value)-AVG(value)*AVG(value)"}[aggregation]
	if expression == "" {
		p := map[string]string{"p70": "0.70", "p95": "0.95", "p99": "0.99"}[aggregation]
		position := `((sample_count-1)*` + p + `+1)`
		lower := `CAST(` + position + ` AS INTEGER)`
		fraction := `(` + position + `-` + lower + `)`
		expression = `(1-` + fraction + `)*MAX(CASE WHEN value_rank=` + lower + ` THEN value END)+` + fraction + `*COALESCE(MAX(CASE WHEN value_rank=` + lower + `+1 THEN value END),MAX(CASE WHEN value_rank=` + lower + ` THEN value END))`
	}
	sql := `WITH source AS (` + source + `), bucketed AS (
	 SELECT *,CAST(((CAST(strftime('%s',substr(time,1,19)) AS INTEGER)-CAST(strftime('%s',substr(?,1,19)) AS INTEGER))*10000000
	 +CAST(substr(substr(time,21,7)||'0000000',1,7) AS INTEGER)-?)/(?*10000000) AS INTEGER) AS bucket FROM source
 ), ranked AS (
 SELECT *,ROW_NUMBER() OVER (PARTITION BY client,tags,bucket ORDER BY time) AS time_rank,
 ROW_NUMBER() OVER (PARTITION BY client,tags,bucket ORDER BY time DESC) AS reverse_rank,
 ROW_NUMBER() OVER (PARTITION BY client,tags,bucket ORDER BY value) AS value_rank,
 COUNT(*) OVER (PARTITION BY client,tags,bucket) AS sample_count FROM bucketed
 ) SELECT client,tags,bucket,` + expression + ` AS value FROM ranked GROUP BY client,tags,bucket ORDER BY client,tags,bucket LIMIT 100001`
	// LocalTime persists seven fractional digits. Preserve them when assigning
	// buckets, otherwise a row just before end can create an extra final bucket.
	args = append(args, models.FromTime(start), start.Nanosecond()/100, interval)
	type metricRow struct {
		Client string
		Tags   string
		Bucket int64
		Value  float64
	}
	var rows []metricRow
	if err := dbcore.GetDBInstance().WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 100000 {
		return nil, fmt.Errorf("too many metric buckets")
	}
	result := make([]MetricSeriesItem, 0)
	var current *MetricSeriesItem
	var lastTags string
	appendPoint := func(bucket int64, value *float64) {
		current.Points = append(current.Points, MetricPointItem{Time: start.Add(time.Duration(bucket*interval) * time.Second).Format(time.RFC3339), Value: value})
		current.Count++
	}
	finish := func() {
		if current == nil {
			return
		}
		if fill {
			for b := int64(current.Count); b < int64(maxPoints) && start.Add(time.Duration(b*interval)*time.Second).Before(end); b++ {
				appendPoint(b, nil)
			}
		}
	}
	for _, row := range rows {
		if current == nil || current.EntityID != row.Client || lastTags != row.Tags {
			if fill && (len(result)+1)*maxPoints > 100000 {
				return nil, fmt.Errorf("too many metric series; narrow the query")
			}
			finish()
			result = append(result, MetricSeriesItem{MetricKey: key, EntityID: row.Client, Points: []MetricPointItem{}, Unit: projection.unit, IntervalSeconds: interval, DownsampleAlgorithm: aggregation})
			current = &result[len(result)-1]
			lastTags = row.Tags
			if row.Tags != "" {
				if err := json.Unmarshal([]byte(row.Tags), &current.Tags); err != nil {
					return nil, err
				}
			}
		}
		if fill {
			for b := int64(current.Count); b < row.Bucket; b++ {
				appendPoint(b, nil)
			}
		}
		value := row.Value
		if aggregation == "stddev" {
			value = math.Sqrt(math.Max(0, value))
		}
		appendPoint(row.Bucket, &value)
	}
	finish()
	return result, nil
}

func metricQueryRange(hours *float64, startValue, endValue string) (time.Time, time.Time, error) {
	end := time.Now()
	if endValue != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endValue)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end time")
		}
	}
	window := 24.0
	if hours != nil {
		window = *hours
		if math.IsNaN(window) || math.IsInf(window, 0) || window <= 0 || window > 744 {
			return time.Time{}, time.Time{}, fmt.Errorf("hours must be greater than 0 and at most 744")
		}
	}
	start := end.Add(-time.Duration(window * float64(time.Hour)))
	if startValue != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startValue)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start time")
		}
	}
	if !start.Before(end) || end.Sub(start) > 31*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid history window (maximum 31 days)")
	}
	return start, end, nil
}
