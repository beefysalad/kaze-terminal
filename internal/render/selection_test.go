package render

import (
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/screen"
)

func printAll(s *screen.Screen, text string) {
	for _, r := range text {
		s.Print(r)
	}
}

func TestSelectedTextAcrossLinesTrimsTrailingSpaces(t *testing.T) {
	s := screen.New(6, 2)
	printAll(s, "ab  ")
	s.SetCursor(0, 1)
	printAll(s, "cd")
	got := SelectedText(s, Selection{Start: Point{0, 0}, End: Point{5, 1}, Active: true})
	if got != "ab\ncd" {
		t.Fatalf("got %q", got)
	}
}

func TestSelectionBackwardsIsNormalized(t *testing.T) {
	s := screen.New(6, 1)
	printAll(s, "abcdef")
	sel := Selection{Start: Point{4, 0}, End: Point{1, 0}, Active: true}
	if got := SelectedText(s, sel); got != "bcde" {
		t.Fatalf("got %q", got)
	}
	if !sel.Contains(Point{2, 0}) || sel.Contains(Point{5, 0}) {
		t.Fatal("Contains must honor the normalized range")
	}
	if (Selection{Start: Point{0, 0}, End: Point{5, 0}}).Contains(Point{1, 0}) {
		t.Fatal("inactive selection contains nothing")
	}
}

func TestWideCharIsSelectedWhole(t *testing.T) {
	s := screen.New(6, 1)
	printAll(s, "a日b")
	// col 1 is the lead of 日, col 2 its continuation
	for _, col := range []int{1, 2} {
		sel := Selection{Start: Point{col, 0}, End: Point{col, 0}, Active: true}
		if got := SelectedText(s, sel); got != "日" {
			t.Fatalf("col %d: got %q", col, got)
		}
	}
	sel := Selection{Start: Point{0, 0}, End: Point{2, 0}, Active: true}
	if got := SelectedText(s, sel); got != "a日" {
		t.Fatalf("got %q", got)
	}
}

func TestWordAtStopsAtSpaces(t *testing.T) {
	s := screen.New(20, 1)
	printAll(s, "git status here")
	sel := WordAt(s, Point{5, 0})
	if got := SelectedText(s, sel); got != "status" {
		t.Fatalf("got %q", got)
	}
}

func TestWordAtKeepsPathsWholeAndStopsAtQuotes(t *testing.T) {
	s := screen.New(40, 1)
	printAll(s, `cat "C:\Users\me\file.go:12" x`)
	sel := WordAt(s, Point{10, 0})
	if got := SelectedText(s, sel); got != `C:\Users\me\file.go:12` {
		t.Fatalf("got %q", got)
	}
}

func TestWordAtWideChar(t *testing.T) {
	s := screen.New(10, 1)
	printAll(s, "a 日本 b")
	sel := WordAt(s, Point{3, 0}) // continuation of 日
	if got := SelectedText(s, sel); got != "日本" {
		t.Fatalf("got %q", got)
	}
}

func TestLineAt(t *testing.T) {
	s := screen.New(8, 2)
	printAll(s, "hello")
	if got := SelectedText(s, LineAt(s, Point{2, 0})); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestSelectionIncludesScrollback(t *testing.T) {
	s := screen.NewWithScrollback(3, 2, 10)
	printAll(s, "aaabbbccc") // "aaa" scrolls off, screen is bbb / ccc
	if s.ScrollbackLen() != 1 {
		t.Fatalf("scrollback %d", s.ScrollbackLen())
	}
	sel := Selection{Start: Point{1, 0}, End: Point{1, 2}, Active: true}
	if got := SelectedText(s, sel); got != "aa\nbbb\ncc" {
		t.Fatalf("got %q", got)
	}
}
