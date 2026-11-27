package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
)

// Render the Workspaces list with real Gio operations; no native window, no input.
func TestWSListRenders(t *testing.T) {
	dir := os.Getenv("KAZE_WSLIST_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_WSLIST_RENDER_DIR for PNG evidence")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, width := range []int{900, 620, 440} {
		for _, menu := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/menu=%v", width, menu), func(t *testing.T) {
				f := newWSPage(t)
				f.press("ws.toggle", 0)
				if menu {
					f.press("ws.menu", 0)
				}
				size := image.Pt(width, 900)
				g := pageContext(size.X, size.Y)
				f.win.drawSettings(g, image.Rect(0, 38, size.X, size.Y))
				gpu, e := headless.NewWindow(size.X, size.Y)
				if e != nil {
					t.Fatal(e)
				}
				defer gpu.Release()
				if e = gpu.Frame(g.Ops); e != nil {
					t.Fatal(e)
				}
				img := image.NewRGBA(image.Rectangle{Max: size})
				if e = gpu.Screenshot(img); e != nil {
					t.Fatal(e)
				}
				out, e := os.Create(filepath.Join(dir, fmt.Sprintf("ws-%d-menu%v.png", width, menu)))
				if e != nil {
					t.Fatal(e)
				}
				defer out.Close()
				if e = png.Encode(out, img); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
