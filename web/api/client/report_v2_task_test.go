package client

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/database/tasks"
	"github.com/komari-monitor/komari/pkg/config"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	agentRuntime "github.com/komari-monitor/komari/web/agent"
)

func checkV2PresenceAndTaskResult(t *testing.T) {
	db := dbcore.GetDBInstance()
	if err := config.Set(config.NotificationEnabledKey, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"assigned", "other"} {
		if err := db.Create(&models.Client{UUID: id, Token: id, Name: id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	agentRuntime.SetClientProtocolVersion("assigned", 2)
	refreshPostPresence("assigned")
	t.Cleanup(func() {
		postPresenceMu.Lock()
		postPresenceStates["assigned"].timer.Stop()
		delete(postPresenceStates, "assigned")
		postPresenceMu.Unlock()
		agentRuntime.DeleteConnectedClients("assigned")
	})
	if !agentRuntime.IsV2Client("assigned") {
		t.Fatal("first presence refresh downgraded v2 command protocol")
	}
	if err := tasks.CreateTask("task", []string{"assigned"}, "echo marker"); err != nil {
		t.Fatal(err)
	}
	finished := time.Now().UTC().Truncate(time.Millisecond)
	req := v2.Request{JSONRPC: v2.Version, ID: "result", Method: v2.MethodAgentTaskResult, Params: map[string]any{
		"task_id": "task", "result": "stdout\n[stderr] marker", "exit_code": 7, "finished_at": finished.Format(time.RFC3339Nano),
	}}
	if got := handleV2RPC("other", req, true); got.Error == nil {
		t.Fatal("unassigned agent completed another client's task")
	}
	if got := handleV2RPC("assigned", req, true); got.Error != nil {
		t.Fatal(got.Error)
	}
	result, err := tasks.GetSpecificTaskResult("task", "assigned")
	if err != nil || result.ExitCode == nil || *result.ExitCode != 7 || result.Result != "stdout\n[stderr] marker" || result.FinishedAt == nil || !result.FinishedAt.ToTime().Equal(finished) {
		t.Fatalf("task result did not roundtrip: %+v, %v", result, err)
	}
}
