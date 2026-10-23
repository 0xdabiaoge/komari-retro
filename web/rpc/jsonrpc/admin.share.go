package jsonrpc

import (
	"context"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/pkg/share"
)

func init() {
	RegisterWithGroupAndMeta("listViewerShares", rpc.RoleAdmin, listViewerShares, &rpc.MethodMeta{Summary: "List read-only shares"})
	RegisterWithGroupAndMeta("createViewerShare", rpc.RoleAdmin, createViewerShare, &rpc.MethodMeta{Summary: "Create a scoped read-only share"})
	RegisterWithGroupAndMeta("revokeViewerShare", rpc.RoleAdmin, revokeViewerShare, &rpc.MethodMeta{Summary: "Revoke a read-only share"})
}
func listViewerShares(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	grants, err := share.List()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Cannot load shares", nil)
	}
	for i := range grants {
		grants[i].Hash = ""
	}
	return grants, nil
}
func createViewerShare(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var grant share.Grant
	if req.BindParams(&grant) != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid share", nil)
	}
	nodes, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Cannot load nodes", nil)
	}
	visible := map[string]bool{}
	for _, n := range nodes {
		visible[n.UUID] = !n.Hidden
	}
	for _, id := range grant.Nodes {
		if !visible[id] {
			return nil, rpc.MakeError(rpc.InvalidParams, "Choose a visible node", nil)
		}
	}
	grant, token, err := share.Create(grant)
	if err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	return map[string]any{"grant": grant, "path": "/s/" + token}, nil
}
func revokeViewerShare(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		ID string `json:"id"`
	}
	if req.BindParams(&p) != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid share", nil)
	}
	if err := share.Revoke(p.ID); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Share not found", nil)
	}
	return nil, nil
}
