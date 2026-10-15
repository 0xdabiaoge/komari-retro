//go:build windows

package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsTerminalShellRoundtrip(t *testing.T) {
	// Test the system shell independently of modules injected by the test host.
	t.Setenv("PSModulePath", filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules"))
	impl, err := newTerminalImpl()
	if err != nil {
		t.Fatal(err)
	}
	defer impl.term.Close()
	output := make(chan []byte, 16)
	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := impl.term.Read(buf)
			if n > 0 {
				output <- buf[:n]
			}
			if err != nil {
				return
			}
		}
	}()
	_, err = impl.term.Write([]byte("Write-Output ('NATIVE_' + 'TERMINAL_OK')\r"))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(12 * time.Second)
	var received []byte
	for {
		select {
		case chunk := <-output:
			received = append(received, chunk...)
			if bytes.Contains(chunk, []byte("\x1b[6n")) {
				_, _ = impl.term.Write([]byte("\x1b[1;1R"))
			}
			if bytes.Contains(received, []byte("NATIVE_TERMINAL_OK")) {
				return
			}
		case <-deadline:
			t.Fatalf("terminal output timed out: %q", received)
		}
	}
}
