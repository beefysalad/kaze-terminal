package render

import (
	"image/color"
	"testing"
	"time"
)

// waitFor polls cond under the session lock for at most two seconds.
func (h *harness) waitFor(cond func() bool) {
	h.t.Helper()
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		h.s.Lock()
		ok := cond()
		h.s.Unlock()
		if ok {
			return
		}
	}
	h.t.Fatal("output did not land in time")
}

func rgba(c color.NRGBA) color.RGBA { return color.RGBA{c.R, c.G, c.B, 0xff} }

// bgAt is the top-left pixel of a cell, which no glyph reaches: its background.
func (h *harness) bgAt(col, row int) color.RGBA {
	w, ch := h.r.CellSize()
	return h.r.pixelAt(col*w, row*ch)
}

// cellHasPixel reports whether any pixel of the cell satisfies ok.
func (h *harness) cellHasPixel(col, row int, ok func(color.RGBA) bool) bool {
	w, ch := h.r.CellSize()
	for y := row * ch; y < (row+1)*ch; y++ {
		for x := col * w; x < (col+1)*w; x++ {
			if ok(h.r.pixelAt(x, y)) {
				return true
			}
		}
	}
	return false
}

func TestMatchCellsTinted(t *testing.T) {
	h := newHarness(t, 30, 4)
	h.out("find the warn here")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 9, To: 12}}, Cur: -1, Gen: 1})
	h.frame()
	p := h.r.pal
	for c := 9; c <= 12; c++ {
		if got := h.bgAt(c, 0); got != rgba(p.Match) {
			t.Fatalf("col %d = %v, want the match tint %v", c, got, p.Match)
		}
	}
	for _, c := range []int{8, 13, 0} {
		if got := h.bgAt(c, 0); got != rgba(p.BG) {
			t.Fatalf("col %d = %v, want the background", c, got)
		}
	}
}

func TestCurrentMatchStronger(t *testing.T) {
	h := newHarness(t, 30, 4)
	h.out("warn one warn two")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 0, To: 3, Match: 0}, {Line: 0, From: 9, To: 12, Match: 1}}, Cur: 1, Gen: 1})
	h.frame()
	p := h.r.pal
	if got := h.bgAt(0, 0); got != rgba(p.Match) {
		t.Fatalf("first match = %v, want the tint", got)
	}
	if got := h.bgAt(9, 0); got != rgba(p.MatchCur) {
		t.Fatalf("current match = %v, want %v", got, p.MatchCur)
	}
	bgLum := luminance(p.MatchCur)
	if !h.cellHasPixel(9, 0, func(c color.RGBA) bool {
		return luminance(color.NRGBA{c.R, c.G, c.B, 0xff}) < bgLum/3
	}) {
		t.Fatal("the current match's text is not dark ink on amber")
	}
}

func TestSelectionWinsOverMatch(t *testing.T) {
	h := newHarness(t, 30, 4)
	h.out("find the warn here")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 9, To: 12}}, Cur: 0, Gen: 1})
	h.frameSel(&Selection{Start: Point{10, 0}, End: Point{11, 0}, Active: true})
	p := h.r.pal
	if got := h.bgAt(10, 0); got != rgba(p.Selection) {
		t.Fatalf("selected match cell = %v, want the selection", got)
	}
	if got := h.bgAt(9, 0); got != rgba(p.MatchCur) {
		t.Fatalf("unselected match cell = %v, want the current match", got)
	}
}

func TestWideMatchCoversBothHalves(t *testing.T) {
	h := newHarness(t, 10, 2)
	h.out("日x")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 0, To: 0}}, Cur: -1, Gen: 1})
	h.frame()
	p := h.r.pal
	w, ch := h.r.CellSize()
	if got := h.r.pixelAt(w+w/2, ch-1); got != rgba(p.Match) {
		t.Fatalf("right half = %v, want the tint", got)
	}
	if got := h.bgAt(2, 0); got != rgba(p.BG) {
		t.Fatalf("the cell after = %v, want the background", got)
	}
}

func TestMatchAcrossWrap(t *testing.T) {
	h := newHarness(t, 10, 3)
	h.out("0123456789abc")
	h.waitFor(func() bool { return h.s.Screen().Cell(2, 1).Rune == 'c' })
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 8, To: 9}, {Line: 1, From: 0, To: 1}}, Cur: -1, Gen: 1})
	h.frame()
	p := h.r.pal
	for _, at := range [][2]int{{8, 0}, {9, 0}, {0, 1}, {1, 1}} {
		if got := h.bgAt(at[0], at[1]); got != rgba(p.Match) {
			t.Fatalf("%v = %v, want the tint", at, got)
		}
	}
	for _, at := range [][2]int{{7, 0}, {2, 1}, {0, 2}} {
		if got := h.bgAt(at[0], at[1]); got != rgba(p.BG) {
			t.Fatalf("%v = %v, want the background", at, got)
		}
	}
}

func TestMarksFollowScrolling(t *testing.T) {
	h := newHarness(t, 10, 3)
	h.out("l0\r\nl1\r\nl2\r\nl3\r\nl4\r\nl5")
	h.waitFor(func() bool { return h.s.Screen().ScrollbackLen() == 3 })                  // out returns on the first change
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 1, From: 0, To: 1}}, Cur: -1, Gen: 1}) // "l1", in history
	h.frame()
	p := h.r.pal
	for row := 0; row < 3; row++ {
		if got := h.bgAt(0, row); got != rgba(p.BG) {
			t.Fatalf("live view row %d = %v, want no tint (l1 is in history)", row, got)
		}
	}
	scroll := func(off int) {
		h.s.Lock()
		h.s.Screen().SetViewOffset(off)
		h.s.Unlock()
		h.frame()
	}
	scroll(3) // rows: l0 l1 l2
	if got := h.bgAt(0, 1); got != rgba(p.Match) {
		t.Fatalf("offset 3: l1 on row 1 = %v, want the tint", got)
	}
	scroll(2) // rows: l1 l2 l3
	if got, was := h.bgAt(0, 0), h.bgAt(0, 1); got != rgba(p.Match) || was != rgba(p.BG) {
		t.Fatalf("offset 2: row 0 = %v (want tint), row 1 = %v (want background)", got, was)
	}
}

func TestMarksFollowLiveOutput(t *testing.T) {
	h := newHarness(t, 10, 3)
	h.out("aa\r\nwarn\r\ncc")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 1, From: 0, To: 3}}, Cur: -1, Gen: 1})
	h.frame()
	p := h.r.pal
	if got := h.bgAt(0, 1); got != rgba(p.Match) {
		t.Fatalf("row 1 = %v, want the tint", got)
	}
	h.out("\r\ndd") // scrolls one line: warn moves to row 0
	h.frame()
	if got, was := h.bgAt(0, 0), h.bgAt(0, 1); got != rgba(p.Match) || was != rgba(p.BG) {
		t.Fatalf("after a scroll: row 0 = %v (want tint), row 1 = %v (want background)", got, was)
	}
}

func TestMarksSurviveRowRedraw(t *testing.T) {
	h := newHarness(t, 20, 3)
	h.out("warn")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 0, To: 3}}, Cur: -1, Gen: 1})
	h.frame()
	h.out(" more") // the same row changes, the marks do not
	h.waitFor(func() bool { return h.s.Screen().Cell(8, 0).Rune == 'e' })
	h.frame()
	if got := h.bgAt(1, 0); got != rgba(h.r.pal.Match) {
		t.Fatalf("after the row redrew, the match = %v, want the tint", got)
	}
}

func TestMarksRedrawOnlyChangedCells(t *testing.T) {
	h := newHarness(t, 20, 6)
	h.out("warn\r\nx\r\nwarn")
	h.frame()
	m := &Marks{Spans: []MatchSpan{{Line: 0, From: 0, To: 3, Match: 0}, {Line: 2, From: 0, To: 3, Match: 1}}, Cur: 0, Gen: 1}
	h.r.SetMarks(m)
	if n := h.frame(); n != 2 {
		t.Fatalf("setting marks redrew %d rows, want 2", n)
	}
	if n := h.frame(); n != 0 {
		t.Fatalf("unchanged marks redrew %d rows, want 0", n)
	}
	m.Cur, m.Gen = 1, 2
	if n := h.frame(); n != 2 {
		t.Fatalf("moving the current match redrew %d rows, want 2", n)
	}
}

func TestClearingMarksRestoresCells(t *testing.T) {
	h := newHarness(t, 20, 3)
	h.out("warn")
	h.frame()
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 0, From: 0, To: 3}}, Cur: 0, Gen: 1})
	h.frame()
	h.r.SetMarks(nil)
	if n := h.frame(); n != 1 {
		t.Fatalf("clearing marks redrew %d rows, want 1", n)
	}
	if got := h.bgAt(0, 0); got != rgba(h.r.pal.BG) {
		t.Fatalf("cleared match = %v, want the background", got)
	}
	if n := h.frame(); n != 0 {
		t.Fatalf("the frame after clearing redrew %d rows, want 0", n)
	}
}

func TestMarksStillFrameAllocatesNothing(t *testing.T) {
	build := func(marks bool) *harness {
		h := newHarness(t, 80, 24)
		for i := 0; i < 23; i++ {
			h.out("warn and warn and more warn here\r\n")
		}
		if marks {
			m := &Marks{Cur: 3, Gen: 1}
			for i := 0; i < 50; i++ {
				m.Spans = append(m.Spans, MatchSpan{Line: i / 3, From: (i % 3) * 9, To: (i%3)*9 + 3, Match: int32(i)})
			}
			h.r.SetMarks(m)
		}
		h.frame()
		h.frame()
		return h
	}
	plain, marked := build(false), build(true)
	a := testing.AllocsPerRun(20, func() { plain.frame() })
	b := testing.AllocsPerRun(20, func() { marked.frame() })
	if b != a {
		t.Fatalf("a still frame with marks allocates %.0f, without %.0f", b, a)
	}
	if n := marked.frame(); n != 0 {
		t.Fatalf("a still frame with marks redrew %d rows", n)
	}
}

func TestNoMarksNoExtraRows(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.frame()
	h.r.SetMarks(nil)
	if n := h.frame(); n != 0 {
		t.Fatalf("idle frame with nil marks redrew %d rows, want 0", n)
	}
}
