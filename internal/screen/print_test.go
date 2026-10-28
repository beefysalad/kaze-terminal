package screen_test

import (
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
)

func TestPrintAdvancesCursor(t *testing.T) {
	s := screen.New(10, 3)
	for _, r := range "hi" {
		s.Print(r)
	}
	if c, _ := s.Cursor(); c != 2 {
		t.Fatalf("col %d", c)
	}
	if s.Cell(1, 0).Rune != 'i' {
		t.Fatal("wrong cell")
	}
}

func TestWideCharOccupiesTwoCells(t *testing.T) {
	s := screen.New(10, 3)
	s.Print('日')
	if got := s.Cell(0, 0); got.Rune != '日' || got.Width != 2 {
		t.Fatalf("lead %+v", got)
	}
	if got := s.Cell(1, 0); got.Width != 0 {
		t.Fatalf("continuation %+v", got)
	}
	if c, _ := s.Cursor(); c != 2 {
		t.Fatalf("col %d", c)
	}
}

func TestWideCharAtLastColumnWrapsWhole(t *testing.T) {
	s := screen.New(5, 3)
	for _, r := range "abcd" {
		s.Print(r)
	}
	s.Print('日')
	if s.Cell(4, 0).Rune != ' ' {
		t.Fatalf("last col should be blank, got %q", s.Cell(4, 0).Rune)
	}
	if s.Cell(0, 1).Rune != '日' {
		t.Fatal("wide char should start next line")
	}
}

func TestOverwritingHalfOfWideCharClearsBothHalves(t *testing.T) {
	s := screen.New(10, 3)
	s.Print('日')
	s.SetCursor(1, 0)
	s.Print('x')
	if got := s.Cell(0, 0); got.Rune != ' ' || got.Width != 1 {
		t.Fatalf("lead not cleared: %+v", got)
	}
	if s.Cell(1, 0).Rune != 'x' {
		t.Fatal("x not written")
	}
}

func TestOverwritingLeadOfWideCharClearsContinuation(t *testing.T) {
	s := screen.New(10, 3)
	s.Print('日')
	s.SetCursor(0, 0)
	s.Print('x')
	if got := s.Cell(1, 0); got.Rune != ' ' || got.Width != 1 {
		t.Fatalf("continuation not cleared: %+v", got)
	}
}

func TestPrintAtBottomRightWrapsAndScrolls(t *testing.T) {
	s := screen.New(3, 2)
	for _, r := range "abcdefg" {
		s.Print(r)
	}
	if s.Cell(0, 0).Rune != 'd' || s.Cell(0, 1).Rune != 'g' {
		t.Fatal("did not scroll")
	}
}

func TestPendingWrapDefersUntilNextRune(t *testing.T) {
	s := screen.New(3, 2)
	for _, r := range "abc" {
		s.Print(r)
	}
	if c, r := s.Cursor(); c != 2 || r != 0 {
		t.Fatalf("cursor %d,%d, want 2,0", c, r)
	}
	s.Print('d')
	if s.Cell(0, 1).Rune != 'd' {
		t.Fatal("d should wrap to next row")
	}
}

func TestCursorMoveCancelsPendingWrap(t *testing.T) {
	s := screen.New(3, 2)
	for _, r := range "abc" {
		s.Print(r)
	}
	s.SetCursor(2, 0)
	s.Print('x')
	if s.Cell(2, 0).Rune != 'x' || s.Cell(0, 1).Rune != ' ' {
		t.Fatal("SetCursor should cancel the pending wrap")
	}
}

func TestCarriageReturnCancelsPendingWrap(t *testing.T) {
	s := screen.New(3, 2)
	for _, r := range "abc" {
		s.Print(r)
	}
	s.CarriageReturn()
	s.Print('x')
	if s.Cell(0, 0).Rune != 'x' {
		t.Fatal("CR should return to column 0 on the same row")
	}
}

func TestAutoWrapOffOverwritesLastColumn(t *testing.T) {
	s := screen.New(3, 2)
	s.SetAutoWrap(false)
	for _, r := range "abcd" {
		s.Print(r)
	}
	if s.Cell(2, 0).Rune != 'd' {
		t.Fatalf("last col = %q", s.Cell(2, 0).Rune)
	}
	if _, r := s.Cursor(); r != 0 {
		t.Fatal("cursor should stay on row 0")
	}
}

func TestPrintUsesPen(t *testing.T) {
	s := screen.New(5, 1)
	p := s.Pen()
	p.FG = screen.Indexed(2)
	p.BG = screen.RGB(1, 2, 3)
	p.Attrs = screen.AttrBold
	s.Print('a')
	got := s.Cell(0, 0)
	if got.FG != screen.Indexed(2) || got.BG != screen.RGB(1, 2, 3) || got.Attrs != screen.AttrBold {
		t.Fatalf("pen not applied: %+v", got)
	}
}

func TestZeroWidthRuneIsDropped(t *testing.T) {
	s := screen.New(5, 1)
	s.Print('e')
	s.Print('́')
	if c, _ := s.Cursor(); c != 1 {
		t.Fatalf("col %d", c)
	}
}

func TestLineFeedAtBottomScrolls(t *testing.T) {
	s := screen.New(3, 2)
	s.Print('a')
	s.LineFeed()
	s.CarriageReturn()
	s.Print('b')
	s.LineFeed()
	if s.Cell(0, 0).Rune != 'b' {
		t.Fatal("did not scroll")
	}
	if _, r := s.Cursor(); r != 1 {
		t.Fatalf("row %d", r)
	}
	if s.Cell(0, 1).Rune != ' ' {
		t.Fatal("new bottom row not blank")
	}
}

func TestLineFeedKeepsColumn(t *testing.T) {
	s := screen.New(5, 3)
	s.Print('a')
	s.Print('b')
	s.LineFeed()
	if c, r := s.Cursor(); c != 2 || r != 1 {
		t.Fatalf("cursor %d,%d", c, r)
	}
}
