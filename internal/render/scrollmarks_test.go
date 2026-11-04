package render

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
)

// heldLines makes a harness of 10 rows holding n lines in all (history plus
// the screen) and lays out one frame.
func heldLines(t *testing.T, n int) *harness {
	t.Helper()
	h := newHarness(t, 20, 10)
	h.out(strings.Repeat("x\r\n", n-1) + "x") // n lines
	h.waitFor(func() bool { return h.s.Screen().ScrollbackLen() == n-10 })
	h.frame()
	return h
}

// marksOn are marks with one single-row match on each line, the k-th current.
func marksOn(cur int32, lines ...int) *Marks {
	m := &Marks{Cur: cur, Gen: 1}
	for i, l := range lines {
		m.Spans = append(m.Spans, MatchSpan{Line: l, From: 0, To: 0, Match: int32(i)})
	}
	return m
}

func scrollGtx(ops *op.Ops) layout.Context {
	return layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(2000, 2000)), Now: time.Now()}
}

var track100 = image.Rect(500, 0, 506, 100)

func TestScrollMarksPositions(t *testing.T) {
	h := heldLines(t, 1000)
	h.r.SetMarks(marksOn(-1, 0, 500, 999))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	if got := h.r.markYs; len(got) != 3 || got[0] != 0 || got[1] != 50 || got[2] != 99 {
		t.Fatalf("mark rows = %v, want [0 50 99]", got)
	}
	if h.r.scrollPaint.cur != (image.Rectangle{}) {
		t.Fatalf("no current match, but one was drawn at %v", h.r.scrollPaint.cur)
	}
}

func TestScrollMarksDedupe(t *testing.T) {
	h := heldLines(t, 1000)
	lines := make([]int, 1000)
	for i := range lines {
		lines[i] = i
	}
	h.r.SetMarks(marksOn(-1, lines...))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	if n := len(h.r.markYs); n != 100 {
		t.Fatalf("%d marks on a 100 px track, want 100", n)
	}
}

func TestScrollMarksMatchAcrossWrapCountsOnce(t *testing.T) {
	h := heldLines(t, 1000)
	h.r.SetMarks(&Marks{Spans: []MatchSpan{{Line: 499, From: 18, To: 19}, {Line: 500, From: 0, To: 1}}, Cur: -1, Gen: 1})
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	if got := h.r.markYs; len(got) != 1 || got[0] != 49 {
		t.Fatalf("mark rows = %v, want [49], the match's start only", got)
	}
}

func TestScrollMarksCurrentDrawnLast(t *testing.T) {
	h := heldLines(t, 1000)
	h.r.SetMarks(marksOn(1, 0, 500, 999))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	want := image.Rect(500, 50, 506, 53) // 3 dp tall, 1 px per dp here
	if got := h.r.scrollPaint.cur; got != want {
		t.Fatalf("current mark at %v, want %v", got, want)
	}
	if !h.r.scrollPaint.curLast {
		t.Fatal("the current mark was not drawn last")
	}
}

func TestScrollMarksViewBand(t *testing.T) {
	h := heldLines(t, 100) // 90 history lines and 10 rows
	h.s.Lock()
	h.s.Screen().SetViewOffset(40) // top: line 50
	h.s.Unlock()
	h.r.SetMarks(marksOn(-1, 3))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	want := image.Rect(499, 50, 507, 60)
	if got := h.r.scrollPaint.band; got != want {
		t.Fatalf("view band %v, want %v", got, want)
	}
}

func TestScrollMarksNoBandWithoutHistory(t *testing.T) {
	h := newHarness(t, 20, 10)
	h.out("warn")
	h.frame()
	h.r.SetMarks(marksOn(0, 0))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	if h.r.scrollPaint.band != (image.Rectangle{}) {
		t.Fatalf("a band was drawn with nothing to scroll: %v", h.r.scrollPaint.band)
	}
	if len(h.r.markYs) != 1 || h.r.markYs[0] != 0 {
		t.Fatalf("mark rows = %v, want [0]", h.r.markYs)
	}
}

func TestScrollMarksNothingWithoutMarks(t *testing.T) {
	h := heldLines(t, 100)
	h.r.SetMarks(marksOn(0, 3))
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	added := h.r.marksAdded
	h.r.SetMarks(nil)
	h.frame()
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
	h.r.SetMarks(marksOn(0, 3))
	h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), image.Rectangle{})
	if h.r.marksAdded != added {
		t.Fatal("scroll marks were drawn with no marks or an empty track")
	}
}

func TestScrollMarksCached(t *testing.T) {
	h := heldLines(t, 1000)
	m := marksOn(0, 0, 500, 999)
	h.r.SetMarks(m)
	h.frame()
	ops := new(op.Ops)
	draw := func() {
		ops.Reset()
		h.r.DrawScrollMarks(scrollGtx(ops), track100)
	}
	draw()
	rec := h.r.marksRecorded
	draw()
	if h.r.marksRecorded != rec {
		t.Fatal("an unchanged frame recorded the scroll marks again")
	}
	if a := testing.AllocsPerRun(20, draw); a != 0 {
		t.Fatalf("an unchanged frame's scroll marks allocate %.0f, want 0", a)
	}
	m.Cur, m.Gen = 1, 2
	draw()
	if h.r.marksRecorded != rec+1 {
		t.Fatalf("a new current match recorded %d times, want once", h.r.marksRecorded-rec)
	}
	draw()
	if h.r.marksRecorded != rec+1 {
		t.Fatal("recorded again with nothing changed")
	}
}

// The view band is the mockup's 7 % white on a dark theme; on a light one
// (Washi) white would not show, so it is 7 % black.
func TestScrollMarksBandShowsOnLightTheme(t *testing.T) {
	for _, light := range []bool{false, true} {
		h := heldLines(t, 100)
		if light {
			p := DefaultPalette()
			p.FG, p.BG = hex(0x2b2a33), hex(0xf6f2ea)
			h.r = NewRenderer(14, p, Options{})
			h.frame()
		}
		h.r.SetMarks(marksOn(-1, 3))
		h.frame()
		h.r.DrawScrollMarks(scrollGtx(new(op.Ops)), track100)
		want := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x12}
		if light {
			want = color.NRGBA{A: 0x12}
		}
		if got := h.r.scrollPaint.bandColor; got != want {
			t.Fatalf("light %v: band colour %v, want %v", light, got, want)
		}
	}
}
