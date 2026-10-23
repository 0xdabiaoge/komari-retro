package jsonrpc_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/pkg/share"
	v1 "github.com/komari-monitor/komari/protocol/v1"
	agent "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	report "github.com/komari-monitor/komari/web/report"
	"github.com/komari-monitor/komari/web/router"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func checkViewerSecurityBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := dbcore.GetDBInstance()
	if err := config.SetMany(map[string]any{"private_site": true, "permanent_share_token": "legacy-test", "theme": "retro", "custom_head": "PRIVATE_HTML_CANARY", "custom_body": "PRIVATE_BODY_CANARY", "notification_enabled": false, "admin_path": "/entry-fixture", "admin_view_path": "/view-fixture"}); err != nil {
		t.Fatal(err)
	}
	user, err := accounts.CreateAccount("viewer-owner", "fixture-only-password")
	if err != nil {
		t.Fatal(err)
	}
	session, err := accounts.CreateSession(user.UUID, 3600, "fixture", "127.0.0.1", "password")
	if err != nil {
		t.Fatal(err)
	}
	owner := "session_token=" + session
	for _, n := range []models.Client{{UUID: "viewer-audit-allowed", Token: "AGENT_SECRET", Name: "allowed-node", KernelVersion: "KERNEL_SECRET", Remark: "PRIVATE_REMARK", Price: 9.99, Currency: "USD", BillingCycle: 30, ExpiredAt: models.FromTime(time.Now().Add(24 * time.Hour))}, {UUID: "viewer-audit-outside", Token: "OTHER_SECRET", Name: "outside-node"}, {UUID: "viewer-audit-hidden", Token: "HIDDEN_SECRET", Name: "hidden-node", Hidden: true}} {
		if err := db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.ThemeConfiguration{Short: "retro", Data: `{"guestDisplay":{"showPrice":false,"showExpiredAt":false},"_komari_onboarding_v1":{"secret":"THEME_SECRET"},"appearance":"dark","unknown":"UNKNOWN_SECRET"}`}).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"viewer-audit-allowed", "viewer-audit-outside", "viewer-audit-hidden"} {
		r := v1.Report{UUID: id, Message: "RAW_MESSAGE_SECRET", Method: "RAW_METHOD_SECRET", UpdatedAt: time.Now(), CPU: v1.CPUReport{Usage: 42}}
		agent.SetLatestReport(id, &r)
		defer agent.DeleteLatestReport(id)
		report.Records.SetDefault(id, []v1.Report{r})
		defer report.Records.Delete(id)
		if err := db.Create(&models.Record{Client: id, Time: models.FromTime(time.Now().Add(-time.Minute)), Cpu: 42}).Error; err != nil {
			t.Fatal(err)
		}
	}
	task := models.PingTask{Id: 501, Name: "visible-ping", Type: "icmp", Target: "PRIVATE_TARGET", Clients: models.StringArray{"viewer-audit-allowed", "viewer-audit-outside", "viewer-audit-hidden"}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"viewer-audit-allowed", "viewer-audit-outside", "viewer-audit-hidden"} {
		if err := db.Create(&models.PingRecord{Client: id, TaskId: 501, Time: models.FromTime(time.Now().Add(-time.Minute)), Value: 20}).Error; err != nil {
			t.Fatal(err)
		}
	}
	engine := gin.New()
	engine.SetTrustedProxies(nil)
	engine.Use(api.RequestLimits(), api.IdentityMiddleware(), api.PrivateSiteMiddleware())
	router.Register(engine)
	server := httptest.NewServer(api.ViewerHandler(engine))
	defer server.Close()
	call := func(prefix, method, cookie string, params any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		request, _ := http.NewRequest("POST", server.URL+prefix+"/rpc2", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Cookie", cookie)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result map[string]any
		if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	mustResult := func(prefix, method, cookie string, params any) any {
		t.Helper()
		r := call(prefix, method, cookie, params)
		if r["error"] != nil {
			t.Fatalf("%s: %v", method, r["error"])
		}
		return r["result"]
	}
	grant, token, err := share.Create(share.Grant{Name: "scoped", Nodes: []string{"viewer-audit-allowed"}, HistoryHours: 1, MaxConnections: 2, ShowPrice: true, ShowExpiry: true, ExpiresAt: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	cookie := "temp_key=" + token
	for _, prefix := range []string{"/api", "/viewer-api"} {
		t.Run(prefix, func(t *testing.T) {
			for _, method := range rpc.ListMethods() {
				if strings.HasPrefix(method, "admin:") && call(prefix, method, cookie, nil)["error"] == nil {
					t.Fatalf("guest called %s", method)
				}
			}
			for _, method := range []string{"public:getNodesInformation", "common:getNodes"} {
				data := mustResult(prefix, method, cookie, nil)
				encoded, _ := json.Marshal(data)
				for _, secret := range []string{"AGENT_SECRET", "KERNEL_SECRET", "PRIVATE_REMARK", "price", "currency", "billing_cycle", "expired_at", "created_at", "outside-node", "hidden-node"} {
					if bytes.Contains(encoded, []byte(secret)) {
						t.Fatalf("%s leaked %s", method, secret)
					}
				}
				if !bytes.Contains(encoded, []byte("allowed-node")) {
					t.Fatal("allowed node lost")
				}
			}
			for _, method := range []string{"public:getPublicSettings", "common:getPublicInfo"} {
				data := mustResult(prefix, method, cookie, nil)
				encoded, _ := json.Marshal(data)
				for _, secret := range []string{"PRIVATE_HTML", "PRIVATE_BODY", "THEME_SECRET", "UNKNOWN_SECRET", "_komari"} {
					if bytes.Contains(encoded, []byte(secret)) {
						t.Fatalf("%s leaked %s", method, secret)
					}
				}
				if !bytes.Contains(encoded, []byte("dark")) {
					t.Fatal("public theme setting lost")
				}
			}
			for _, method := range []string{"rpc.methods", "rpc.help", "rpc.version", "public:getVersion", "common:getVersion", "admin:getSettings", "admin:listViewerShares", "admin:revokeViewerShare"} {
				if r := call(prefix, method, cookie, map[string]any{"method": "admin:exec", "id": grant.ID}); r["error"] == nil {
					t.Fatalf("guest called %s", method)
				}
			}
			for _, method := range []string{"common:getNodes", "common:getNodesLatestStatus", "public:getClientRecentRecords", "common:getNodeRecentStatus", "public:getRecordsByUUID", "common:getRecords", "public:queryMetrics", "public:getPingMetricStats"} {
				if r := call(prefix, method, cookie, map[string]any{"uuid": "viewer-audit-outside", "entity_id": "viewer-audit-outside", "hours": 1}); r["error"] == nil {
					t.Fatalf("scope bypass: %s", method)
				}
			}
			for _, method := range []string{"public:getClientRecentRecords", "common:getNodesLatestStatus"} {
				data := mustResult(prefix, method, cookie, map[string]any{"uuid": "viewer-audit-allowed"})
				encoded, _ := json.Marshal(data)
				if bytes.Contains(encoded, []byte("RAW_MESSAGE")) || bytes.Contains(encoded, []byte("RAW_METHOD")) || bytes.Contains(encoded, []byte("viewer-audit-outside")) {
					t.Fatalf("raw report leaked: %s", encoded)
				}
			}
			for _, q := range []struct {
				method string
				params any
			}{
				{"common:getRecords", map[string]any{"hours": 1}},
				{"common:getRecords", map[string]any{"hours": 1, "type": "ping"}},
				{"public:getPingRecords", map[string]any{"hours": "1", "task_id": "501"}},
				{"public:getPublicPingTasks", nil},
				{"public:queryMetrics", map[string]any{"hours": 1, "metric_keys": []string{"cpu.usage"}}},
				{"public:getPingMetricStats", map[string]any{"hours": 1}},
			} {
				data := mustResult(prefix, q.method, cookie, q.params)
				encoded, _ := json.Marshal(data)
				if bytes.Contains(encoded, []byte("viewer-audit-outside")) || bytes.Contains(encoded, []byte("viewer-audit-hidden")) {
					t.Fatalf("aggregate/membership leaked through %s: %s", q.method, encoded)
				}
			}
			for _, params := range []map[string]any{
				{"hours": 2}, {"start": time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
				{"end": time.Now().Add(-time.Hour).Format(time.RFC3339)},
				{"START": time.Now().Add(-2 * time.Hour).Format(time.RFC3339)},
				{"hours": 1, "Hours": 744},
				{"start": "", "end": time.Now().Add(-time.Hour).Format(time.RFC3339)},
			} {
				if call(prefix, "public:queryMetrics", cookie, params)["error"] == nil {
					t.Fatal("history range bypass")
				}
			}
		})
	}
	t.Run("billing-opt-in-and-scope", func(t *testing.T) {
		if err := db.Model(&models.ThemeConfiguration{}).Where("short = ?", "retro").Update("data", `{"guestDisplay":{"showPrice":true,"showExpiredAt":true}}`).Error; err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(mustResult("/viewer-api", "common:getNodes", cookie, nil))
		if !bytes.Contains(encoded, []byte("9.99")) || !bytes.Contains(encoded, []byte("expired_at")) {
			t.Fatal("explicit opt-in lost")
		}
		_, restricted, err := share.Create(share.Grant{Name: "no-billing", Nodes: []string{"viewer-audit-allowed"}, HistoryHours: 0, MaxConnections: 2})
		if err != nil {
			t.Fatal(err)
		}
		restrictedCookie := "temp_key=" + restricted
		encoded, _ = json.Marshal(mustResult("/viewer-api", "common:getNodes", restrictedCookie, nil))
		if bytes.Contains(encoded, []byte("price")) || bytes.Contains(encoded, []byte("expired_at")) {
			t.Fatal("per-share field policy bypass")
		}
		if call("/viewer-api", "public:getClientRecentRecords", restrictedCookie, map[string]any{"uuid": "viewer-audit-allowed"})["error"] == nil {
			t.Fatal("history disabled but readable")
		}
		_, longToken, err := share.Create(share.Grant{Name: "rolling", Nodes: []string{"viewer-audit-allowed"}, HistoryHours: 24, MaxConnections: 2})
		if err != nil {
			t.Fatal(err)
		}
		longCookie := "temp_key=" + longToken
		end := time.Now().Add(-time.Hour).Format(time.RFC3339)
		if call("/viewer-api", "public:queryMetrics", longCookie, map[string]any{"end": end})["error"] == nil {
			t.Fatal("end-only default extended beyond rolling window")
		}
		mustResult("/viewer-api", "public:queryMetrics", longCookie, map[string]any{"end": end, "hours": 1})
	})
	t.Run("hash-only-and-admin-permission", func(t *testing.T) {
		raw, _ := config.Get(share.RegistryKey)
		encoded, _ := json.Marshal(raw)
		if bytes.Contains(encoded, []byte(token)) {
			t.Fatal("raw token persisted")
		}
		encoded, _ = json.Marshal(mustResult("/api", "admin:listViewerShares", owner, nil))
		if bytes.Contains(encoded, []byte("token_hash")) {
			t.Fatal("registry hash returned")
		}
		if call("/viewer-api", "admin:listViewerShares", owner+"; "+cookie, nil)["error"] == nil {
			t.Fatal("ambient administrator cookie elevated viewer")
		}
	})
	t.Run("anonymous-websockets-and-capacity", func(t *testing.T) {
		dial := func(path, cred string) (*websocket.Conn, *http.Response, error) {
			h := http.Header{}
			h.Set("Origin", server.URL)
			if cred != "" {
				h.Set("Cookie", cred)
			}
			return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, h)
		}
		for _, path := range []string{"/api/rpc2", "/viewer-api/rpc2", "/api/clients", "/viewer-api/clients"} {
			ws, r, e := dial(path, "")
			if e == nil {
				ws.Close()
				t.Fatal("anonymous upgrade accepted")
			}
			if r == nil || r.StatusCode != 404 {
				t.Fatal("unexpected rejection")
			}
		}
		for i := 0; i < 256; i++ {
			ws, r, e := dial("/viewer-api/rpc2", "")
			if e == nil {
				ws.Close()
				t.Fatal("anonymous capacity admission bypass")
			}
			if r == nil || r.StatusCode != 404 {
				t.Fatal("anonymous burst rejection missing")
			}
		}
		legacy, _, e := dial("/viewer-api/clients", cookie)
		if e != nil {
			t.Fatal(e)
		}
		legacy.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := legacy.WriteMessage(websocket.TextMessage, []byte("get")); err != nil {
			t.Fatal(err)
		}
		_, payload, err := legacy.ReadMessage()
		legacy.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"RAW_MESSAGE_SECRET", "RAW_METHOD_SECRET", "viewer-audit-outside", "viewer-audit-hidden"} {
			if bytes.Contains(payload, []byte(private)) {
				t.Fatalf("legacy socket leaked %s", private)
			}
		}
		var sockets []*websocket.Conn
		defer func() {
			for _, ws := range sockets {
				ws.Close()
			}
		}()
		for i := 0; i < 2; i++ {
			ws, _, e := dial("/viewer-api/rpc2", cookie)
			if e != nil {
				t.Fatal(e)
			}
			sockets = append(sockets, ws)
		}
		if ws, r, e := dial("/viewer-api/rpc2", cookie); e == nil {
			ws.Close()
			t.Fatal("quota bypass")
		} else if r == nil || r.StatusCode != 429 {
			t.Fatal("quota rejection missing")
		}
		if call("/api", "admin:getSettings", owner, nil)["error"] != nil {
			t.Fatal("viewer quota blocked administrator")
		}
		h, e := http.Get(server.URL + "/ping")
		if e != nil || h.StatusCode != 200 {
			t.Fatal("health blocked")
		}
		h.Body.Close()
		// Revocation applies to already-established sockets, not just new handshakes.
		if err := share.Revoke(grant.ID); err != nil {
			t.Fatal(err)
		}
		ws := sockets[0]
		ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := ws.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "common:getNodes"}); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := ws.ReadJSON(&result); err != nil {
			t.Fatal(err)
		}
		if result["error"] == nil {
			t.Fatal("revoked websocket still reads nodes")
		}
		if call("/viewer-api", "common:getNodes", cookie, nil)["error"] == nil {
			t.Fatal("revoked grant still valid")
		}
	})
	t.Run("public-build-fingerprint", func(t *testing.T) {
		for _, path := range []string{"/api/version", "/favicon.ico", "/viewer-bootstrap.js"} {
			r, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			if r.StatusCode != 404 {
				t.Fatal(fmt.Sprint(path, r.StatusCode))
			}
		}
	})
}
