package ui

import (
	"image"
	"testing"
)

// The settings sidebar footer draws the approved K from the lockup's vector
// mark, 22dp tall, just before "kaze <version>".
func TestSettingsFooterDrawsTheVectorMark(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.openSettings()
	win.drawSettings(pageContext(800, 600), image.Rect(0, 38, 800, 576))
	m := win.set.footMark
	if m.Empty() || m.Dy() != 22 {
		t.Fatalf("footer mark %v, want a 22-tall box", m)
	}
	if m.Max.Y > 576 || m.Min.Y < 38+576/2 {
		t.Fatalf("footer mark %v is not at the bottom of the sidebar", m)
	}
}

// The lockup is the approved SVG's mark (three shapes) and wordmark (four
// letters), read once into the SVG's own coordinates.
func TestLockupShapesMatchTheSVG(t *testing.T) {
	lk := lockupShapes()
	if len(lk.mark) != 3 || len(lk.word) != 4 {
		t.Fatalf("mark %d shapes, word %d, want 3 and 4", len(lk.mark), len(lk.word))
	}
	mb, wb := lk.markBounds, lk.wordBounds
	if !nearF(mb.Min.X, 24) || !nearF(mb.Min.Y, 28) || !nearF(mb.Max.X, 128) || !nearF(mb.Max.Y, 140) {
		t.Errorf("mark bounds %v, want about (24,28)-(128,140)", mb)
	}
	// The k's stem starts 139 font units in at scale 0.056; the e ends near 397.
	if !nearF(wb.Min.X, 168+139*0.056) || wb.Max.X < 394 || wb.Max.X > 400 {
		t.Errorf("word x %v..%v", wb.Min.X, wb.Max.X)
	}
	if wb.Max.Y < 118 || wb.Max.Y > 122 {
		t.Errorf("word bottom %v, want near the 119 baseline", wb.Max.Y)
	}
	if lockupShapes().mark[0].segs[0] != lk.mark[0].segs[0] {
		t.Error("parsed twice with different results")
	}
}

func nearF(a, b float32) bool { return a-b < 1 && b-a < 1 }
