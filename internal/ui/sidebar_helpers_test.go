package ui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// sideClickAt presses and releases at sidebar-local p, the press stamped at.
func sideClickAt(win *window, r *input.Router, p image.Point, b pointer.Buttons, at time.Duration) {
	w := sidePt(win, p)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: b, Position: w, Time: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: w, Time: at},
	)
}

// stepAgent starts agent id in tab with a current step "Grep auth".
func stepAgent(win *window, tab tabs.TabID, id string) subagents.Key {
	k := startAgent(win, tab, "s", id)
	win.subs.Apply(subagents.Event{Kind: subagents.EvTool, Tab: int(tab), Src: int(tab), Session: "s", AgentID: id, AgentType: "Explore", Tool: "Grep", Summary: "auth", At: time.Now()})
	return k
}

func sideMove(win *window, r *input.Router, kind pointer.Kind, p image.Point) {
	r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: sidePt(win, p)})
}

// gtx0Dp is n dp at the test pages' scale.
func gtx0Dp(n unit.Dp) int { return pageContext(sidebarW, 100).Dp(n) }

// renderImage draws ops in a headless GPU window of size and returns the pixels.
func renderImage(t *testing.T, ops *op.Ops, size image.Point) *image.RGBA {
	t.Helper()
	gpu, e := headless.NewWindow(size.X, size.Y)
	if e != nil {
		t.Fatal(e)
	}
	defer gpu.Release()
	if e = gpu.Frame(ops); e != nil {
		t.Fatal(e)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if e = gpu.Screenshot(img); e != nil {
		t.Fatal(e)
	}
	return img
}

// writeRender draws ops in a headless GPU window of size and saves it as a PNG at path.
func writeRender(t *testing.T, ops *op.Ops, size image.Point, path string) {
	t.Helper()
	img := renderImage(t, ops, size)
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
}
