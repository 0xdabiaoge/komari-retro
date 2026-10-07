package jsonrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/api/admin"
	jsonrpc "github.com/komari-monitor/komari/web/rpc/jsonrpc"
	"github.com/pquerna/otp/totp"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	engine.POST("/enable", api.RequireRole(api.RoleAdmin), admin.Enable2FA)
	engine.GET("/ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })
	engine.GET("/api/admin/test", api.RequireRole(api.RoleAdmin), func(c *gin.Context) { c.Status(200) })
	srv := httptest.NewServer(engine)
	defer srv.Close()
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
	t.Run("Existing2FACannotBeReplacedWithoutOldCode", func(t *testing.T) {
		if err := accounts.Enable2Fa(user.UUID, "JBSWY3DPEHPK3PXP"); err != nil {
			t.Fatal(err)
		}
		key, err := totp.Generate(totp.GenerateOpts{Issuer: "audit", AccountName: "audit"})
		if err != nil {
			t.Fatal(err)
		}
		code, err := totp.GenerateCode(key.Secret(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/enable?code="+code, nil)
		r.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		r.AddCookie(&http.Cookie{Name: "2fa_secret", Value: key.Secret()})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		changed, err := accounts.GetUserByUUID(user.UUID)
		if err != nil {
			t.Fatal(err)
		}
		if changed.TwoFactor == key.Secret() || w.Code == 200 {
			t.Fatalf("replacement not reproduced: status=%d", w.Code)
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
	t.Run("SensitiveSettingsAndReadOnlyKeys", func(t *testing.T) {
		current, err := accounts.GetUserByUUID(user.UUID)
		if err != nil {
			t.Fatal(err)
		}
		meta := &rpc.ContextMeta{Permission: rpc.RoleAdmin, User: &current, UserUUID: user.UUID, SessionToken: token}
		req := &rpc.JsonRpcRequest{Method: "admin:editSettings", ID: 1, Params: map[string]any{"api_key_scope": "read-only"}}
		if jsonrpc.Dispatch(context.Background(), meta, req).Error == nil {
			t.Fatal("sensitive settings changed without step-up")
		}
		code, err := totp.GenerateCode(current.TwoFactor, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		req.Params = map[string]any{"api_key_scope": "read-only", "2fa_code": code}
		if response := jsonrpc.Dispatch(context.Background(), meta, req); response.Error != nil {
			t.Fatal(response.Error)
		}
		all, err := config.GetAll()
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := all["2fa_code"]; exists {
			t.Fatal("OTP persisted in configuration")
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
