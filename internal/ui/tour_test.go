package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
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
func TestTourBoardStepOpensAndRestoresBoard(t *testing.T) {
	w := sheetWindow(t, "midnight")
	w.hideSettings()
	w.openTour(true)
	w.tourMove(4)
	if !w.board.open || !w.tour.boardOpenedByTour {
		t.Fatal("board not opened")
	}
	w.tourBack()
	if w.board.open {
		t.Fatal("board not restored")
	}
	w.tourMove(4)
	w.closeTour()
	if w.board.open {
		t.Fatal("board not closed")
	}
}
func TestTourOpenMarksSeen(t *testing.T) {
	w := &window{cfgPath: filepath.Join(t.TempDir(), "config.toml")}
	w.openTour(false)
	b, e := os.ReadFile(filepath.Join(w.tourDir(), "tour.json"))
	if e != nil || !strings.Contains(string(b), "true") {
		t.Fatal(e, string(b))
	}
	w.closeTour()
	if w.focusTarget() == &w.tour.tag {
		t.Fatal("tour kept focus")
	}
}
