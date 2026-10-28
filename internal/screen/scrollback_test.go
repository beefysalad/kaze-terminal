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
	blank := screen.Cell{Rune: ' ', Width: 1, FG: screen.DefaultColor, BG: screen.DefaultColor}
	s := screen.NewWithScrollback(2, 1, 10)
	if s.VisibleCell(5, 0) != blank || s.VisibleCell(0, 3) != blank {
		t.Fatal("out of range should be blank")
	}
	for _, r := range "abcd" {
		s.Print(r)
		s.LineFeed()
		s.CarriageReturn()
	}
	s.SetViewOffset(3) // larger than the 1 visible row
	if s.VisibleCell(5, 0) != blank || s.VisibleCell(0, 1) != blank || s.VisibleCell(-1, 0) != blank {
		t.Fatal("out of range with an offset should be blank")
	}
	if s.VisibleCell(0, 0).Rune != 'b' {
		t.Fatalf("got %q", s.VisibleCell(0, 0).Rune)
	}
}

func feed1(s *screen.Screen, text string) {
	for _, r := range text {
		s.Print(r)
		s.LineFeed()
		s.CarriageReturn()
	}
}

func TestViewStaysAnchoredWhileRingFills(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 10)
	feed1(s, "abc")
	s.SetViewOffset(2)
	feed1(s, "d")
	if s.VisibleCell(0, 0).Rune != 'b' {
		t.Fatalf("got %q", s.VisibleCell(0, 0).Rune)
	}
}

func TestViewStaysAnchoredWhenRingIsFull(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 5)
	feed1(s, "abcdefgh") // ring holds d..h
	s.SetViewOffset(2)   // shows g
	if s.VisibleCell(0, 0).Rune != 'g' {
		t.Fatalf("setup: got %q", s.VisibleCell(0, 0).Rune)
	}
	feed1(s, "i") // evicts d; g must not move
	if s.VisibleCell(0, 0).Rune != 'g' {
		t.Fatalf("view crept to %q", s.VisibleCell(0, 0).Rune)
	}
	feed1(s, "jkl") // g is evicted after 'l'? ring is g..k then h..l
	if s.ViewOffset() > s.ScrollbackLen() {
		t.Fatal("offset must stay within history")
	}
	feed1(s, "mnop")
	if s.VisibleCell(0, 0).Rune != s.ScrollbackLine(0)[0].Rune {
		t.Fatal("once evicted, the view pins to the oldest line")
	}
}

func TestEvictedCountsLinesDroppedFromTheRing(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 3)
	feed := func(n int) {
		for i := 0; i < n; i++ {
			s.Print('x')
			s.LineFeed()
			s.CarriageReturn()
		}
	}
	feed(3) // fills the ring exactly
	if got := s.Evicted(); got != 0 {
		t.Fatalf("evicted %d before the ring was full, want 0", got)
	}
	feed(5)
	if got := s.Evicted(); got != 5 {
		t.Fatalf("evicted %d after 5 more lines, want 5", got)
	}
	s.EraseInDisplay(3) // clearing history drops every held line
	if got := s.Evicted(); got != 8 {
		t.Fatalf("evicted %d after ED 3, want 8", got)
	}
}

func TestEvictedCountsLinesWithNoScrollback(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 0)
	for i := 0; i < 4; i++ {
		s.Print('x')
		s.LineFeed()
		s.CarriageReturn()
	}
	if got := s.Evicted(); got != 4 {
		t.Fatalf("evicted %d with no history, want 4", got)
	}
}
