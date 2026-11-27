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
