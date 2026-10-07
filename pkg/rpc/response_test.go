package rpc

import (
	"encoding/json"
	"testing"
)

func TestResponseWireContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *JsonRpcResponse
		want     string
	}{
		{"void success", SuccessResponse(7, nil), `{"jsonrpc":"2.0","id":7,"result":null}`},
		{"false success", SuccessResponse("a", false), `{"jsonrpc":"2.0","id":"a","result":false}`},
		{"invalid request", ErrorResponse(nil, InvalidRequest, "invalid", nil), `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.response)
			if err != nil || string(got) != tc.want {
				t.Fatalf("response = %s, error = %v; want %s", got, err, tc.want)
			}
		})
	}
}
