package ui

import (
	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"testing"
)

func TestTourWheelRouter(t *testing.T) {
	w := sheetWindow(t, "midnight")
	w.hideSettings()
	w.openTour(true)
	w.tourMove(5)
	var router input.Router
	frame := func() {
		g := pageContext(320, 240)
		g.Source = router.Source()
		w.tourEvents(g)
		w.drawTour(g)
		router.Frame(g.Ops)
	}
	frame()
	frame()
	router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(150, 120), Scroll: f32.Pt(0, 60)})
	frame()
	if w.tour.scroll == 0 {
		t.Fatal("wheel did not scroll overflowing body")
	}
}

func TestTourHookHitsExcludeFooter(t *testing.T) {
	w := sheetWindow(t, "midnight")
	w.hideSettings()
	w.openTour(true)
	w.tourMove(5)
	w.tour.hooks = []tourHook{{"Claude Code", hookNeedsFix}, {"Codex", hookNeedsFix}}
	for _, scroll := range []int{0, 20, 40} {
		w.tour.scroll = scroll
		w.drawTour(pageContext(320, 240))
		for _, a := range w.tour.hits {
			for _, b := range w.tour.hits {
				if a.action > 0 && b.action <= 0 && !a.r.Intersect(b.r).Empty() {
					t.Fatalf("hook overlaps footer: %v %v", a, b)
				}
			}
		}
	}
}

func TestTourBoardPreservesDocumentSelection(t *testing.T) {
	w := sheetWindow(t, "midnight")
	w.hideSettings()
	w.docs.shown = docID(42)
	w.openTour(true)
	w.tourMove(4)
	w.tourBack()
	if w.docs.shown != docID(42) {
		t.Fatal("tour lost document selection")
	}
}
