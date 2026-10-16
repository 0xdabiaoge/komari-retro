package jsonrpc

func ResetDashboardCachesForTest() {
	for _, c := range []*dashboardCache{&dashboardMetricsCache, &dashboardPingCache} {
		c.mu.Lock()
		c.value = nil
		c.mu.Unlock()
	}
}
