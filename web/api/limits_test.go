package api

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAgentBurstHasBoundedCapacitySeparateFromAdministration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
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
