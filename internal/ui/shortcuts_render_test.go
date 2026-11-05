package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// TestShortcutsRenders writes headless captures of Settings > Shortcuts in
// Midnight and Washi: at rest (with a changed row and an unbound one),
// recording, a clash, a warning and a refused key. Set
// KAZE_SHORTCUTS_RENDER_DIR to run it.
func TestShortcutsRenders(t *testing.T) {
	dir := os.Getenv("KAZE_SHORTCUTS_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_SHORTCUTS_RENDER_DIR for headless captures of the Shortcuts page")
	}
	os.MkdirAll(dir, 0o700)
	const w, h = 1000, 900
	for _, name := range []string{"midnight", "washi"} {
		th, ok := theme.ByKey(name)
		if !ok {
			t.Fatalf("no theme %s", name)
		}
		for _, state := range []string{"default", "recording", "clash", "warning", "refused"} {
			win, r, _, _ := shortcutsWindow(t, "[keys]\nnext_waiting = \"ctrl+alt+j\"\nnew_tab = \"none\"\n")
			win.applyTheme(th)
			keysFrames(t, win, r)
			switch state {
			case "recording":
				clickKeys(t, win, r, "find")
			case "clash":
				clickKeys(t, win, r, "find")
				pressKeys(t, win, r, defaultOf("toggle_sidebar"))
			case "warning":
				clickKeys(t, win, r, "palette")
				pressIn(t, win, r, "R", key.ModCtrl)
			case "refused":
				clickKeys(t, win, r, "copy")
				pressKeys(t, win, r, input.Shortcut{Name: "K"})
			}
			if state != "default" {
				keysControl(t, win, r, "key_rec", win.set.keyMsg.action+win.set.keyRec) // keeps the row in view
				if b := win.set.keyStrip.box; !b.Empty() && b.Max.Y > win.set.pane.Max.Y {
					win.set.scroll += b.Max.Y - win.set.pane.Max.Y + 16
					keysFrames(t, win, r)
				}
			}
			g := pageContext(w, h)
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			gpu, err := headless.NewWindow(w, h)
			if err != nil {
				t.Fatal(err)
			}
			if err = gpu.Frame(g.Ops); err != nil {
				gpu.Release()
				t.Fatal(err)
			}
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			err = gpu.Screenshot(img)
			gpu.Release()
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "shortcuts-"+th.Key+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, img)
			f.Close()
		}
	}
}
