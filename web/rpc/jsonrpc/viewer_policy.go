package jsonrpc

import (
	"context"
	"fmt"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/pkg/viewer"
	"math"
	"strconv"
	"strings"
	"time"
)

func shareAllows(ctx context.Context, uuid string) bool {
	meta := rpc.MetaFromContext(ctx)
	return meta == nil || meta.Permission == rpc.RoleAdmin || meta.Share.Allows(uuid)
}

// Explicit audience surface: future public/common registrations are closed
// until their field and share-scope behavior is reviewed.
func guestMethodAllowed(method string) bool {
	switch method {
	case "rpc.ping", "rpc:ping", "public:getMe", "common:getMe",
		"public:getPublicSettings", "common:getPublicInfo",
		"public:getNodesInformation", "common:getNodes", "common:getNodesLatestStatus",
		"public:getClientRecentRecords", "common:getNodeRecentStatus",
		"public:getRecordsByUUID", "common:getRecords", "public:getPingRecords",
		"public:getPublicPingTasks", "public:queryMetrics", "public:getPingMetricStats", "public:listMetricDefinitions":
		return true
	}
	return false
}
func guestNodePolicy(ctx context.Context) viewer.Policy {
	if meta := rpc.MetaFromContext(ctx); meta != nil && meta.AdminEntranceValid {
		return viewer.Policy{Price: true, Expiry: true}
	}
	policy := viewer.CurrentPolicy()
	if meta := rpc.MetaFromContext(ctx); meta != nil && meta.Share != nil {
		policy.Price = policy.Price && meta.Share.ShowPrice
		policy.Expiry = policy.Expiry && meta.Share.ShowExpiry
	}
	return policy
}
func guestPublicSettings(ctx context.Context, input map[string]any) map[string]any {
	// The secret owner entrance intentionally renders the owner's sign-in UI.
	// ViewerHandler removes this cookie before either compatibility transport.
	if meta := rpc.MetaFromContext(ctx); meta != nil && meta.AdminEntranceValid {
		return input
	}
	out := viewer.PublicSettings(input)
	theme := out["theme_settings"].(map[string]any)
	policy := guestNodePolicy(ctx)
	theme["guestDisplay"] = map[string]any{"showPrice": policy.Price, "showExpiredAt": policy.Expiry}
	theme["guestDisplay.showPrice"] = policy.Price
	theme["guestDisplay.showExpiredAt"] = policy.Expiry
	if meta := rpc.MetaFromContext(ctx); meta != nil && meta.Share != nil {
		out["record_preserve_time"] = meta.Share.HistoryHours
		out["ping_record_preserve_time"] = meta.Share.HistoryHours
		if meta.Share.HistoryHours == 0 {
			out["record_enabled"] = false
		}
	}
	return out
}
func checkShareRequest(ctx context.Context, req *rpc.JsonRpcRequest) *rpc.JsonRpcError {
	meta := rpc.MetaFromContext(ctx)
	if meta == nil || meta.Share == nil {
		return nil
	}
	var params map[string]any
	if req.Params != nil && req.BindParams(&params) != nil {
		return rpc.MakeError(rpc.InvalidParams, "Invalid parameters", nil)
	}
	if params == nil {
		params = map[string]any{}
	}
	normalized := make(map[string]any, len(params))
	for key, value := range params {
		canonical := strings.ToLower(key)
		if _, duplicate := normalized[canonical]; duplicate {
			return rpc.MakeError(rpc.InvalidParams, "Ambiguous parameter names", nil)
		}
		normalized[canonical] = value
	}
	params = normalized
	req.Params = params
	for _, key := range []string{"uuid", "entity_id", "start", "end"} {
		if value, exists := params[key]; exists {
			if _, ok := value.(string); !ok {
				return rpc.MakeError(rpc.InvalidParams, "Invalid parameter type", nil)
			}
		}
	}
	for _, key := range []string{"uuid", "entity_id"} {
		if id, ok := params[key].(string); ok && id != "" && !meta.Share.Allows(id) {
			return rpc.MakeError(rpc.InvalidParams, "Node not found", nil)
		}
	}
	if ids, ok := params["uuids"].([]any); ok {
		for _, id := range ids {
			s, ok := id.(string)
			if !ok || !meta.Share.Allows(s) {
				return rpc.MakeError(rpc.InvalidParams, "Node not found", nil)
			}
		}
	}
	history := false
	switch req.Method {
	case "public:getRecordsByUUID", "public:getClientRecentRecords", "public:getPingRecords", "public:queryMetrics", "public:getPingMetricStats", "common:getRecords", "common:getNodeRecentStatus":
		history = true
	}
	if !history {
		return nil
	}
	hours := meta.Share.HistoryHours
	if hours <= 0 {
		return rpc.MakeError(rpc.PermissionDenied, "History is not shared", nil)
	}
	// Normalize defaults before checking end-only windows. Otherwise a metric
	// handler's 24-hour default could extend beyond the rolling share boundary.
	if _, ok := params["hours"]; !ok {
		defaultHours := 1
		switch req.Method {
		case "public:queryMetrics", "public:getPingMetricStats":
			defaultHours = 24
		case "public:getRecordsByUUID", "public:getPingRecords":
			defaultHours = 4
		}
		defaultHours = min(hours, defaultHours)
		if req.Method == "public:getRecordsByUUID" || req.Method == "public:getPingRecords" {
			params["hours"] = strconv.Itoa(defaultHours)
		} else {
			params["hours"] = defaultHours
		}
	}
	now := time.Now()
	oldest := now.Add(-time.Duration(hours)*time.Hour - 2*time.Second)
	if start, ok := params["start"].(string); ok && start != "" {
		parsed, err := time.Parse(time.RFC3339, start)
		if err != nil || parsed.Before(oldest) {
			return rpc.MakeError(rpc.InvalidParams, "History outside share scope", nil)
		}
	}
	if end, ok := params["end"].(string); ok && end != "" {
		parsed, err := time.Parse(time.RFC3339, end)
		if err != nil || parsed.Before(oldest) || parsed.After(now.Add(time.Minute)) {
			return rpc.MakeError(rpc.InvalidParams, "History outside share scope", nil)
		}
	}
	if raw, ok := params["hours"]; ok {
		n, err := strconv.ParseFloat(fmt.Sprint(raw), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 || n > float64(hours) {
			return rpc.MakeError(rpc.InvalidParams, "History outside share scope", nil)
		}
	}
	if start, _ := params["start"].(string); start == "" {
		if end, ok := params["end"].(string); ok && end != "" {
			parsed, err := time.Parse(time.RFC3339, end)
			duration := float64(1)
			if raw, ok := params["hours"]; ok {
				duration, _ = strconv.ParseFloat(fmt.Sprint(raw), 64)
			}
			if duration <= 0 {
				duration = 1
			}
			if err != nil || parsed.Add(-time.Duration(duration*float64(time.Hour))).Before(oldest) {
				return rpc.MakeError(rpc.InvalidParams, "History outside share scope", nil)
			}
		}
	}
	req.Params = params
	return nil
}
