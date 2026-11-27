package ui

import (
	"fmt"
	"gioui.org/gpu/headless"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestTourRenders(t *testing.T) {
	dir := os.Getenv("KAZE_TOUR_RENDER_DIR")
	if dir == "" {
		t.Skip("optional renders")
	}
	os.MkdirAll(dir, 0700)
	for _, theme := range []string{"midnight", "washi"} {
		for _, sz := range []image.Point{{900, 700}, {320, 240}} {
			w := sheetWindow(t, theme)
			w.hideSettings()
			for i := 0; i < 6; i++ {
				w.openTour(true)
				w.tourMove(i)
				g := pageContext(sz.X, sz.Y)
				w.drawTour(g)
				gpu, e := headless.NewWindow(sz.X, sz.Y)
				if e != nil {
					t.Fatal(e)
				}
				if e = gpu.Frame(g.Ops); e != nil {
					t.Fatal(e)
				}
				img := image.NewRGBA(image.Rectangle{Max: sz})
				e = gpu.Screenshot(img)
				gpu.Release()
				if e != nil {
					t.Fatal(e)
				}
				f, e := os.Create(fmt.Sprintf("%s/%s-%d-%d.png", dir, theme, sz.X, i))
				if e != nil {
					t.Fatal(e)
				}
				png.Encode(f, img)
				f.Close()
			}
		}
	}
}
