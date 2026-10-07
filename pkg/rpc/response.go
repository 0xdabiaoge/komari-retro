package rpc

import "encoding/json"

// JsonRpcResponse JSON-RPC 2.0 响应
// 成功时包含 result，失败时包含 error；二者互斥。
// 在 Notification 情况下服务器不会发送任何响应。
type JsonRpcResponse struct {
	Version string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JsonRpcError `json:"error,omitempty"`
}

// MarshalJSON keeps a null success result on the wire while ensuring errors
// never contain result. The response id is required, including id:null for
// parse errors and invalid requests.
func (r JsonRpcResponse) MarshalJSON() ([]byte, error) {
	if r.Error != nil {
		return json.Marshal(struct {
			Version string        `json:"jsonrpc"`
			ID      any           `json:"id"`
			Error   *JsonRpcError `json:"error"`
		}{r.Version, r.ID, r.Error})
	}
	return json.Marshal(struct {
		Version string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  any    `json:"result"`
	}{r.Version, r.ID, r.Result})
}

// SuccessResponse 构造成功响应
func SuccessResponse(id any, result any) *JsonRpcResponse {
	return &JsonRpcResponse{Version: RPC_VERSION, ID: id, Result: result}
}

// ErrorResponse 构造失败响应
func ErrorResponse(id any, code int, msg string, data any) *JsonRpcResponse {
	return &JsonRpcResponse{Version: RPC_VERSION, ID: id, Error: &JsonRpcError{Code: code, Message: msg, Data: data}}
}

// InternalErrorResponse 统一内部错误
func InternalErrorResponse(id any, err error) *JsonRpcResponse {
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	return ErrorResponse(id, InternalError, msg, nil)
}
