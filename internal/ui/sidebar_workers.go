package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

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
