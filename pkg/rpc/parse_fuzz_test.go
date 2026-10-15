package rpc

import "testing"

func FuzzParseRequests(f *testing.F) {
	for _, seed := range []string{"", "{", "[]", "null", `{"jsonrpc":"2.0","id":1,"method":"rpc.ping"}`, `[{"jsonrpc":"2.0","id":{},"method":"rpc.ping"}]`, "\xff\xfe"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		requests, err := ParseRequests(data)
		if err != nil {
			return
		}
		if len(requests) == 0 {
			t.Fatal("successful parse returned no requests")
		}
		for _, request := range requests {
			if request == nil || request.Validate() != nil {
				t.Fatal("parser accepted an invalid request")
			}
		}
	})
}
