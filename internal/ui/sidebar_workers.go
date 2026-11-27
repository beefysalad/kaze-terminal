package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func (win *window) workerPresentation(id tabs.TabID) (agent, first string, needs bool) {
	if w := win.workers[id]; w != nil {
		return w.agent, w.firstMessage, w.needs
	}
	return
}

// workerMark draws a branching stem and two terminal panes, using paths only.
func (c *chrome) workerMark(gtx layout.Context, r image.Rectangle, col color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	pt := func(x, y float32) f32.Point {
		return f32.Pt(float32(r.Min.X)+x*float32(r.Dx())/16, float32(r.Min.Y)+y*float32(r.Dy())/16)
	}
	p.MoveTo(pt(3, 2))
	p.LineTo(pt(3, 12))
	p.LineTo(pt(7, 12))
	p.MoveTo(pt(3, 5))
	p.LineTo(pt(7, 5))
	for _, y := range []float32{2, 9} {
		p.MoveTo(pt(7, y))
		p.LineTo(pt(14, y))
		p.LineTo(pt(14, y+5))
		p.LineTo(pt(7, y+5))
		p.Close()
	}
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: float32(max(gtx.Dp(1), 1))}.Op())
}

// workerBranch joins adjacent worker rows; the last sibling ends at its elbow.
func (c *chrome) workerBranch(gtx layout.Context, x, top, bottom, mid, end int) {
	col := c.sideTheme().Border
	paint.FillShape(gtx.Ops, col, clip.Rect(image.Rect(x, top, x+1, bottom+1)).Op())
	paint.FillShape(gtx.Ops, col, clip.Rect(image.Rect(x, mid, end, mid+1)).Op())
}

// fitWorkerName keeps worker identifiers in the terminal face.
func (c *chrome) fitWorkerName(g layout.Context, s string, z unit.Sp, room int, col color.NRGBA) (op.CallOp, image.Point, string) {
	s = cutRunes(c.fit(s), maxSideText)
	r := []rune(s)
	for len(r) > 0 {
		shown := string(r)
		if len(r) < len([]rune(s)) {
			shown += "…"
		}
		call, sz := c.labelMonoW(g, shown, z, col, font.Bold)
		if sz.X <= room {
			return call, sz, shown
		}
		r = r[:len(r)-1]
	}
	call, sz := c.labelMono(g, "", z, col)
	return call, sz, ""
}
