package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

var (
	filePendingMu sync.Mutex
	filePending   = make(map[string]chan v2.FileResult)
)

func randomFileRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HandleFileResult receives results pushed by the agent
func HandleFileResult(result v2.FileResult) {
	filePendingMu.Lock()
	ch, exists := filePending[result.RequestID]
	filePendingMu.Unlock()
	if exists && ch != nil {
		select {
		case ch <- result:
		default:
		}
	}
}

// ExecuteFileOperation sends a file operation to the client agent and waits for the result
func ExecuteFileOperation(ctx context.Context, uuid, op string, args map[string]interface{}) (json.RawMessage, error) {
	reqID := randomFileRequestID()
	ch := make(chan v2.FileResult, 1)

	filePendingMu.Lock()
	filePending[reqID] = ch
	filePendingMu.Unlock()

	defer func() {
		filePendingMu.Lock()
		delete(filePending, reqID)
		filePendingMu.Unlock()
	}()

	opReq := v2.FileOperation{
		UUID:      uuid,
		RequestID: reqID,
		Op:        op,
		Args:      args,
	}

	dispatched := DispatchV2Event(uuid, v2.MethodAgentFile, opReq)
	if !dispatched {
		return nil, errors.New("agent is offline or unreachable")
	}

	// Default timeout if context has none
	timeout := 45 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, errors.New("timeout waiting for agent response")
	case res := <-ch:
		if !res.OK {
			if res.Error != "" {
				return nil, errors.New(res.Error)
			}
			return nil, errors.New("file operation failed")
		}
		return res.Result, nil
	}
}
