package screen_test

import (
	"reflect"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/screen"
)

func TestNewScreenIsBlank(t *testing.T) {
	s := screen.New(80, 24)
	c, r := s.Size()
	if c != 80 || r != 24 {
		t.Fatalf("size %dx%d", c, r)
	}
	if got := s.Cell(0, 0); got.Rune != ' ' || got.Width != 1 || got.FG != screen.DefaultColor {
		t.Fatalf("blank cell = %+v", got)
	}
}

func TestDirtyRowsReportedOnceThenCleared(t *testing.T) {
	s := screen.New(80, 24)
	s.TakeDirty()
	s.SetCell(3, 5, screen.Cell{Rune: 'a', Width: 1})
	s.SetCell(4, 5, screen.Cell{Rune: 'b', Width: 1})
	s.SetCell(0, 2, screen.Cell{Rune: 'c', Width: 1})
	if got := s.TakeDirty(); !reflect.DeepEqual(got, []int{2, 5}) {
		t.Fatalf("dirty = %v", got)
	}
	if got := s.TakeDirty(); len(got) != 0 {
		t.Fatalf("second take = %v", got)
	}
}

func TestSettingSameCellIsNotDirty(t *testing.T) {
	s := screen.New(80, 24)
	s.TakeDirty()
	s.SetCell(0, 0, s.Cell(0, 0))
	if got := s.TakeDirty(); len(got) != 0 {
		t.Fatalf("dirty = %v", got)
	}
}

func TestCursorIsClamped(t *testing.T) {
	s := screen.New(80, 24)
	s.SetCursor(500, -3)
	if c, r := s.Cursor(); c != 79 || r != 0 {
		t.Fatalf("cursor %d,%d", c, r)
	}
}
