package terminal

import (
	"runtime"
	"sync"
	"testing"
)

func TestTerminalMapConcurrentLookup(t *testing.T) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			TerminalSessionsMutex.Lock()
			TerminalSessions["audit-nil"] = nil
			TerminalSessionsMutex.Unlock()
			runtime.Gosched()
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			ForwardTerminal("audit-nil")
			runtime.Gosched()
		}
	}()
	close(start)
	wg.Wait()
	TerminalSessionsMutex.Lock()
	delete(TerminalSessions, "audit-nil")
	TerminalSessionsMutex.Unlock()
}
