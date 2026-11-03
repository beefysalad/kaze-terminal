package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gioui.org/gpu/headless"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// Opt-in development evidence from the actual Gio frame. No native window,
// global input, shell process, network setup or owner configuration is used.
func TestEmptyWindowRenders(t *testing.T) {
	dir := os.Getenv("KAZE_EMPTY_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_EMPTY_RENDER_DIR for headless visual evidence")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"midnight", "washi"} {
		for _, size := range []image.Point{image.Pt(1000, 600), image.Pt(400, 340)} {
			win, _ := sideWindow(t, true)
			win.km, _ = buildKeymap(runtime.GOOS, nil)
			win.binds, _ = resolveBindings(runtime.GOOS, nil)
			th, _ := theme.ByKey(key)
			win.applyTheme(th)
			g := pageContext(size.X, size.Y)
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
			if size.X < 600 {
				suffix = "narrow"
			}
			f, err := os.Create(filepath.Join(dir, key+"-"+suffix+".png"))
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
