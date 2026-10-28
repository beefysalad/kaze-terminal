package render

import (
	"image"
	"strings"
	"testing"
)

func TestReleaseRebuildsOnNextLayout(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	h.r.Release()
	if h.r.strips != nil || h.r.drawn != nil {
		t.Fatal("Release kept the strip images or drawn state")
	}
	if n := h.frame(); n != 24 {
		t.Fatalf("frame after Release redrew %d rows, want 24", n)
	}
	if len(h.r.strips) != stripCount {
		t.Fatalf("strips = %d after rebuild", len(h.r.strips))
	}
}

func TestViewOffsetChangeRedraws(t *testing.T) {
	h := newHarness(t, 20, 4)
	h.out(strings.Repeat("line\r\n", 10)) // history to scroll into
	h.frame()
	h.s.Lock()
	h.s.Screen().SetViewOffset(2)
	h.s.Screen().TakeDirty() // the renderer must not depend on the screen marking rows dirty
	h.s.Unlock()
	h.frame()
	h.s.Lock()
	h.s.Screen().SetViewOffset(0)
	h.s.Screen().TakeDirty()
	h.s.Unlock()
	if n := h.frame(); n == 0 {
		t.Fatal("scrolling back to the bottom redrew nothing")
	}
}

func TestCursorImagesAreReused(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	seen := map[*image.RGBA]bool{}
	for i := 0; i < 6; i++ {
		h.out("x")
		h.frame()
		seen[h.r.curImg] = true
	}
	if len(seen) > 2 {
		t.Fatalf("cursor drew into %d different images over 6 moves, want at most 2", len(seen))
	}
}

// A tab shown mid synchronized update must still draw: holding with nothing
// drawn yet would paint only the background for up to the whole hold.
func TestSyncHoldDoesNotHoldAFreshRenderer(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	h.out("\x1b[?2026hx")
	h.r.Release() // the tab is switched away from and back to
	if n := h.frame(); n != 24 {
		t.Fatalf("first frame after Release during sync redrew %d rows, want 24", n)
	}
	if n := h.frame(); n != 0 {
		t.Fatalf("next frame during sync redrew %d rows, want 0 (held)", n)
	}
}

func TestSyncHoldDoesNotHoldTheFirstFrame(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.out("\x1b[?2026hx")
	if n := h.frame(); n != 24 {
		t.Fatalf("first frame during sync redrew %d rows, want 24", n)
	}
}
