package term_test

import (
	"testing"
	"time"

	"github.com/patrick-gogo/kaze/internal/term"
)

// Output already waiting on the PTY must not be applied before the pre-feed.
func TestPrefeedRunsBeforeAnyOutput(t *testing.T) {
	f := newFake()
	go f.outW.Write([]byte("PS> ")) // waiting before the session exists
	var blank bool
	s := term.NewSessionOpts(f, 20, 3, term.Opts{Prefeed: func(s *term.Session) {
		time.Sleep(50 * time.Millisecond) // a reader running already would apply the prompt now
		s.Lock()
		blank = s.Screen().Cell(0, 0).Rune == ' '
		s.Applier().Print('W')
		s.Unlock()
	}}, nil)
	t.Cleanup(s.Close)
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.Lock()
		c := s.Screen().Cell(1, 0).Rune
		s.Unlock()
		if c == 'P' {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("prompt never arrived after the pre-feed")
		}
		time.Sleep(time.Millisecond)
	}
	if !blank {
		t.Fatal("the shell's output was applied before the pre-feed ran")
	}
}

func TestOptsScrollbackCapsHistory(t *testing.T) {
	f := newFake()
	s := term.NewSessionOpts(f, 5, 2, term.Opts{Scrollback: 2}, nil)
	t.Cleanup(s.Close)
	s.Lock()
	defer s.Unlock()
	for i := 0; i < 10; i++ {
		s.Screen().LineFeed()
	}
	if n := s.Screen().ScrollbackLen(); n != 2 {
		t.Fatalf("history %d, want 2", n)
	}
}
