package geoip

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

type lifecycleProvider struct {
	name   string
	closed atomic.Bool
}

func (p *lifecycleProvider) Name() string { return p.name }
func (p *lifecycleProvider) GetGeoInfo(net.IP) (*GeoInfo, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("lookup used a closed provider")
	}
	return &GeoInfo{ISOCode: "US"}, nil
}
func (p *lifecycleProvider) UpdateDatabase() error {
	_, err := p.GetGeoInfo(nil)
	return err
}
func (p *lifecycleProvider) Close() error { p.closed.Store(true); return nil }

func TestConcurrentProviderReplacementKeepsInFlightOperationsSafe(t *testing.T) {
	old := CurrentProvider
	t.Cleanup(func() { replaceProvider(old) })
	replaceProvider(&lifecycleProvider{name: "initial-concurrency-fixture"})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := GetGeoInfo(net.ParseIP("192.0.2.1")); err != nil {
					t.Error(err)
				}
				if err := UpdateDatabase(); err != nil {
					t.Error(err)
				}
				_ = ProviderName()
			}
		}()
	}
	for i := 0; i < 20; i++ {
		replaceProvider(&lifecycleProvider{name: fmt.Sprintf("concurrency-fixture-%d", i)})
	}
	wg.Wait()
}
