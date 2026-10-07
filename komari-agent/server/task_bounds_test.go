package server

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCommandOutputAndTimeoutBounds(t *testing.T) {
	var output cappedBuffer
	input := bytes.Repeat([]byte("x"), 256<<10)
	if n, err := output.Write(input); err != nil || n != len(input) {
		t.Fatal(n, err)
	}
	if output.Len() != 128<<10 || !strings.Contains(output.String(), "truncated") {
		t.Fatal("output limit missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	command := "sleep 30"
	if runtime.GOOS == "windows" {
		command = "Start-Sleep -Seconds 30"
	}
	cmd, cleanup, err := buildTaskCommandContext(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	configureTaskCancellation(cmd)
	cmd.WaitDelay = time.Second
	started := time.Now()
	err = cmd.Run()
	if err == nil || time.Since(started) > 5*time.Second {
		t.Fatal("command was not promptly canceled", err, time.Since(started))
	}
}

func TestLoopbackPings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	if latency, err := httpPing(srv.URL, time.Second); err != nil || latency < 0 {
		t.Fatal(latency, err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	if latency, err := tcpPing(listener.Addr().String(), time.Second); err != nil || latency < 0 {
		t.Fatal(latency, err)
	}
}
