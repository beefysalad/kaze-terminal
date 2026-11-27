package ui

import (
	"image"
	"testing"
)

func TestTourCardRectStaysInsideWindow(t *testing.T) {
	for _, sz := range []image.Point{{320, 240}, {480, 400}, {900, 700}} {
		w := image.Rectangle{Max: sz}
		r := tourCardRect(w, image.Rectangle{}, image.Pt(420, 300))
		if !r.In(w) {
			t.Fatal(r, w)
		}
	}
}
func TestTourReplayDoesNotMarkSeen(t *testing.T) {
	w := &window{}
	w.openTour(true)
	if !w.tour.open {
		t.Fatal("closed")
	}
	w.closeTour()
	if w.tour.open || !w.wantFocus {
		t.Fatal("focus")
	}
}
