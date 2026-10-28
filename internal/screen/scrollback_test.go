package screen_test

import (
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
)

func TestScrolledLinesGoToScrollback(t *testing.T) {
	s := screen.NewWithScrollback(3, 2, 100)
	for _, r := range "aaabbbccc" {
		s.Print(r)
	}
	if s.ScrollbackLen() != 1 || s.ScrollbackLine(0)[0].Rune != 'a' {
		t.Fatal("expected 'aaa' in scrollback")
	}
}

func TestScrollbackIsCapped(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 5)
	for i := 0; i < 20; i++ {
		s.Print(rune('a' + i))
		s.LineFeed()
		s.CarriageReturn()
	}
	if s.ScrollbackLen() != 5 {
		t.Fatalf("len %d", s.ScrollbackLen())
	}
	if s.ScrollbackLine(0)[0].Rune != 'p' || s.ScrollbackLine(4)[0].Rune != 't' {
		t.Fatal("ring should hold the newest 5 lines, oldest first")
	}
}

func TestViewOffsetShowsOlderLines(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 100)
	for _, r := range "abc" {
		s.Print(r)
		s.LineFeed()
		s.CarriageReturn()
	}
	s.SetViewOffset(2)
	if s.VisibleCell(0, 0).Rune != 'b' {
		t.Fatalf("got %q", s.VisibleCell(0, 0).Rune)
	}
}

func TestViewOffsetClampsAndMarksDirty(t *testing.T) {
	s := screen.NewWithScrollback(1, 2, 10)
	s.TakeDirty()
	s.SetViewOffset(5)
	if s.ViewOffset() != 0 {
		t.Fatalf("offset %d, want clamp to 0", s.ViewOffset())
	}
	for _, r := range "abc" {
		s.Print(r)
		s.LineFeed()
		s.CarriageReturn()
	}
	s.TakeDirty()
	s.SetViewOffset(99)
	if s.ViewOffset() != s.ScrollbackLen() {
		t.Fatalf("offset %d, want %d", s.ViewOffset(), s.ScrollbackLen())
	}
	if len(s.TakeDirty()) != 2 {
		t.Fatal("SetViewOffset must mark all rows dirty")
	}
	s.SetViewOffset(-3)
	if s.ViewOffset() != 0 {
		t.Fatal("negative offset must clamp to 0")
	}
}

func TestVisibleCellSpansScrollbackAndScreen(t *testing.T) {
	s := screen.NewWithScrollback(2, 2, 10)
	for _, r := range "aabbcc" { // a, b scrolled or live depending on offset
		s.Print(r)
	}
	// scrollback: aa ; screen: bb / cc
	s.SetViewOffset(1)
	if s.VisibleCell(0, 0).Rune != 'a' || s.VisibleCell(0, 1).Rune != 'b' {
		t.Fatal("offset 1 should show aa over bb")
	}
}

func TestVisibleCellOutOfRangeIsBlank(t *testing.T) {
	s := screen.NewWithScrollback(2, 1, 10)
	if s.VisibleCell(5, 0) != s.Cell(5, 0) {
		t.Fatal("out of range should be blank")
	}
}
