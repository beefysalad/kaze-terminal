package ui

import (
	"image"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func ids(n int) []tabs.TabID {
	out := make([]tabs.TabID, n)
	for i := range out {
		out[i] = tabs.TabID(i + 1)
	}
	return out
}

var m1 = newStripMetrics(1) // 1 px per dp keeps the numbers readable

func TestStripFewTabsFillAtMax(t *testing.T) {
	l := layoutTabStrip(m1, 1000, ids(2), 1, 0)
	if len(l.Tabs) != 2 {
		t.Fatalf("tabs=%d", len(l.Tabs))
	}
	if l.Tabs[0].Rect.Dx() != 230 || l.Tabs[0].Rect.Dy() != 36 {
		t.Fatalf("rect %v", l.Tabs[0].Rect)
	}
	if l.Tabs[1].Rect.Min.X != 230 {
		t.Fatalf("second tab x=%d", l.Tabs[1].Rect.Min.X)
	}
	if l.Plus.Min.X != 460 || l.Plus.Dx() != 28 {
		t.Fatalf("plus follows the tabs: %v", l.Plus)
	}
	if l.Scrolls() || l.Scroll != 0 {
		t.Fatal("must not scroll")
	}
}

func TestStripShrinksBeforeScrolling(t *testing.T) {
	// 4 tabs in 600-28=572 px -> 143 each, between min and max.
	l := layoutTabStrip(m1, 600, ids(4), 1, 0)
	if w := l.Tabs[0].Rect.Dx(); w != 143 {
		t.Fatalf("w=%d", w)
	}
	if l.Scrolls() {
		t.Fatal("should fit")
	}
	if l.Plus.Max.X != 600 || l.Plus.Min.X != 572 {
		t.Fatalf("plus %v", l.Plus)
	}
}

func TestStripOverflowScrollsAndPinsPlus(t *testing.T) {
	l := layoutTabStrip(m1, 500, ids(10), 1, 0)
	if !l.Scrolls() {
		t.Fatal("should scroll")
	}
	if l.Tabs[0].Rect.Dx() != 120 {
		t.Fatalf("min width, got %d", l.Tabs[0].Rect.Dx())
	}
	if l.Plus.Max.X != 500 {
		t.Fatalf("plus pinned right: %v", l.Plus)
	}
	if l.MaxScroll != 10*120-(500-28) {
		t.Fatalf("max scroll %d", l.MaxScroll)
	}
}

func TestStripScrollClamped(t *testing.T) {
	if l := layoutTabStrip(m1, 500, ids(10), 99, 99999); l.Scroll != l.MaxScroll {
		t.Fatalf("hi %d/%d", l.Scroll, l.MaxScroll)
	}
	l := layoutTabStrip(m1, 500, ids(10), 99, -50)
	if l.Scroll != 0 {
		t.Fatalf("lo %d", l.Scroll)
	}
}

func TestStripRevealsActive(t *testing.T) {
	for _, active := range []tabs.TabID{1, 5, 10} {
		for _, sc := range []int{0, 300, 99999} {
			l := layoutTabStrip(m1, 500, ids(10), active, sc)
			r := l.Tabs[int(active)-1].Rect
			if r.Min.X < 0 || r.Max.X > l.ViewW {
				t.Fatalf("active %d scroll %d not visible: %v view %d", active, sc, r, l.ViewW)
			}
		}
	}
}

func TestStripNoRevealWhenActiveUnknown(t *testing.T) {
	l := layoutTabStrip(m1, 500, ids(10), 99, 200)
	if l.Scroll != 200 {
		t.Fatalf("scroll=%d", l.Scroll)
	}
}

func TestStripHitTests(t *testing.T) {
	l := layoutTabStrip(m1, 1000, ids(2), 1, 0)
	// close x is 18 wide, inside the tab's right edge, centred vertically.
	c := l.Tabs[0].Close
	if c.Dx() != 18 || c.Dy() != 18 || !c.In(l.Tabs[0].Rect) {
		t.Fatalf("close %v", c)
	}
	if h := l.Hit(c.Min.Add(image.Pt(2, 2))); h.Kind != hitClose || h.Index != 0 {
		t.Fatalf("close hit %+v", h)
	}
	if h := l.Hit(image.Pt(10, 10)); h.Kind != hitTab || h.Index != 0 {
		t.Fatalf("tab hit %+v", h)
	}
	if h := l.Hit(image.Pt(235, 10)); h.Kind != hitTab || h.Index != 1 {
		t.Fatalf("tab1 hit %+v", h)
	}
	if h := l.Hit(l.Plus.Min.Add(image.Pt(3, 3))); h.Kind != hitPlus {
		t.Fatalf("plus hit %+v", h)
	}
	if h := l.Hit(image.Pt(900, 10)); h.Kind != hitNone {
		t.Fatalf("empty hit %+v", h)
	}
	if h := l.Hit(image.Pt(10, 40)); h.Kind != hitNone {
		t.Fatalf("below strip %+v", h)
	}
	if h := l.Hit(image.Pt(-1, 10)); h.Kind != hitNone {
		t.Fatalf("left of strip %+v", h)
	}
}

func TestStripHitIgnoresTabsUnderPlus(t *testing.T) {
	l := layoutTabStrip(m1, 500, ids(10), 1, 0)
	// A point over the pinned plus must hit plus, never a tab scrolled beneath it.
	if h := l.Hit(image.Pt(l.Plus.Min.X+1, 10)); h.Kind != hitPlus {
		t.Fatalf("%+v", h)
	}
	// A tab cut off by the view edge is hit only within the view.
	if h := l.Hit(image.Pt(l.ViewW-1, 10)); h.Kind == hitNone {
		t.Fatalf("visible edge should hit a tab")
	}
}

func TestStripScaleAndBounds(t *testing.T) {
	m := newStripMetrics(2)
	l := layoutTabStrip(m, 2000, ids(1), 1, 0)
	if l.Tabs[0].Rect.Dy() != 72 || l.Tabs[0].Rect.Dx() != 460 {
		t.Fatalf("scaled %v", l.Tabs[0].Rect)
	}
	// Degenerate inputs stay bounded and do not panic.
	for _, w := range []int{-5, 0, 1, 27, 28, 29} {
		l := layoutTabStrip(m1, w, ids(3), 2, 100)
		if l.Scroll < 0 || l.Scroll > l.MaxScroll {
			t.Fatalf("w=%d scroll %d max %d", w, l.Scroll, l.MaxScroll)
		}
	}
	if l := layoutTabStrip(m1, 800, nil, 0, 0); len(l.Tabs) != 0 || l.Plus.Empty() {
		t.Fatalf("empty: %+v", l)
	}
	if l := layoutTabStrip(m1, 800, ids(5000), 4000, 0); len(l.Tabs) != 5000 {
		t.Fatal("many tabs")
	}
}
