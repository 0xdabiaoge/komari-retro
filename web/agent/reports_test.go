package agent

import (
	v1 "github.com/komari-monitor/komari/protocol/v1"
	"testing"
)

func TestReportSnapshotIsIndependent(t *testing.T) {
	report := &v1.Report{UUID: "node", GPU: &v1.GPUDetailReport{DetailedInfo: []v1.GPUDeviceInfo{{Name: "original"}}}}
	SetLatestReport("clone-test", report)
	report.UUID = "changed"
	report.GPU.DetailedInfo[0].Name = "changed"
	snapshot := GetLatestReport()["clone-test"]
	snapshot.UUID = ""
	snapshot.GPU.DetailedInfo[0].Name = "changed again"
	current := GetLatestReport()["clone-test"]
	if current.UUID != "node" || current.GPU.DetailedInfo[0].Name != "original" {
		t.Fatal("shared cache was mutated")
	}
}
