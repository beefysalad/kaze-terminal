package ui

import (
	"testing"
)

func TestMarkIsScaledOnceAndHasTheViolet(t *testing.T) {
	op1, img := markImage(18)
	if img == nil {
		t.Fatal("no mark image")
	}
	if b := img.Bounds(); b.Dx() != 18 || b.Dy() != 18 {
		t.Fatalf("mark is %v, want 18x18", b)
	}
	violet := false
	for y := 0; y < 18 && !violet; y++ {
		for x := 0; x < 18; x++ {
			c := img.NRGBAAt(x, y)
			if c.A > 200 && c.B > 200 && c.R > 100 && c.G < c.B {
				violet = true
				break
			}
		}
	}
	if !violet {
		t.Fatal("scaled mark has no opaque violet pixel")
	}
	op2, img2 := markImage(18)
	if img2 != img || op1 != op2 {
		t.Fatal("same size was decoded and scaled again")
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
