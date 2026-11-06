package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in development evidence: the title bar in every theme, Windows and macOS
// layouts, sidebar shown and hidden, at 2x. No native window or input.
func TestTitleBarRenders(t *testing.T) {
	dir := os.Getenv("KAZE_TITLE_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_TITLE_RENDER_DIR for headless captures of the title bar")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, th := range theme.All() {
		for _, custom := range []bool{true, false} {
			for _, side := range []int{sidebarW, 0} {
				win, _ := sideWindow(t, custom)
				openIn(t, win, "")
				win.applyTheme(th)
				win.side.w = side * 2
				size := image.Pt(1100*2, 42*2)
				g := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}, Constraints: layout.Exact(size)}
				win.drawBar(g, size.Y)
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
				name := th.Key + "-windows"
				if !custom {
					name = th.Key + "-macos"
				}
				if side == 0 {
					name += "-nosidebar"
				}
				f, err := os.Create(filepath.Join(dir, name+".png"))
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
}
