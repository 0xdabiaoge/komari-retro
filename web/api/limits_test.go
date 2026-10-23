package api

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func configureViewerCapacityTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := config.Set(config.PrivateSiteKey, false); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBurstHasBoundedCapacitySeparateFromAdministration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// This unit fixture supplies an already authenticated identity. Real
	// credential selection is covered by the HTTP boundary integration tests.
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/api/clients/report" {
			c.Set("role", RoleClient)
		} else {
			c.Set("role", RoleAdmin)
		}
		c.Next()
	})
	r.Use(RequestLimits())
	r.POST("/api/clients/report", func(c *gin.Context) { time.Sleep(20 * time.Millisecond); c.Status(200) })
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Go(func() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/api/clients/report", nil))
			if w.Code != 200 {
				t.Errorf("legitimate burst rejected: %d", w.Code)
			}
		})
	}
	wg.Wait()
	// Saturating both Agent admission and its waiting queue remains bounded,
	// but leaves the independent administrator pool available.
	for i := 0; i < cap(agentRequestSlots); i++ {
		agentRequestSlots <- struct{}{}
	}
	for i := 0; i < cap(agentRequestWaiters); i++ {
		agentRequestWaiters <- struct{}{}
	}
	defer func() {
		for len(agentRequestSlots) > 0 {
			<-agentRequestSlots
		}
		for len(agentRequestWaiters) > 0 {
			<-agentRequestWaiters
		}
	}()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/clients/report", nil))
	if w.Code != 503 || w.Header().Get("Retry-After") != "1" {
		t.Fatal("overflow is unbounded or missing retry hint")
	}
	r.POST("/api/admin/check", func(c *gin.Context) { c.Status(200) })
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/admin/check", nil))
	if w.Code != 200 {
		t.Fatal("Agent pool exhaustion blocked administrator")
	}
}

func TestViewerAdmissionReservesAgentAndAdministratorCapacity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Public mode here avoids needing a share registry: quotas still apply.
	// Configuration failure is fail-closed, so provide a test config database.
	configureViewerCapacityTest(t)
	releases := []func(){}
	defer func() {
		for _, release := range releases {
			release()
		}
	}()
	for i := 0; i < 128; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/rpc2", nil)
		c.Request.RemoteAddr = fmt.Sprintf("198.51.%d.%d:1234", i/250, i%250+1)
		release, ok := AdmitViewerSocket(c)
		if !ok {
			t.Fatal("guest pool smaller than configured")
		}
		releases = append(releases, release)
	}
	c, w := gin.CreateTestContext(httptest.NewRecorder())
	_ = w
	c.Request = httptest.NewRequest("GET", "/api/rpc2", nil)
	c.Request.RemoteAddr = "203.0.113.1:1234"
	if release, ok := AdmitViewerSocket(c); ok {
		release()
		t.Fatal("global viewer quota bypass")
	}
	for i := 0; i < 500; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/rpc2", nil)
		c.Set("role", RoleClient)
		release, ok := AdmitViewerSocket(c)
		if !ok {
			t.Fatal("viewer load blocked authenticated Agent admission")
		}
		release()
	}
	c.Set("role", RoleAdmin)
	release, ok := AdmitViewerSocket(c)
	if !ok {
		t.Fatal("viewer load blocked administrator")
	}
	release()
	for i := 0; i < cap(requestSlots); i++ {
		requestSlots <- struct{}{}
	}
	defer func() {
		for len(requestSlots) > 0 {
			<-requestSlots
		}
	}()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/api/admin/check" {
			c.Set("role", RoleAdmin)
		} else if c.Request.URL.Path == "/api/clients/report" {
			c.Set("role", RoleClient)
		}
		c.Next()
	}, RequestLimits())
	for _, path := range []string{"/api/admin/check", "/api/clients/report", "/api/public"} {
		r.POST(path, func(c *gin.Context) { c.Status(200) })
	}
	for _, path := range []string{"/api/admin/check", "/api/clients/report", "/api/public"} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest("POST", path, nil))
		want := 200
		if path == "/api/public" {
			want = 503
		}
		if response.Code != want {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
