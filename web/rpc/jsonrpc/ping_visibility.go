package jsonrpc

import (
	"context"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
)

// Project both task membership and task existence onto the caller's audience.
// A task assigned only to hidden/deleted nodes (or future nodes) is not public.
func visiblePingTasks(ctx context.Context, input []models.PingTask) ([]models.PingTask, error) {
	if isLoginFromCtx(ctx) {
		return input, nil
	}
	nodes, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, err
	}
	visible := map[string]bool{}
	for _, node := range nodes {
		if !node.Hidden && shareAllows(ctx, node.UUID) {
			visible[node.UUID] = true
		}
	}
	out := make([]models.PingTask, 0, len(input))
	for _, task := range input {
		members := models.StringArray{}
		for _, id := range task.Clients {
			if visible[id] {
				members = append(members, id)
			}
		}
		if len(members) > 0 {
			task.Clients = members
			out = append(out, task)
		}
	}
	return out, nil
}
