package term_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze/internal/term"
)

// A big paste into a shell that reads slowly must not stall the UI thread.
func TestHugePasteWriteDoesNotBlock(t *testing.T) {
	f := newFake()
	f.block = make(chan struct{}) // the PTY accepts nothing until the test ends
	s := term.NewSession(f, 80, 24, nil)
	t.Cleanup(func() {
		close(f.block)
		s.Close()
	})
	payload := bytes.Repeat([]byte("x"), 1<<20)

	s.Write([]byte("warm")) // the writer goroutine is now parked in the blocked PTY write
	start := time.Now()
	s.Write(payload)
	if d := time.Since(start); d > 5*time.Millisecond {
		t.Fatalf("1MB Write took %v, want under 5ms", d)
	}
}
