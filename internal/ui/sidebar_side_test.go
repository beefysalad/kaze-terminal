package ui

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/io/input"
	"gioui.org/unit"
)

// nearColor reports whether the pixel at p is c, give or take rounding.
func nearColor(img *image.RGBA, p image.Point, c color.NRGBA) bool {
	got := img.RGBAAt(p.X, p.Y)
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	return d(got.R, c.R) <= 2 && d(got.G, c.G) <= 2 && d(got.B, c.B) <= 2
}

func TestRightSidebarRects(t *testing.T) {
	for _, c := range []struct {
		mode string
		w    int
	}{{"full", sidebarW}, {"rail", railW}} {
		win, _ := sideWindow(t, true)
		openIn(t, win, "")
		win.opt.Config.SidebarSide, win.opt.Config.SidebarMode = "right", c.mode
		var r input.Router
		sizedFrame(t, win, &r, 1200, 700)
		if win.side.at.X != 1200-c.w || win.gridMin.X != 8 || win.gridRect.Max.X != 1200-c.w-8 {
			t.Fatalf("%s right: sidebar x %d, grid %v", c.mode, win.side.at.X, win.gridRect)
		}
	}
}

func TestRightSidebarMirrorsDividerAndAccent(t *testing.T) {
	for _, right := range []bool{false, true} {
		size := image.Pt(sidebarW, 400)
		gtx := pageContext(size.X, size.Y)
		c := newChrome()
		h := c.layoutSidebar(gtx, withSpaces(twoGroupsOneLoose()), sideState{space: "a", right: right}, size)
		img := renderImage(t, gtx.Ops, size)
		th := c.sideTheme()
		border, inner := image.Pt(size.X-1, 200), image.Pt(0, 200)
		if right {
			border, inner = inner, border
		}
		if !nearColor(img, border, th.Border) || nearColor(img, inner, th.Border) {
			t.Fatalf("right %v: border at %v is %v, outer edge %v is %v", right, border, img.RGBAAt(border.X, border.Y), inner, img.RGBAAt(inner.X, inner.Y))
		}
		var row image.Rectangle
		for _, r := range h.spaces {
			if r.key == "a" {
				row = r.rect
			}
		}
		if row.Empty() {
			t.Fatalf("right %v: the selected space has no rect", right)
		}
		mid := (row.Min.Y + row.Max.Y) / 2
		on, off := image.Pt(row.Min.X, mid), image.Pt(row.Max.X-1, mid)
		if right {
			on, off = off, on
		}
		if !nearColor(img, on, th.Accent) || nearColor(img, off, th.Accent) {
			t.Fatalf("right %v: accent at %v is %v, other side %v is %v", right, on, img.RGBAAt(on.X, on.Y), off, img.RGBAAt(off.X, off.Y))
		}
	}
}

func TestRightSidebarClicksStillHit(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/alpha/x"))
	b := openIn(t, win, osPath("/code/beta/x"))
	win.mgr.SetActive(a.ID)
	win.opt.Config.SidebarSide = "right"
	var r input.Router
	sideFrame(t, win, &r)
	if len(win.side.hits.spaces) < 2 {
		t.Fatalf("%d spaces", len(win.side.hits.spaces))
	}
	hit := win.side.hits.spaces[1]
	if hit.key != "key-beta" {
		t.Fatalf("second space is %q, want key-beta", hit.key)
	}
	p := hit.rect.Min.Add(hit.rect.Max).Div(2).Add(win.side.at)
	clickAt(t, win, &r, p)
	if got := win.mgr.Active(); got == nil || got.ID != b.ID {
		t.Fatalf("a click at %v on the right sidebar did not show tab %d", p, b.ID)
	}
}

// At a fractional scale the mode follows the same rounded px thresholds the rest
// of the layout uses: a window exactly gtx.Dp(minSideWindow) wide shows full.
func TestSideWidthFractionalScale(t *testing.T) {
	// 1.1 is the review's case; at 1.333 a truncating dp conversion turns 746 px into 559 dp.
	for _, scale := range []float32{1.1, 1.333} {
		win, _ := sideWindow(t, true)
		gtx := pageContext(1000, 600)
		gtx.Metric = unit.Metric{PxPerDp: scale, PxPerSp: scale}
		w := gtx.Dp(minSideWindow)
		if got := win.sideWidth(gtx, w); win.side.mode != sideFull || got != gtx.Dp(sidebarW) {
			t.Fatalf("width %d at %v: mode %v, sidebar %d", w, scale, win.side.mode, got)
		}
		if win.sideWidth(gtx, w-1); win.side.mode != sideRail {
			t.Fatalf("width %d at %v: mode %v, want rail", w-1, scale, win.side.mode)
		}
		if win.sideWidth(gtx, gtx.Dp(minRailWindow)); win.side.mode != sideRail {
			t.Fatalf("rail threshold at %v: mode %v, want rail", scale, win.side.mode)
		}
		if win.sideWidth(gtx, gtx.Dp(minRailWindow)-1); win.side.mode != sideHidden {
			t.Fatalf("under the rail threshold at %v: mode %v, want hidden", scale, win.side.mode)
		}
	}
}

// The rail's divider faces the terminal too: x 0 on the right, its last column on the left.
func TestRailBorderOnInnerEdge(t *testing.T) {
	for _, right := range []bool{false, true} {
		win, _ := sideWindow(t, true)
		win.opt.Config.SidebarMode = "rail"
		if right {
			win.opt.Config.SidebarSide = "right"
		}
		size := image.Pt(railW, 400)
		gtx := pageContext(size.X, size.Y)
		win.side.mode = sideRail
		win.drawSide(gtx, image.Rectangle{Max: size})
		img := renderImage(t, gtx.Ops, size)
		border, outer := image.Pt(size.X-1, 200), image.Pt(0, 200)
		if right {
			border, outer = outer, border
		}
		if !nearColor(img, border, win.ui.th.Border) || nearColor(img, outer, win.ui.th.Border) {
			t.Fatalf("rail right %v: border at %v is %v, outer %v is %v", right, border, img.RGBAAt(border.X, border.Y), outer, img.RGBAAt(outer.X, outer.Y))
		}
	}
}
