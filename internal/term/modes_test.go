package term_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
	"github.com/patrick-gogo/kaze/internal/term"
	"github.com/patrick-gogo/kaze/internal/vt"
)

func run(cols, rows int, input string) *screen.Screen {
	s, _ := run2(cols, rows, input)
	return s
}

func run2(cols, rows int, input string) (*screen.Screen, *term.Applier) {
	s := screen.New(cols, rows)
	a := term.NewApplier(s, io.Discard)
	vt.NewParser(a).Feed([]byte(input))
	return s, a
}

func applierFor(_ *screen.Screen, a *term.Applier) *term.Applier { return a }

func row(s *screen.Screen, r int) string {
	cols, _ := s.Size()
	var b []rune
	for c := 0; c < cols; c++ {
		if cell := s.Cell(c, r); cell.Width != 0 {
			b = append(b, cell.Rune)
		}
	}
	return string(bytes.TrimRight([]byte(string(b)), " "))
}

func TestAltScreenSwitchKeepsMainContent(t *testing.T) {
	s := run(10, 2, "main\x1b[?1049hALT\x1b[?1049l")
	if row(s, 0) != "main" {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestAltScreenRestoresCursorAndClears(t *testing.T) {
	s, a := run2(10, 3, "ab\x1b[?1049h")
	if row(s, 0) != "" || !a.Modes().AltScreen {
		t.Fatalf("%q %+v", row(s, 0), a.Modes())
	}
	vt.NewParser(a).Feed([]byte("zzz\x1b[?1049lX"))
	if row(s, 0) != "abX" {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestAltScreenHasNoScrollback(t *testing.T) {
	s := run(5, 2, "\x1b[?1049ha\r\nb\r\nc\r\nd")
	if s.ScrollbackLen() != 0 {
		t.Fatalf("sb=%d", s.ScrollbackLen())
	}
}

func TestModesToggle(t *testing.T) {
	a := applierFor(run2(10, 2, "\x1b[?1h\x1b[?2004h\x1b[?25l\x1b[?1002h\x1b[?1006h\x1b[?2026h"))
	m := a.Modes()
	if !m.AppCursorKeys || !m.BracketedPaste || m.CursorVisible || !m.MouseButton || !m.MouseSGR || !m.Synchronized {
		t.Fatalf("%+v", m)
	}
	if a.SyncSince().IsZero() {
		t.Fatal("SyncSince should be set")
	}
	vt.NewParser(a).Feed([]byte("\x1b[?2026l\x1b[?1l\x1b[?25h"))
	m = a.Modes()
	if m.Synchronized || m.AppCursorKeys || !m.CursorVisible || !a.SyncSince().IsZero() {
		t.Fatalf("%+v", m)
	}
}

func TestCursorVisibleByDefault(t *testing.T) {
	_, a := run2(10, 2, "")
	if !a.Modes().CursorVisible {
		t.Fatal("cursor should start visible")
	}
}

func TestDeviceStatusReportRepliesCursorPosition(t *testing.T) {
	var buf bytes.Buffer
	s := screen.New(10, 5)
	vt.NewParser(term.NewApplier(s, &buf)).Feed([]byte("\x1b[2;3H\x1b[6n"))
	if buf.String() != "\x1b[2;3R" {
		t.Fatalf("%q", buf.String())
	}
}

func TestDeviceStatusReportOK(t *testing.T) {
	var buf bytes.Buffer
	vt.NewParser(term.NewApplier(screen.New(10, 5), &buf)).Feed([]byte("\x1b[5n"))
	if buf.String() != "\x1b[0n" {
		t.Fatalf("%q", buf.String())
	}
}

func TestPrimaryDeviceAttributesReply(t *testing.T) {
	var buf bytes.Buffer
	vt.NewParser(term.NewApplier(screen.New(10, 5), &buf)).Feed([]byte("\x1b[c"))
	if buf.String() != "\x1b[?62;22c" {
		t.Fatalf("%q", buf.String())
	}
}

func TestSynchronizedModeQueryReply(t *testing.T) {
	var buf bytes.Buffer
	p := vt.NewParser(term.NewApplier(screen.New(10, 5), &buf))
	p.Feed([]byte("\x1b[?2026$p"))
	if buf.String() != "\x1b[?2026;2$y" {
		t.Fatalf("%q", buf.String())
	}
	buf.Reset()
	p.Feed([]byte("\x1b[?2026h\x1b[?2026$p"))
	if buf.String() != "\x1b[?2026;1$y" {
		t.Fatalf("%q", buf.String())
	}
}

func TestOSCTitle(t *testing.T) {
	a := applierFor(run2(10, 1, "\x1b]0;claude\x07"))
	if a.Title() != "claude" {
		t.Fatalf("%q", a.Title())
	}
	vt.NewParser(a).Feed([]byte("\x1b]2;two\x1b\\x1b]1;icon\x07"))
	if a.Title() != "two" {
		t.Fatalf("%q", a.Title())
	}
}
