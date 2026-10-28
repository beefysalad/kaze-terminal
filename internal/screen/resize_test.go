package screen_test

import (
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
)

func printAt(s *screen.Screen, col, row int, text string) {
	s.SetCursor(col, row)
	for _, r := range text {
		s.Print(r)
	}
}

func rowText(s *screen.Screen, r int) string {
	cols, _ := s.Size()
	var b strings.Builder
	for c := 0; c < cols; c++ {
		if cell := s.Cell(c, r); cell.Width != 0 {
			b.WriteRune(cell.Rune)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func TestResizeGrowPadsWithBlanks(t *testing.T) {
	s := screen.New(4, 2)
	printAt(s, 0, 0, "abcd")
	printAt(s, 0, 1, "ef")
	s.Resize(6, 3)
	if c, r := s.Size(); c != 6 || r != 3 {
		t.Fatalf("size %dx%d", c, r)
	}
	if rowText(s, 0) != "abcd" || rowText(s, 1) != "ef" || rowText(s, 2) != "" {
		t.Fatalf("rows %q %q %q", rowText(s, 0), rowText(s, 1), rowText(s, 2))
	}
	if got := s.Cell(5, 2); got.Rune != ' ' || got.Width != 1 {
		t.Fatalf("pad cell = %+v", got)
	}
}

func TestResizeShrinkTruncatesColumns(t *testing.T) {
	s := screen.New(6, 2)
	printAt(s, 0, 0, "abcdef")
	s.Resize(3, 2)
	if rowText(s, 0) != "abc" {
		t.Fatalf("row 0 = %q", rowText(s, 0))
	}
	if col, _ := s.Cursor(); col != 2 {
		t.Fatalf("cursor col = %d", col)
	}
}

func TestResizeShrinkRowsKeepsCursorLine(t *testing.T) {
	s := screen.NewWithScrollback(4, 4, 100)
	for r, txt := range []string{"r0", "r1", "r2", "r3"} {
		printAt(s, 0, r, txt)
	}
	s.SetCursor(0, 3)
	s.Resize(4, 2)
	if rowText(s, 0) != "r2" || rowText(s, 1) != "r3" {
		t.Fatalf("rows %q %q", rowText(s, 0), rowText(s, 1))
	}
	if _, row := s.Cursor(); row != 1 {
		t.Fatalf("cursor row = %d", row)
	}
	if s.ScrollbackLen() != 2 || s.ScrollbackLine(1)[1].Rune != '1' {
		t.Fatalf("scrollback len %d", s.ScrollbackLen())
	}
}

func TestResizeShrinkRowsAboveCursorTruncatesBottom(t *testing.T) {
	s := screen.New(4, 4)
	printAt(s, 0, 0, "top")
	printAt(s, 0, 3, "bot")
	s.SetCursor(0, 0)
	s.Resize(4, 2)
	if rowText(s, 0) != "top" || rowText(s, 1) != "" {
		t.Fatalf("rows %q %q", rowText(s, 0), rowText(s, 1))
	}
}

func TestResizeResetsScrollRegion(t *testing.T) {
	s := screen.New(4, 10)
	s.SetScrollRegion(2, 5)
	s.Resize(4, 3)
	// With a full-screen region, a line feed on the last row scrolls the whole screen.
	printAt(s, 0, 0, "x")
	s.SetCursor(0, 2)
	s.LineFeed()
	if rowText(s, 0) != "" {
		t.Fatalf("region not reset; row 0 = %q", rowText(s, 0))
	}
}

func TestResizeSplitWideCharIsBlanked(t *testing.T) {
	s := screen.New(4, 1)
	printAt(s, 0, 0, "ab日")
	s.Resize(3, 1)
	if got := s.Cell(2, 0); got.Width != 1 || got.Rune != ' ' {
		t.Fatalf("orphan wide lead = %+v", got)
	}
}

func TestResizeMarksAllDirty(t *testing.T) {
	s := screen.New(4, 2)
	s.TakeDirty()
	s.Resize(4, 3)
	if got := s.TakeDirty(); len(got) != 3 {
		t.Fatalf("dirty = %v", got)
	}
}

func TestResizeClampsToOneByOne(t *testing.T) {
	s := screen.New(4, 2)
	s.Resize(0, -3)
	if c, r := s.Size(); c != 1 || r != 1 {
		t.Fatalf("size %dx%d", c, r)
	}
}

func TestResizeResizesBothBuffers(t *testing.T) {
	s := screen.New(4, 2)
	printAt(s, 0, 0, "main")
	s.UseAltScreen(true)
	printAt(s, 0, 0, "alt")
	s.Resize(6, 3)
	printAt(s, 0, 2, "zzzzzz") // would index out of range if alt kept the old size
	s.UseAltScreen(false)
	if c, r := s.Size(); c != 6 || r != 3 {
		t.Fatalf("size %dx%d", c, r)
	}
	if rowText(s, 0) != "main" {
		t.Fatalf("main row 0 = %q", rowText(s, 0))
	}
	printAt(s, 0, 2, "yyyyyy")
	if rowText(s, 2) != "yyyyyy" {
		t.Fatalf("main row 2 = %q", rowText(s, 2))
	}
	s.UseAltScreen(true)
	if rowText(s, 0) != "" {
		t.Fatal("re-entering alt should be blank")
	}
}

func TestResizeOldScrollbackStillReadable(t *testing.T) {
	s := screen.NewWithScrollback(3, 1, 10)
	feed1(s, "a")
	s.Resize(8, 1)
	if s.ScrollbackLine(0)[0].Rune != 'a' {
		t.Fatal("scrollback lost")
	}
	s.SetViewOffset(1)
	if got := s.VisibleCell(7, 0); got.Rune != ' ' {
		t.Fatalf("narrow history line should pad, got %+v", got)
	}
}

func TestResizeWiderClearsSavedWrapPending(t *testing.T) {
	for name, restore := range map[string]func(*screen.Screen){
		"decsc":   func(s *screen.Screen) { s.RestoreCursor() },
		"alt1049": func(s *screen.Screen) { s.RestoreAltCursor() },
	} {
		s := screen.New(4, 2)
		printAt(s, 0, 0, "abcd") // wrap pending at the right edge
		s.SaveCursor()
		s.SaveAltCursor()
		s.Resize(8, 2)
		restore(s)
		s.Print('X')
		if c := s.Cell(3, 0); c.Rune != 'X' {
			t.Fatalf("%s: cell(3,0) = %q, want X at the restored position", name, c.Rune)
		}
		if col, row := s.Cursor(); row != 0 || col != 4 {
			t.Fatalf("%s: cursor (%d,%d), want (4,0)", name, col, row)
		}
	}
}

func TestResizeShiftsSavedRowOnAltScreen(t *testing.T) {
	s := screen.New(4, 4)
	s.UseAltScreen(true)
	s.SetCursor(0, 2)
	s.SaveCursor()
	s.SetCursor(0, 3)
	s.Resize(4, 2) // drops 2 rows
	s.RestoreCursor()
	if _, row := s.Cursor(); row != 0 {
		t.Fatalf("restored row = %d, want 0", row)
	}
}
