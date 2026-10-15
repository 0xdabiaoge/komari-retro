package jsonrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
)

func TestSecurityRegressions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	flags.DatabaseFile = filepath.Join(t.TempDir(), "audit.db")
	flags.DatabaseType = "sqlite"
	dbcore.GetDBInstance()
	t.Cleanup(func() { _ = dbcore.Close() })
	user, err := accounts.CreateAccount("audit-only", "isolated-test-password")
	if err != nil {
		t.Fatal(err)
	}
	token, err := accounts.CreateSession(user.UUID, 3600, "audit", "127.0.0.1", "password")
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.SetTrustedProxies(nil)
	engine.Use(api.RequestLimits())
	engine.Use(api.IdentityMiddleware())
	engine.GET("/api/rpc2", jsonrpc.OnRpcRequest)
	engine.POST("/api/rpc2", jsonrpc.OnRpcRequest)
	engine.GET("/ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })
	engine.GET("/api/admin/test", api.RequireRole(api.RoleAdmin), func(c *gin.Context) { c.Status(200) })
	srv := httptest.NewServer(engine)
	defer srv.Close()

	t.Run("PingPresetCatalogAndImportRequireAdministrator", func(t *testing.T) {
		for _, method := range []string{"admin:getPingPresets", "admin:importPingPresets"} {
			for _, meta := range []*rpc.ContextMeta{{Permission: rpc.RoleGuest, TempShareValid: true}, {Permission: rpc.RoleClient}, {Permission: rpc.RoleAdmin, APIKey: true, APIKeyScope: "read-only"}} {
				response := jsonrpc.Dispatch(context.Background(), meta, &rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: method, Params: map[string]any{}})
				if response.Error == nil || response.Result != nil {
					t.Fatalf("non-admin acquired ping presets via %s", method)
				}
			}
		}
		response := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{Version: "2.0", ID: 2, Method: "admin:getPingPresets", Params: map[string]any{}})
		if response.Error != nil {
			t.Fatal("administrator cannot read preset catalog")
		}
	})

	t.Run("RemovedExternalLoginSettingsCannotReturn", func(t *testing.T) {
		for _, key := range []string{"o_auth_enabled", "o_auth_provider", "o_auth_client_id", "o_auth_client_secret", "disable_password_login", "oauth_enabled", "oauth_enable", "oauth_provider"} {
			response := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: "admin:editSettings", Params: map[string]any{key: "synthetic"}})
			if response.Error == nil || response.Error.Code != rpc.InvalidParams {
				t.Fatalf("removed setting %s was accepted", key)
			}
		}
	})
	t.Run("PublicPingTaskMembershipAndSummariesHidePrivateNodes", func(t *testing.T) {
		db := dbcore.GetDBInstance()
		for _, node := range []models.Client{
			{UUID: "preset-visible", Token: "preset-visible", Name: "visible"},
			{UUID: "preset-hidden", Token: "preset-hidden", Name: "hidden", Hidden: true},
		} {
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
		}
		defer db.Where("uuid IN ?", []string{"preset-visible", "preset-hidden"}).Delete(&models.Client{})
		for _, task := range []models.PingTask{
			{Id: 201, Name: "mixed-task", Clients: models.StringArray{"preset-visible", "preset-hidden", "preset-deleted"}, Target: "secret-target", Type: "icmp"},
			{Id: 202, Name: "private-only-task", Clients: models.StringArray{"preset-hidden"}, Target: "secret-target", Type: "icmp"},
			{Id: 203, Name: "future-only-task", Clients: models.StringArray{}, DefaultOn: true, Target: "secret-target", Type: "icmp"},
		} {
			if err := db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
		}
		defer db.Where("id IN ?", []uint{201, 202, 203}).Delete(&models.PingTask{})
		for _, call := range []struct {
			method string
			params map[string]any
		}{
			{"public:getPublicPingTasks", map[string]any{}},
			{"public:getPingRecords", map[string]any{"task_id": "201", "hours": "1"}},
			{"common:getRecords", map[string]any{"type": "ping", "task_id": 201, "hours": 1}},
			{"common:getRecords", map[string]any{"type": "ping", "hours": 1}},
		} {
			for _, permission := range []string{rpc.RoleGuest, rpc.RoleAdmin} {
				response := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: permission}, &rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: call.method, Params: call.params})
				if response.Error != nil {
					t.Fatalf("%s: %v", call.method, response.Error)
				}
				encoded, _ := json.Marshal(response.Result)
				body := string(encoded)
				if !strings.Contains(body, "mixed-task") {
					t.Fatalf("visible task lost: %s", body)
				}
				if strings.Contains(body, "secret-target") {
					t.Fatalf("target exposed: %s", body)
				}
				if permission == rpc.RoleGuest {
					for _, secret := range []string{"preset-hidden", "preset-deleted", "private-only-task", "future-only-task"} {
						if strings.Contains(body, secret) {
							t.Fatalf("%s leaked %s", call.method, secret)
						}
					}
				} else if call.method == "public:getPublicPingTasks" && !strings.Contains(body, "preset-hidden") {
					t.Fatal("administrator lost complete membership")
				}
			}
		}
	})

	t.Run("ResultAndBatchMemoryBudgets", func(t *testing.T) {
		calls := 0
		_ = jsonrpc.Register("securityBudgetFixture", func(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
			calls++
			var params struct {
				Size int `json:"size"`
			}
			_ = req.BindParams(&params)
			return strings.Repeat("x", params.Size), nil
		})
		single := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{Version: "2.0", ID: 1, Method: "common:securityBudgetFixture", Params: map[string]any{"size": 5 << 20}})
		if single.Error == nil || single.Error.Code != rpc.OutOfRange {
			t.Fatal("oversized result accepted")
		}
		calls = 0
		batch := []map[string]any{}
		for i := 0; i < 3; i++ {
			batch = append(batch, map[string]any{"jsonrpc": "2.0", "id": i, "method": "common:securityBudgetFixture", "params": map[string]any{"size": 3 << 20}})
		}
		payload, _ := json.Marshal(batch)
		request := httptest.NewRequest("POST", "/api/rpc2", bytes.NewReader(payload))
		request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, request)
		var responses []rpc.JsonRpcResponse
		if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
			t.Fatal(err)
		}
		if len(responses) != 3 || responses[0].Error != nil || responses[1].Error == nil || responses[2].Error == nil || calls != 2 || w.Body.Len() > 4<<20 {
			t.Fatalf("batch budget violated: results=%d calls=%d bytes=%d", len(responses), calls, w.Body.Len())
		}
	})

	t.Run("ForwardedIPCannotBeForged", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/ip", nil)
		r.RemoteAddr = "198.51.100.9:1234"
		r.Header.Set("X-Forwarded-For", "203.0.113.123")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if w.Body.String() != "198.51.100.9" {
			t.Fatalf("unexpected result %s", w.Body.String())
		}
	})
	t.Run("OneItemBatchReturnsArray", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/api/rpc2", bytes.NewBufferString(`[{"jsonrpc":"2.0","id":1,"method":"rpc:ping"}]`))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		if !strings.HasPrefix(w.Body.String(), "[") {
			t.Fatal("not reproduced")
		}
	})
	t.Run("AdminSettingsAndReadOnlyKeys", func(t *testing.T) {
		meta := &rpc.ContextMeta{Permission: rpc.RoleAdmin, User: &user, UserUUID: user.UUID, SessionToken: token}
		req := &rpc.JsonRpcRequest{Method: "admin:editSettings", ID: 1, Params: map[string]any{"api_key_scope": "read-only"}}
		if response := jsonrpc.Dispatch(context.Background(), meta, req); response.Error != nil {
			t.Fatal(response.Error)
		}
		if scope, err := config.GetAs[string](config.ApiKeyScopeKey, ""); err != nil || scope != "read-only" {
			t.Fatalf("admin setting was not applied: scope=%q err=%v", scope, err)
		}
		config.Set(config.ApiKeyKey, "isolated-read-only-key")
		defer config.Set(config.ApiKeyKey, "")
		defer config.Set(config.ApiKeyScopeKey, "full")
		db := dbcore.GetDBInstance()
		if err := db.Create(&models.Client{UUID: "readonly-probe", Name: "readonly", Token: "must-not-leak-agent-secret"}).Error; err != nil {
			t.Fatal(err)
		}
		defer db.Where("uuid = ?", "readonly-probe").Delete(&models.Client{})
		for _, method := range []string{"common:getNodes", "common:getMe"} {
			body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method})
			r := httptest.NewRequest("POST", "/api/rpc2", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer isolated-read-only-key")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, r)
			if strings.Contains(w.Body.String(), "must-not-leak-agent-secret") || strings.Contains(w.Body.String(), `"error"`) {
				t.Fatal("read-only query leaked credentials or failed", w.Body.String())
			}
		}
		for _, path := range []string{"/api/admin/test", "/api/rpc2"} {
			body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"admin:getSettings"}`)
			method := "POST"
			if path == "/api/admin/test" {
				method = "GET"
			}
			r := httptest.NewRequest(method, path, body)
			r.Header.Set("Authorization", "Bearer isolated-read-only-key")
			r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, r)
			if path == "/api/admin/test" {
				if w.Code != 403 {
					t.Fatal("read-only REST allowed", w.Code)
				}
			} else if !strings.Contains(w.Body.String(), "error") {
				t.Fatal("read-only RPC allowed")
			}
		}
	})
	t.Run("PingStatsUseSelectedWindowAndHideNodes", func(t *testing.T) {
		db := dbcore.GetDBInstance()
		now := time.Now()
		for _, hidden := range []bool{false, true} {
			id := "visible"
			if hidden {
				id = "hidden"
			}
			if err := db.Create(&models.Client{UUID: id, Token: id, Name: id, Hidden: hidden}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Create(&models.PingTask{Id: 123, Name: "loopback", Target: "127.0.0.1", Type: "tcp"}).Error; err != nil {
			t.Fatal(err)
		}
		for _, rec := range []models.PingRecord{
			{Client: "visible", TaskId: 123, Time: models.LocalTime(now.Add(-2 * time.Hour)), Value: 20},
			{Client: "visible", TaskId: 123, Time: models.LocalTime(now.Add(-time.Minute)), Value: -1},
			{Client: "hidden", TaskId: 123, Time: models.LocalTime(now.Add(-time.Minute)), Value: 1},
		} {
			if err := db.Create(&rec).Error; err != nil {
				t.Fatal(err)
			}
		}
		for _, hours := range []int{1, 24} {
			response := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleGuest}, &rpc.JsonRpcRequest{Method: "public:getPingMetricStats", ID: 1, Params: map[string]any{"hours": hours}})
			if response.Error != nil {
				t.Fatal(response.Error)
			}
			encoded, _ := json.Marshal(response.Result)
			var result struct {
				Stats []struct {
					EntityID     string `json:"entity_id"`
					Total, Valid int
					Loss         float64
				}
			}
			if err := json.Unmarshal(encoded, &result); err != nil {
				t.Fatal(err)
			}
			wantTotal, wantValid, wantLoss := 1, 0, float64(100)
			if hours == 24 {
				wantTotal, wantValid, wantLoss = 2, 1, 50
			}
			if len(result.Stats) != 1 || result.Stats[0].EntityID != "visible" || result.Stats[0].Total != wantTotal || result.Stats[0].Valid != wantValid || result.Stats[0].Loss != wantLoss {
				t.Fatalf("incorrect statistics %s", encoded)
			}
		}
	})
	t.Run("MetricQueryContract", checkMetricQueryContract)
	t.Run("LegacyGPUHistoryFilter", func(t *testing.T) {
		db := dbcore.GetDBInstance()
		if err := db.Create(&models.Record{Client: "gpu-filter", Time: models.FromTime(time.Now().Add(-time.Second)), Gpu: 42}).Error; err != nil {
			t.Fatal(err)
		}
		defer db.Where("client = ?", "gpu-filter").Delete(&models.Record{})
		response := jsonrpc.Dispatch(context.Background(), &rpc.ContextMeta{Permission: rpc.RoleAdmin}, &rpc.JsonRpcRequest{Method: "public:getRecordsByUUID", ID: 1, Params: map[string]any{"uuid": "gpu-filter", "hours": "1", "load_type": "gpu"}})
		if response.Error != nil {
			t.Fatal(response.Error)
		}
		encoded, err := json.Marshal(response.Result)
		if err != nil || !strings.Contains(string(encoded), `"gpu":42`) {
			t.Fatalf("GPU history missing: %s, %v", encoded, err)
		}
	})
	t.Run("RevokedSessionLosesWebSocketAdmin", func(t *testing.T) {
		h := http.Header{}
		h.Set("Cookie", "session_token="+token)
		h.Set("Origin", srv.URL)
		ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/api/rpc2", h)
		if err != nil {
			t.Fatal(err)
		}
		defer ws.Close()
		ws.SetReadDeadline(time.Now().Add(10 * time.Second))
		request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "admin:getSettings"}
		if err = ws.WriteJSON(request); err != nil {
			t.Fatal(err)
		}
		var before map[string]json.RawMessage
		if err = ws.ReadJSON(&before); err != nil {
			t.Fatal(err)
		}
		if before["error"] != nil {
			t.Fatal("initial admin request failed")
		}
		if err = accounts.DeleteSession(token); err != nil {
			t.Fatal(err)
		}
		request["id"] = 2
		if err = ws.WriteJSON(request); err != nil {
			t.Fatal(err)
		}
		var after map[string]json.RawMessage
		if err = ws.ReadJSON(&after); err != nil {
			t.Fatal(err)
		}
		if after["error"] == nil {
			t.Fatal("revoked WebSocket session still authorized")
		}
		r := httptest.NewRequest("POST", "/api/rpc2", bytes.NewBufferString(`{"jsonrpc":"2.0","id":3,"method":"admin:getSettings"}`))
		r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		var fresh map[string]json.RawMessage
		json.Unmarshal(w.Body.Bytes(), &fresh)
		if fresh["error"] == nil {
			t.Fatal("fresh request unexpectedly authorized")
		}
	})
}
