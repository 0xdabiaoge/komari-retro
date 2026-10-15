package admin

import (
	"bytes"
	"os"
	"sync"
	"testing"
)

func TestConcurrentThemeArchivesRemainIndependent(t *testing.T) {
	var wg sync.WaitGroup
	ready := make(chan string, 12)
	proceed := make(chan struct{})
	for i := 0; i < cap(ready); i++ {
		wg.Add(1)
		go func(marker byte) {
			defer wg.Done()
			payload := bytes.Repeat([]byte{marker}, 1000)
			name, err := writeThemeArchive(payload)
			if err != nil {
				t.Error(err)
				ready <- ""
				return
			}
			defer os.Remove(name)
			ready <- name
			<-proceed
			actual, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(actual, payload) {
				t.Errorf("archive crossed request boundaries: %v", err)
			}
		}(byte(i))
	}
	names := make(map[string]bool)
	for i := 0; i < cap(ready); i++ {
		name := <-ready
		if name == "" || names[name] {
			t.Error("archive name must be unique")
		}
		names[name] = true
	}
	close(proceed)
	wg.Wait()
}
