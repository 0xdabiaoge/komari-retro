// Exercise real Agent WebSocket framing against a disposable loopback fixture.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func run() error {
	var fixture struct {
		Base string `json:"base"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&fixture); err != nil {
		return err
	}
	u, err := url.Parse(fixture.Base)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Scheme != "http" {
		return fmt.Errorf("Agent scale test requires the disposable loopback fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sem := make(chan struct{}, 50)
	failures := make(chan error, 500)
	var ready, rounds, finished sync.WaitGroup
	ready.Add(500)
	rounds.Add(500)
	finished.Add(500)
	for i := 1; i <= 500; i++ {
		go func(id int) {
			defer finished.Done()
			completed := false
			defer func() {
				if !completed {
					rounds.Done()
				}
			}()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				ready.Done()
				return
			}
			address := "ws://" + u.Host + "/api/clients/v2/rpc?token=fixture-token-" + fmt.Sprint(id)
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, address, nil)
			if err != nil {
				<-sem
				ready.Done()
				failures <- fmt.Errorf("fixture Agent dial: %w", err)
				cancel()
				return
			}
			defer conn.Close()
			report := func(round int) error {
				requestID := fmt.Sprintf("fixture-%d-%d", id, round)
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				payload := map[string]any{"jsonrpc": "2.0", "id": requestID, "method": "agent.report", "params": map[string]any{"report": map[string]any{"cpu": map[string]any{"usage": 25}, "ram": map[string]any{"used": 512, "total": 1024}, "uptime": 3600}}}
				if err := conn.WriteJSON(payload); err != nil {
					return err
				}
				for {
					var response struct {
						ID    string `json:"id"`
						Error any    `json:"error"`
					}
					if err := conn.ReadJSON(&response); err != nil {
						return err
					}
					if response.ID == requestID {
						if response.Error != nil {
							return fmt.Errorf("Agent report rejected")
						}
						return nil
					}
				}
			}
			err = report(0)
			<-sem
			ready.Done()
			if err != nil {
				failures <- err
				cancel()
				return
			}
			for round := 1; round <= 3; round++ {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				if err = report(round); err != nil {
					failures <- err
					cancel()
					return
				}
			}
			// Retain early sockets until every Agent has completed its report rounds.
			// Continue reporting while waiting, as real Agents do.
			completed = true
			rounds.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
					if err = report(4); err != nil {
						failures <- err
						cancel()
						return
					}
				}
			}
		}(i)
	}
	ready.Wait()
	done := make(chan struct{})
	go func() { rounds.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	cancel()
	finished.Wait()
	select {
	case err := <-failures:
		return err
	default:
	}
	if ctx.Err() == context.DeadlineExceeded {
		return ctx.Err()
	}
	fmt.Println("PASS 500 simultaneous authenticated Agent WebSockets, 50 concurrent handshakes and three report rounds")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
