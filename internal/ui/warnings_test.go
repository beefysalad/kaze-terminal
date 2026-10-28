package ui

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/term"
)

type pipePTY struct {
	r    *io.PipeReader
	w    *io.PipeWriter
	done chan struct{}
	once sync.Once
}

func newPipePTY() *pipePTY {
	r, w := io.Pipe()
	return &pipePTY{r: r, w: w, done: make(chan struct{})}
}
func (p *pipePTY) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipePTY) Write(b []byte) (int, error) { return len(b), nil }
func (p *pipePTY) Resize(int, int) error       { return nil }
func (p *pipePTY) Close() error {
	p.once.Do(func() { p.w.Close(); close(p.done) })
	return nil
}
func (p *pipePTY) Wait() (int, error) { <-p.done; return 0, nil } // Close ends it, so no goroutine leaks

func visibleRow(s *term.Session, row int) string {
	s.Lock()
	defer s.Unlock()
	scr := s.Screen()
	cols, _ := scr.Size()
	var b strings.Builder
	for c := 0; c < cols; c++ {
		if r := scr.VisibleCell(c, row).Rune; r > 0 {
			b.WriteRune(r)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// ConPTY clears the screen when the shell starts, so warnings live in history
// and the view starts scrolled back to show them.
func TestWarningsSurviveTheShellClearingTheScreen(t *testing.T) {
	p := newPipePTY()
	s := term.NewSession(p, 40, 10, nil)
	t.Cleanup(s.Close)
	injectWarnings(s, []string{"font_size too big", "unknown setting \"x\""})

	go p.w.Write([]byte("\x1b[2J\x1b[m\x1b[HPS> "))
	deadline := time.Now().Add(5 * time.Second)
	for visibleRow(s, 2) != "PS>" {
		if time.Now().After(deadline) {
			t.Fatalf("rows: %q / %q / %q", visibleRow(s, 0), visibleRow(s, 1), visibleRow(s, 2))
		}
		time.Sleep(time.Millisecond)
	}
	if got := visibleRow(s, 0); got != "kaze: font_size too big" {
		t.Fatalf("row 0 = %q", got)
	}
	if got := visibleRow(s, 1); got != "kaze: unknown setting \"x\"" {
		t.Fatalf("row 1 = %q", got)
	}
	s.Lock()
	off := s.Screen().ViewOffset()
	dim := s.Screen().VisibleCell(0, 0).Attrs
	s.Unlock()
	if off != 2 {
		t.Fatalf("view offset = %d, want 2", off)
	}
	if dim == 0 {
		t.Fatal("warning text is not dim")
	}
}

func TestNoWarningsLeavesScreenAlone(t *testing.T) {
	p := newPipePTY()
	s := term.NewSession(p, 40, 10, nil)
	t.Cleanup(s.Close)
	injectWarnings(s, nil)
	s.Lock()
	defer s.Unlock()
	if s.Screen().ScrollbackLen() != 0 || s.Screen().ViewOffset() != 0 {
		t.Fatal("no warnings must not touch the screen")
	}
}
