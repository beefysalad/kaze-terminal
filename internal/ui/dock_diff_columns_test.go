package ui

import (
	"image"
	"strings"
	"testing"
	"unicode/utf8"
)

// inkSpan is the first and last column of diff row i of step id that is not
// the pane's background, right of the gutter. The rows must be context rows.
func inkSpan(t *testing.T, win *window, img *image.RGBA, id string, i int) (first, last int) {
	t.Helper()
	var step image.Rectangle
	for _, st := range win.dock.hits.steps {
		if st.id == id {
			step = st.rect
		}
	}
	if step.Empty() {
		t.Fatalf("step %q not drawn", id)
	}
	lineH := gtxDp(win, readLineDp)
	top := step.Max.Y + gtxDp(win, 2) + i*lineH
	first, last = -1, -1
	for x := step.Min.X + gtxDp(win, 50); x < step.Max.X; x++ {
		for y := top + 1; y < top+lineH-1; y++ {
			if !nearColour(img.RGBAAt(x, y), win.pal.BG) {
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	if first < 0 {
		t.Fatalf("row %d has no ink", i)
	}
	return first, last
}

func columnShot(t *testing.T, lines ...string) (*window, *image.RGBA) {
	t.Helper()
	win, r, _, s, _ := rdDock(t, rdPatchEdit("e1", "/w/kaze/a.go", 1, lines...))
	s.scroll = 0
	twoFrames(t, win, r)
	return win, diffShot(t, win, r, 1000, 600)
}

func nearPx(a, b, within int) bool { return max(a, b)-min(a, b) <= within }

// "for" is a keyword, so its indent is a whitespace-only span; "fox" is plain,
// so its indent is part of a longer one.
func TestCodeStartsInOneColumnWhateverTheSpans(t *testing.T) {
	win, img := columnShot(t, " "+strings.Repeat(" ", 8)+"for i", " "+strings.Repeat(" ", 8)+"fox i")
	a, _ := inkSpan(t, win, img, "e1", 0)
	b, _ := inkSpan(t, win, img, "e1", 1)
	if !nearPx(a, b, 1) {
		t.Fatalf("same indent starts at x=%d and x=%d", a, b)
	}
}

func TestManySpansEndWhereOnePlainSpanEnds(t *testing.T) {
	code := `if a := b(c); d { return "e", 1, f(g), h(i), j(k) }`
	plain := "// " + strings.Repeat("x", utf8.RuneCountInString(code)-5) + " }"
	win, img := columnShot(t, " "+code, " "+plain)
	_, a := inkSpan(t, win, img, "e1", 0)
	_, b := inkSpan(t, win, img, "e1", 1)
	if !nearPx(a, b, 1) {
		t.Fatalf("many spans end at x=%d, one span at x=%d", a, b)
	}
}

func TestLongPlainLineReachesThePaneEdge(t *testing.T) {
	win, img := columnShot(t, " "+strings.Repeat("x", 190))
	_, last := inkSpan(t, win, img, "e1", 0)
	x, _ := diffRowAt(win, rdHit(t, win.dock.hits, "e1"), 0)
	if right, room := x+2, gtxDp(win, 10); last < right-room {
		t.Fatalf("a long line stops at x=%d, %d px short of the edge at %d", last, right-last, right)
	}
}

// Japanese is wider than its rune count says, so what follows it must still
// land after it rather than on top of it.
func TestWideRunesDoNotOverlapTheNextSpan(t *testing.T) {
	code := `var s = "日本語日本語日本語" + name`
	plain := "// " + strings.Repeat("x", 5) + `"日本語日本語日本語" + name`
	if utf8.RuneCountInString(code) != utf8.RuneCountInString(plain) {
		t.Fatalf("fixture: %d and %d runes", utf8.RuneCountInString(code), utf8.RuneCountInString(plain))
	}
	win, img := columnShot(t, " "+code, " "+plain)
	_, a := inkSpan(t, win, img, "e1", 0)
	_, b := inkSpan(t, win, img, "e1", 1)
	if !nearPx(a, b, 3) {
		t.Fatalf("after wide runes the code ends at x=%d, one span at x=%d", a, b)
	}
}
