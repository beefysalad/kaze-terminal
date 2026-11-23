package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Opt-in development evidence: the whole window from the actual Gio frame at 2x
// (1120 x 640 dp, the mockup's size): the sidebar's spaces and agents, the tab
// strip, the terminal. Spaces alpha (two tabs) and beta, a Shell tab, a working,
// a waiting and an idle agent. No native window, no input.
// Run with KAZE_SPACES_RENDER_DIR=<dir>.
func TestSpacesRender(t *testing.T) {
	dir := os.Getenv("KAZE_SPACES_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_SPACES_RENDER_DIR for headless visual evidence")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, th := range []theme.Theme{theme.Default(), mustTheme(t, "washi")} {
		win, _, _, a1, a2, b1 := spacesFixture(t)
		s1 := openIn(t, win, "")
		win.applyTheme(th)
		win.detected = map[tabs.TabID]detection{a1.ID: {agent: "claude"}, a2.ID: {agent: "codex"}, b1.ID: {agent: "claude"}}
		setAgent(win, a1.ID, agentState{claude: true})
		setAgent(win, b1.ID, agentState{claude: true, running: true, since: t0.Add(-130 * time.Second), command: "/ship-ticket"})
		win.alerts.apply(notify.Msg{Tab: int(a2.ID), Kind: notify.KindNeeds}, a1.ID, true, only(3), t0)
		win.mgr.SetActive(a1.ID)
		_ = s1
		var r input.Router
		size := image.Pt(2240, 1280)
		for range 3 {
			g := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}, Constraints: layout.Exact(size), Source: r.Source(), Now: t0}
			if err := win.frame(g); err != nil {
				t.Fatal(err)
			}
			r.Frame(g.Ops)
			if _, err := writeShot(t, g.Ops, size, filepath.Join(dir, "window-"+th.Key+".png")); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func writeShot(t *testing.T, ops *op.Ops, size image.Point, path string) (*image.RGBA, error) {
	t.Helper()
	gpu, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU here: %v", err)
	}
	defer gpu.Release()
	if err = gpu.Frame(ops); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err = gpu.Screenshot(img); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return img, png.Encode(f, img)
}
