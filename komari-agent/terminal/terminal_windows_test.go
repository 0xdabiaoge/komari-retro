//go:build windows

package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
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
	done := make(chan struct{})
	defer close(done)
	output := make(chan []byte, 16)
	go func() {
		defer close(output)
		for {
			buf := make([]byte, 4096)
			n, err := impl.term.Read(buf)
			if n > 0 {
				select {
				case output <- buf[:n]:
				case <-done:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	// ConPTY can accept input before PowerShell/PSReadLine has finished startup.
	// An early Enter can then be consumed without executing the echoed command.
	// Wait for the shell's first prompt, rather than relying on runner speed.
	prompt := regexp.MustCompile(`(?:PS [^\r\n]*|[A-Za-z]:\\[^\r\n]*)> ?`)
	deadline := time.After(30 * time.Second)
	var received []byte
	sent := false
	answeredQueries := 0
	for {
		select {
		case chunk, ok := <-output:
			if !ok {
				t.Fatalf("terminal closed before command completed: %q", received)
			}
			received = append(received, chunk...)
			// Queries may be split across reads. Answer each complete query once.
			for queries := bytes.Count(received, []byte("\x1b[6n")); answeredQueries < queries; answeredQueries++ {
				if _, err := impl.term.Write([]byte("\x1b[1;1R")); err != nil {
					t.Fatal(err)
				}
			}
			if !sent && prompt.Match(received) {
				if _, err := impl.term.Write([]byte("Write-Output ('NATIVE_' + 'TERMINAL_OK')\r")); err != nil {
					t.Fatal(err)
				}
				sent = true
			}
			if sent && bytes.Contains(received, []byte("NATIVE_TERMINAL_OK")) {
				return
			}
		case <-deadline:
			t.Fatalf("terminal output timed out: %q", received)
		}
	}
}
