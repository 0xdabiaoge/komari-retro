package jsonrpc

import (
	"context"
	"encoding/json"

	"github.com/komari-monitor/komari/pkg/rpc"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func init() {
	RegisterWithGroupAndMeta("fileList", rpc.RoleAdmin, adminFileList, &rpc.MethodMeta{
		Name:    "admin:fileList",
		Summary: "List files in directory",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: false, Description: "Directory path"},
		},
		Returns: "RemoteFileInfo[]",
	})
	RegisterWithGroupAndMeta("fileListRoots", rpc.RoleAdmin, adminFileListRoots, &rpc.MethodMeta{
		Name:    "admin:fileListRoots",
		Summary: "List filesystem roots",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
		},
		Returns: "RemoteFileInfo[]",
	})
	RegisterWithGroupAndMeta("fileStat", rpc.RoleAdmin, adminFileStat, &rpc.MethodMeta{
		Name:    "admin:fileStat",
		Summary: "Get file stat",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "File path"},
		},
		Returns: "RemoteFileInfo",
	})
	RegisterWithGroupAndMeta("fileMkdir", rpc.RoleAdmin, adminFileMkdir, &rpc.MethodMeta{
		Name:    "admin:fileMkdir",
		Summary: "Create a directory",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "Directory path"},
			{Name: "mode", Type: "string", Required: false, Description: "Octal mode"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("fileSearch", rpc.RoleAdmin, adminFileSearch, &rpc.MethodMeta{
		Name:    "admin:fileSearch",
		Summary: "Search files or content",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "Directory path"},
			{Name: "query", Type: "string", Required: true, Description: "Search query"},
			{Name: "content", Type: "boolean", Required: false, Description: "Search inside file content"},
		},
		Returns: "RemoteSearchResult",
	})
	RegisterWithGroupAndMeta("fileDelete", rpc.RoleAdmin, adminFileDelete, &rpc.MethodMeta{
		Name:    "admin:fileDelete",
		Summary: "Delete a file or directory",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "Path to remove"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("fileMove", rpc.RoleAdmin, adminFileMove, &rpc.MethodMeta{
		Name:    "admin:fileMove",
		Summary: "Move or rename file",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "source", Type: "string", Required: true, Description: "Source path"},
			{Name: "destination", Type: "string", Required: true, Description: "Destination path"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("fileCopy", rpc.RoleAdmin, adminFileCopy, &rpc.MethodMeta{
		Name:    "admin:fileCopy",
		Summary: "Copy file or directory",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "source", Type: "string", Required: true, Description: "Source path"},
			{Name: "destination", Type: "string", Required: true, Description: "Destination path"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("fileChmod", rpc.RoleAdmin, adminFileChmod, &rpc.MethodMeta{
		Name:    "admin:fileChmod",
		Summary: "Change file permissions mode",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "File path"},
			{Name: "mode", Type: "string", Required: true, Description: "Octal mode"},
		},
		Returns: "null",
	})
	RegisterWithGroupAndMeta("fileChown", rpc.RoleAdmin, adminFileChown, &rpc.MethodMeta{
		Name:    "admin:fileChown",
		Summary: "Change file owner and group",
		Params: []rpc.ParamMeta{
			{Name: "uuid", Type: "string", Required: true, Description: "Client UUID"},
			{Name: "path", Type: "string", Required: true, Description: "File path"},
			{Name: "owner", Type: "string", Required: false, Description: "User/Owner name or UID"},
			{Name: "group", Type: "string", Required: false, Description: "Group name or GID"},
		},
		Returns: "null",
	})
}

func execFileRPC(ctx context.Context, uuid, op string, args map[string]interface{}) (any, *rpc.JsonRpcError) {
	if uuid == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "Client UUID is required", nil)
	}
	raw, err := agent_runtime.ExecuteFileOperation(ctx, uuid, op, args)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return json.RawMessage(raw), nil
	}
	return out, nil
}

func adminFileList(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
		Path string `json:"path"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "list", map[string]interface{}{"path": p.Path})
}

func adminFileListRoots(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "list_roots", map[string]interface{}{})
}

func adminFileStat(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
		Path string `json:"path"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "stat", map[string]interface{}{"path": p.Path})
}

func adminFileMkdir(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "mkdir", map[string]interface{}{"path": p.Path, "mode": p.Mode})
}

func adminFileSearch(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID    string `json:"uuid"`
		Path    string `json:"path"`
		Query   string `json:"query"`
		Content bool   `json:"content"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "search", map[string]interface{}{
		"path":    p.Path,
		"query":   p.Query,
		"content": p.Content,
	})
}

func adminFileDelete(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
		Path string `json:"path"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "delete", map[string]interface{}{"path": p.Path})
}

func adminFileMove(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID        string `json:"uuid"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "move", map[string]interface{}{
		"source":      p.Source,
		"destination": p.Destination,
	})
}

func adminFileCopy(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID        string `json:"uuid"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "copy", map[string]interface{}{
		"source":      p.Source,
		"destination": p.Destination,
	})
}

func adminFileChmod(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "chmod", map[string]interface{}{"path": p.Path, "mode": p.Mode})
}

func adminFileChown(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID  string `json:"uuid"`
		Path  string `json:"path"`
		Owner string `json:"owner"`
		Group string `json:"group"`
	}
	if err := req.BindParams(&p); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	return execFileRPC(ctx, p.UUID, "chown", map[string]interface{}{
		"path":  p.Path,
		"owner": p.Owner,
		"group": p.Group,
	})
}
