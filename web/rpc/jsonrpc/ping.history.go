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
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/pkg/rpc"
)

type historyPingPoint struct {
	TaskID uint             `json:"task_id"`
	Time   models.LocalTime `json:"time"`
	Value  int              `json:"value"`
	Client string           `json:"client"`
}

type historyPingSummary struct {
	Total, Valid int64
	Sum          float64
	Min, Max     int
	Latest       int
	LatestTime   string
	Bins         map[int]int64
}

func (s *historyPingSummary) add(value int, count int64, latest string, histogram bool) {
	s.Total += count
	if latest > s.LatestTime {
		s.LatestTime, s.Latest = latest, value
	}
	if value < 0 {
		return
	}
	if s.Valid == 0 || value < s.Min {
		s.Min = value
	}
	if value > s.Max {
		s.Max = value
	}
	s.Valid += count
	s.Sum += float64(value) * float64(count)
	if !histogram {
		return
	}
	if s.Bins == nil {
		s.Bins = map[int]int64{}
	}
	s.Bins[value] += count
}

func (s *historyPingSummary) loss() float64 {
	if s.Total == 0 {
		return 0
	}
	return 100 * float64(s.Total-s.Valid) / float64(s.Total)
}

// Summaries use every observation; only the plotted points are sampled in SQL.
// A normal 36-task, one-minute, 24-hour window already exceeds 50,000 raw rows.
func boundedPingHistory(ctx context.Context, uuid string, taskID int, start, end time.Time, limit int) (any, *rpc.JsonRpcError) {
	if limit == 0 {
		limit = 4000
	}
	if limit < 1 || limit > 10000 {
		return nil, rpc.MakeError(rpc.InvalidParams, "History is limited to 10000 points", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	admin := isLoginFromCtx(ctx)
	filter := "p.time>=? AND p.time<?"
	args := []any{models.FromTime(start), models.FromTime(end)}
	if uuid != "" {
		filter += " AND p.client=?"
		args = append(args, uuid)
	}
	if taskID >= 0 {
		filter += " AND p.task_id=?"
		args = append(args, taskID)
	}
	if !admin {
		filter += " AND c.hidden=0"
		if meta := rpc.MetaFromContext(ctx); meta != nil && meta.Share != nil {
			clause, values := meta.Share.SQLPredicate("p.client")
			filter += clause
			args = append(args, values...)
		}
	}
	pingTasks, err := tasks.GetAllPingTasks()
	if err == nil {
		pingTasks, err = visiblePingTasks(ctx, pingTasks)
	}
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Unable to verify ping task visibility", nil)
	}
	from := " FROM ping_records p JOIN clients c ON c.uuid=p.client WHERE " + filter
	rows, err := dbcore.GetReadDB().WithContext(ctx).Raw("SELECT p.client,p.task_id,p.value,COUNT(*),MAX(p.time)"+from+" GROUP BY p.client,p.task_id,p.value ORDER BY p.client,p.task_id,p.value", args...).Rows()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to query ping history", nil)
	}
	byTask := map[uint]*historyPingSummary{}
	byClient := map[string]*historyPingSummary{}
	type quota struct {
		Client string `json:"client"`
		TaskID uint   `json:"task_id"`
		Budget int    `json:"budget"`
	}
	groups := map[string]*quota{}
	counts := map[string]int{}
	for rows.Next() {
		var client, latest string
		var task uint
		var value int
		var count int64
		if err = rows.Scan(&client, &task, &value, &count, &latest); err != nil {
			break
		}
		key := fmt.Sprintf("%s:%d", client, task)
		if groups[key] == nil {
			if len(groups) >= 10000 {
				err = fmt.Errorf("too many node/task series")
				break
			}
			groups[key] = &quota{Client: client, TaskID: task}
		}
		counts[key] += int(count)
		if byTask[task] == nil {
			byTask[task] = &historyPingSummary{Latest: -1}
		}
		if byClient[client] == nil {
			byClient[client] = &historyPingSummary{Latest: -1}
		}
		byTask[task].add(value, count, latest, true)
		byClient[client].add(value, count, latest, false)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to query ping history", nil)
	}
	metadata := make([]struct {
		name   string
		length int
	}, 0, len(groups))
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		metadata = append(metadata, struct {
			name   string
			length int
		}{k, counts[k]})
	}
	budgets := allocateTargets(metadata, limit)
	quotas := make([]quota, 0, len(groups))
	for _, k := range keys {
		if budgets[k] > 0 {
			groups[k].Budget = budgets[k]
			quotas = append(quotas, *groups[k])
		}
	}
	points := []historyPingPoint{}
	if len(quotas) > 0 {
		encoded, _ := json.Marshal(quotas)
		// Keep both window endpoints and distribute the remaining points evenly
		// within each node/task. LIMIT alone would silently drop older history.
		query := `WITH quotas AS MATERIALIZED (
 SELECT json_extract(value,'$.client') AS client,json_extract(value,'$.task_id') AS task_id,
 json_extract(value,'$.budget') AS budget FROM json_each(?)
 ), ranked AS (
 SELECT p.client,p.task_id,p.time,p.value,q.budget,
 ROW_NUMBER() OVER (PARTITION BY p.client,p.task_id ORDER BY p.time,p.rowid) AS rn,
 COUNT(*) OVER (PARTITION BY p.client,p.task_id) AS n
 FROM quotas q JOIN ping_records p ON p.client=q.client AND p.task_id=q.task_id
 JOIN clients c ON c.uuid=p.client WHERE ` + filter + `
 ) SELECT client,task_id,time,value FROM ranked
 WHERE budget>=n OR (budget=1 AND rn=n) OR
 (budget>1 AND (rn=1 OR
 ((rn-1)*(budget-1)/(n-1))>((rn-2)*(budget-1)/(n-1))))
 ORDER BY time,client,task_id LIMIT ?`
		if err := dbcore.GetReadDB().WithContext(ctx).Raw(query, append(append([]any{string(encoded)}, args...), limit)...).Scan(&points).Error; err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to sample ping history", nil)
		}
	}
	info := []map[string]any{}
	for _, task := range pingTasks {
		if (taskID >= 0 && task.Id != uint(taskID)) || (uuid != "" && !task.AppliesToClient(uuid)) {
			continue
		}
		s := byTask[task.Id]
		if s == nil {
			s = &historyPingSummary{Latest: -1}
		}
		bins := make([]pingBin, 0, len(s.Bins))
		for v, n := range s.Bins {
			bins = append(bins, pingBin{float64(v), n})
		}
		sort.Slice(bins, func(i, j int) bool { return bins[i].value < bins[j].value })
		avg, p50, p99 := 0., 0., 0.
		if s.Valid > 0 {
			avg = s.Sum / float64(s.Valid)
			p50, p99 = pingPercentile(bins, s.Valid, .5), pingPercentile(bins, s.Valid, .99)
		}
		i := map[string]any{"id": task.Id, "name": task.Name, "type": task.Type, "interval": task.Interval, "default_on": task.DefaultOn,
			"total": s.Total, "valid": s.Valid, "loss": s.loss(), "min": s.Min, "max": s.Max, "avg": int(avg), "latest": s.Latest,
			"p50": int(math.Round(p50)), "p99": int(math.Round(p99)), "p99_p50_ratio": pingVolatility(p50, p99)}
		if uuid == "" && taskID >= 0 {
			i["clients"] = task.Clients
		}
		info = append(info, i)
	}
	basic := []map[string]any{}
	for client, s := range byClient {
		basic = append(basic, map[string]any{"client": client, "loss": s.loss(), "min": s.Min, "max": s.Max})
	}
	sort.Slice(basic, func(i, j int) bool { return basic[i]["client"].(string) < basic[j]["client"].(string) })
	return map[string]any{"count": len(points), "records": points, "tasks": info, "basic_info": basic, "from": models.FromTime(start), "to": models.FromTime(end)}, nil
}
