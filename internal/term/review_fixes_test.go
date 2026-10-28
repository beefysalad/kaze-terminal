package term

import (
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/screen"
)

func TestDECRCRestoresPendingWrap(t *testing.T) {
	s := run(5, 3, "abcde\x1b7\x1b[1;1H\x1b8x")
	if row(s, 0) != "abcde" || row(s, 1) != "x" {
		t.Fatalf("%q %q", row(s, 0), row(s, 1))
	}
}

func TestED3KeepsVisibleScreen(t *testing.T) {
	s := run(5, 2, "a\r\nb\r\nc\x1b[3J")
	if s.ScrollbackLen() != 0 || row(s, 0) != "b" || row(s, 1) != "c" {
		t.Fatalf("sb=%d %q %q", s.ScrollbackLen(), row(s, 0), row(s, 1))
	}
}

func TestCursorUpStopsAtTopMargin(t *testing.T) {
	s := run(5, 6, "\x1b[3;5r\x1b[4;1H\x1b[9Ax")
	if row(s, 2) != "x" {
		t.Fatalf("inside: %q", row(s, 2))
	}
	s = run(5, 6, "\x1b[3;5r\x1b[2;1H\x1b[9Ax")
	if row(s, 0) != "x" {
		t.Fatalf("outside: %q", row(s, 0))
	}
	s = run(5, 6, "\x1b[3;5r\x1b[4;1H\x1b[9Fx")
	if row(s, 2) != "x" {
		t.Fatalf("CPL: %q", row(s, 2))
	}
}

func TestCursorDownStopsAtBottomMargin(t *testing.T) {
	s := run(5, 6, "\x1b[3;5r\x1b[4;1H\x1b[9Bx")
	if row(s, 4) != "x" {
		t.Fatalf("inside: %q", row(s, 4))
	}
	s = run(5, 6, "\x1b[3;5r\x1b[6;1H\x1b[9Bx")
	if row(s, 5) != "x" {
		t.Fatalf("outside: %q", row(s, 5))
	}
	s = run(5, 6, "\x1b[3;5r\x1b[4;1H\x1b[9Ex")
	if row(s, 4) != "x" {
		t.Fatalf("CNL: %q", row(s, 4))
	}
}

func TestScrollRegionBottomClamped(t *testing.T) {
	s := run(5, 4, "\x1b[2;999r\x1b[4;1H\n")
	_, top := s.Cursor()
	_ = top
	s = run(5, 4, "a\r\nb\r\nc\r\nd\x1b[2;999r\x1b[4;1H\n")
	if row(s, 0) != "a" || row(s, 1) != "c" || row(s, 2) != "d" {
		t.Fatalf("%q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
}

func TestTabForwardStopsAtLastColumn(t *testing.T) {
	s := run(10, 1, "\x1b[1;9H\t\tx")
	if s.Cell(9, 0).Rune != 'x' {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestICHPushesWideCharOffEdge(t *testing.T) {
	s := run(4, 1, "ab\u3042\x1b[1;1H\x1b[@")
	if row(s, 0) != " ab" {
		t.Fatalf("%q", row(s, 0))
	}
	s = run(4, 1, "a\u3042b\x1b[1;1H\x1b[2@")
	if s.Cell(3, 0).Rune == 'あ' {
		t.Fatalf("half wide char left: %+v", s.Cell(3, 0))
	}
}

func TestDCHAndECHWithWideChar(t *testing.T) {
	s := run(6, 1, "a\u3042b\x1b[1;2H\x1b[P")
	if row(s, 0) != "a b" && row(s, 0) != "ab" {
		t.Fatalf("%q", row(s, 0))
	}
	s = run(6, 1, "a\u3042b\x1b[1;3H\x1b[X")
	if c := s.Cell(1, 0); c.Rune == '\u3042' {
		t.Fatalf("lead survived: %+v", c)
	}
}

func TestEL1AndED1(t *testing.T) {
	s := run(6, 1, "abcdef\x1b[1;3H\x1b[1K")
	if row(s, 0) != "   def" {
		t.Fatalf("%q", row(s, 0))
	}
	s = run(4, 2, "abcd\r\nefgh\x1b[2;2H\x1b[1J")
	if row(s, 0) != "" || row(s, 1) != "  gh" {
		t.Fatalf("%q %q", row(s, 0), row(s, 1))
	}
}

func TestDECSCRestoresPen(t *testing.T) {
	s := run(4, 1, "\x1b[31m\x1b7\x1b[0m\x1b8a")
	if s.Cell(0, 0).FG != screen.Indexed(1) {
		t.Fatalf("%+v", s.Cell(0, 0))
	}
}

func TestILDLNoOpOutsideRegion(t *testing.T) {
	s := run(4, 4, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[1;1H\x1b[L\x1b[M\x1b[4;1H\x1b[L\x1b[M")
	if row(s, 0) != "a" || row(s, 1) != "b" || row(s, 2) != "c" || row(s, 3) != "d" {
		t.Fatalf("%q %q %q %q", row(s, 0), row(s, 1), row(s, 2), row(s, 3))
	}
}

func TestRIAboveRegionJustMovesUp(t *testing.T) {
	s := run(4, 4, "a\r\nb\r\nc\r\nd\x1b[3;4r\x1b[2;1H\x1bM")
	if row(s, 0) != "a" || row(s, 1) != "b" || row(s, 2) != "c" {
		t.Fatalf("%q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
	if _, r := s.Cursor(); r != 0 {
		t.Fatalf("row %d", r)
	}
}

func TestSUAndSD(t *testing.T) {
	s := run(3, 3, "a\r\nb\r\nc\x1b[S")
	if row(s, 0) != "b" || row(s, 1) != "c" || row(s, 2) != "" {
		t.Fatalf("SU %q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
	s = run(3, 3, "a\r\nb\r\nc\x1b[T")
	if row(s, 0) != "" || row(s, 1) != "a" || row(s, 2) != "b" {
		t.Fatalf("SD %q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
}

func TestCUPClampsPastEdge(t *testing.T) {
	s := run(5, 3, "\x1b[99;99Hx")
	if s.Cell(4, 2).Rune != 'x' {
		t.Fatal("not clamped")
	}
}
