//go:build !windows

package subagents

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO in a transcript's place never blocks the reader: it is turned away
// before it is opened.
func TestReadersSkipFIFO(t *testing.T) {
	p := filepath.Join(t.TempDir(), "agent-x.jsonl")
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Skip("no FIFO:", err)
	}
	done := make(chan bool, 1)
	go func() {
		_, ok := FirstPrompt(p)
		var tr Transcript
		done <- ok || tr.Read(p) == nil
	}()
	select {
	case bad := <-done:
		if bad {
			t.Fatal("a FIFO read as a transcript")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reading a FIFO blocked") // the goroutine stays blocked; the test binary exits
	}
}
