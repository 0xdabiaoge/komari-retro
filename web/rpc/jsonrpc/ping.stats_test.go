package jsonrpc

import (
	"math"
	"testing"
)

func TestFrequencyPercentilesAndZeroBaseline(t *testing.T) {
	bins := []pingBin{{0, 1}, {1, 1}}
	p50, p95, p99 := pingPercentile(bins, 2, .5), pingPercentile(bins, 2, .95), pingPercentile(bins, 2, .99)
	if p50 != .5 || p95 != .95 || p99 != .99 || math.Abs(pingVolatility(p50, p99)-.98) > .000001 || pingVolatility(0, 10) != 10 {
		t.Fatal(p50, p95, p99)
	}
	bins = []pingBin{{10, 9}, {100, 1}}
	if pingPercentile(bins, 10, .5) != 10 || math.Abs(pingPercentile(bins, 10, .95)-59.5) > .000001 {
		t.Fatal("frequency weights ignored")
	}
}
