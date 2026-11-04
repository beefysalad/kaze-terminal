package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in development evidence: the About page in every theme, wide and
// narrow, from the actual Gio frame at 2x. No native window or input.
func TestAboutRenders(t *testing.T) {
	dir := os.Getenv("KAZE_ABOUT_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_ABOUT_RENDER_DIR for headless visual evidence")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, th := range theme.All() {
		for _, dp := range []image.Point{image.Pt(900, 640), image.Pt(560, 640)} {
			win, _ := alertWindow(t, 1)
			win.openSettings()
			win.selectSection(slices.IndexFunc(win.sections(), func(s settingsSection) bool { return s.title == "About" }))
			win.upd.checked = true
			win.applyTheme(th)
			size := dp.Mul(2)
			g := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}, Constraints: layout.Exact(size)}
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			gpu, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Fatal(err)
			}
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rectangle{Max: size})
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			suffix := "wide"
			if dp.X < 600 {
				suffix = "narrow"
			}
			f, err := os.Create(filepath.Join(dir, th.Key+"-"+suffix+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, img)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
