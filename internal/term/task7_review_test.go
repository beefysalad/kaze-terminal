package term_test

import (
	"bytes"
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
	"github.com/patrick-gogo/kaze/internal/term"
	"github.com/patrick-gogo/kaze/internal/vt"
)

func TestFocusEventsModeIsTrackedAndReported(t *testing.T) {
	var buf bytes.Buffer
	a := term.NewApplier(screen.New(10, 2), &buf)
	p := vt.NewParser(a)
	p.Feed([]byte("\x1b[?1004h\x1b[?1004$p"))
	if !a.Modes().FocusEvents || buf.String() != "\x1b[?1004;1$y" {
		t.Fatalf("on: %+v %q", a.Modes(), buf.String())
	}
	buf.Reset()
	p.Feed([]byte("\x1b[?1004l\x1b[?1004$p"))
	if a.Modes().FocusEvents || buf.String() != "\x1b[?1004;2$y" {
		t.Fatalf("off: %+v %q", a.Modes(), buf.String())
	}
}

func TestDECSCOnAltScreenDoesNotBreak1049Restore(t *testing.T) {
	s := run(10, 4, "\x1b[2;3H\x1b[?1049h\x1b[4;6H\x1b7\x1b[?1049l")
	if col, r := s.Cursor(); col != 2 || r != 1 {
		t.Fatalf("cursor %d,%d, want 2,1", col, r)
	}
}

func TestResetLeavesAltScreenAndResetsModes(t *testing.T) {
	s, a := run2(10, 3, "main\x1b[?1049h\x1b[?1h\x1b[?25l\x1b[?2004h\x1b[?1002h\x1b[?1004h\x1b[?2026h\x1bc")
	if s.OnAltScreen() {
		t.Fatal("still on the alt screen after RIS")
	}
	if got := a.Modes(); got != (term.Modes{CursorVisible: true}) {
		t.Fatalf("modes %+v", got)
	}
	if !a.SyncSince().IsZero() {
		t.Fatal("SyncSince should be cleared")
	}
	if row(s, 0) != "" {
		t.Fatalf("RIS should clear the main screen, row 0 = %q", row(s, 0))
	}
}

func TestResetRestoresFullScrollRegion(t *testing.T) {
	s := run(5, 4, "\x1b[2;3r\x1bca\r\nb\r\nc\r\nd\n")
	if row(s, 0) != "b" {
		t.Fatalf("row 0 = %q", row(s, 0))
	}
}

func TestAltScreenSwitchResetsScrollRegion(t *testing.T) {
	s := run(5, 4, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[?1049h\x1b[?1049l\x1b[4;1H\n")
	if row(s, 0) != "b" {
		t.Fatalf("leaving alt: row 0 = %q", row(s, 0))
	}
	s = run(5, 4, "\x1b[2;3r\x1b[?1049h\x1b[1;1Ha\r\nb\r\nc\r\nd\n")
	if row(s, 0) != "b" {
		t.Fatalf("entering alt: row 0 = %q", row(s, 0))
	}
}
