package term

import (
	"io"
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/screen"
	"github.com/patrick-gogo/kaze-terminal/internal/vt"
)

func run(cols, rows int, input string) *screen.Screen {
	s := screen.New(cols, rows)
	vt.NewParser(NewApplier(s, io.Discard)).Feed([]byte(input))
	return s
}

func row(s *screen.Screen, r int) string {
	cols, _ := s.Size()
	var b strings.Builder
	for c := 0; c < cols; c++ {
		cell := s.Cell(c, r)
		if cell.Width == 0 {
			continue
		}
		b.WriteRune(cell.Rune)
	}
	return strings.TrimRight(b.String(), " ")
}

func TestCUPIsOneBased(t *testing.T) {
	s := run(10, 5, "\x1b[3;4Hx")
	if row(s, 2) != "   x" {
		t.Fatalf("%q", row(s, 2))
	}
}

func TestEraseInLineToEnd(t *testing.T) {
	s := run(10, 1, "abcdef\x1b[1;3H\x1b[K")
	if row(s, 0) != "ab" {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestEraseDisplayAll(t *testing.T) {
	s := run(5, 2, "abc\r\nde\x1b[2J")
	if row(s, 0) != "" || row(s, 1) != "" {
		t.Fatal("not cleared")
	}
}

func TestScrollRegionKeepsLinesOutsideIt(t *testing.T) {
	s := run(5, 4, "top\r\nA\r\nB\r\nbot\x1b[2;3r\x1b[3;1H\n")
	if row(s, 0) != "top" || row(s, 1) != "B" || row(s, 2) != "" || row(s, 3) != "bot" {
		t.Fatalf("%q %q %q %q", row(s, 0), row(s, 1), row(s, 2), row(s, 3))
	}
}

func TestBackspaceCarriageReturnTab(t *testing.T) {
	s := run(20, 1, "ab\bX\rZ\tT")
	if row(s, 0) != "ZX      T" {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestDeleteAndInsertChars(t *testing.T) {
	s := run(10, 1, "abcdef\x1b[1;2H\x1b[2P\x1b[1@")
	if row(s, 0) != "a def" {
		t.Fatalf("%q", row(s, 0))
	}
}

func TestRegionScrollDirtiesOnlyRegionRows(t *testing.T) {
	s := run(5, 5, "a\r\nb\r\nc\r\nd\r\ne\x1b[2;4r\x1b[4;1H")
	s.TakeDirty()
	vt.NewParser(NewApplier(s, io.Discard)).Feed([]byte("\n"))
	d := s.TakeDirty()
	if len(d) != 3 || d[0] != 1 || d[2] != 3 {
		t.Fatalf("dirty %v", d)
	}
}

func TestRegionScrollSkipsScrollback(t *testing.T) {
	s := run(5, 4, "a\r\nb\r\nc\x1b[1;3r\x1b[3;1H\n\n")
	if s.ScrollbackLen() != 0 {
		t.Fatalf("scrollback %d", s.ScrollbackLen())
	}
	s = run(5, 2, "a\r\nb\r\nc")
	if s.ScrollbackLen() != 1 {
		t.Fatalf("full-screen scroll should record, got %d", s.ScrollbackLen())
	}
}

func TestReverseIndexAtTopScrollsDown(t *testing.T) {
	s := run(5, 3, "a\r\nb\r\nc\x1b[1;1H\x1bM")
	if row(s, 0) != "" || row(s, 1) != "a" || row(s, 2) != "b" {
		t.Fatalf("%q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
}

func TestInsertAndDeleteLines(t *testing.T) {
	s := run(5, 4, "a\r\nb\r\nc\r\nd\x1b[2;1H\x1b[L")
	if row(s, 1) != "" || row(s, 2) != "b" || row(s, 3) != "c" {
		t.Fatalf("%q %q %q", row(s, 1), row(s, 2), row(s, 3))
	}
	vt.NewParser(NewApplier(s, io.Discard)).Feed([]byte("\x1b[2M"))
	if row(s, 0) != "a" || row(s, 1) != "c" || row(s, 2) != "" {
		t.Fatalf("%q %q %q", row(s, 0), row(s, 1), row(s, 2))
	}
}

func TestSaveRestoreCursor(t *testing.T) {
	s := run(10, 3, "\x1b[2;4H\x1b7\x1b[1;1H\x1b8x")
	if row(s, 1) != "   x" {
		t.Fatalf("%q", row(s, 1))
	}
}

func TestEraseCharsAndFullReset(t *testing.T) {
	s := run(10, 1, "abcdef\x1b[1;2H\x1b[3X")
	if row(s, 0) != "a   ef" {
		t.Fatalf("%q", row(s, 0))
	}
	vt.NewParser(NewApplier(s, io.Discard)).Feed([]byte("\x1bc"))
	if c, r := s.Cursor(); row(s, 0) != "" || c != 0 || r != 0 {
		t.Fatal("reset failed")
	}
}

func TestEraseSplitsWideChar(t *testing.T) {
	s := run(6, 1, "あい\x1b[1;2H\x1b[K")
	if row(s, 0) != "" {
		t.Fatalf("%q", row(s, 0))
	}
}
