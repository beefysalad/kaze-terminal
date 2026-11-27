package ui

import (
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"image"
	"image/color"
)

func (w *window) drawTour(g layout.Context) {
	if !w.tour.open {
		return
	}
	ops, c := g.Ops, w.ui
	th := c.th
	bounds := image.Rectangle{Max: g.Constraints.Max}
	paint.FillShape(ops, color.NRGBA{A: 150}, clip.Rect(bounds).Op())
	s := tourSteps()[w.tour.step]
	r := tourCardRect(bounds, image.Rectangle{}, image.Pt(g.Dp(440), g.Dp(370)))
	paint.FillShape(ops, th.ToastBG, clip.UniformRRect(r, g.Dp(12)).Op(ops))
	stroke(ops, r, g.Dp(12), th.ToastLine)
	cl := clip.Rect(r).Push(ops)
	x, y := r.Min.X+g.Dp(18), r.Min.Y+g.Dp(16)
	put := func(text string, z unit.Sp, col color.NRGBA) {
		call, sz := c.label(g, text, z, col)
		place(ops, call, image.Pt(x, y))
		y += sz.Y + g.Dp(7)
	}
	put(s.Label, 11, th.Muted)
	call, sz := c.labelW(g, s.Title, 20, th.ToastTitle, font.Bold)
	place(ops, call, image.Pt(x, y))
	y += sz.Y + g.Dp(12)
	// Reuse the settings word wrapper, including bounded splitting of long words.
	p := &page{gtx: g, c: c, st: &w.set}
	for _, line := range p.wrap(s.Body, 13, max(r.Dx()-g.Dp(36), 1)) {
		put(line, 13, th.ToastFG)
	}
	w.tour.hits = w.tour.hits[:0]
	button := func(text string, action int, at image.Point) image.Point {
		call, sz := c.label(g, text, 12, th.ToastGo)
		place(ops, call, at)
		rr := image.Rectangle{Min: at.Sub(image.Pt(4, 4)), Max: at.Add(sz).Add(image.Pt(4, 4))}
		w.tour.hits = append(w.tour.hits, tourHit{rr, action})
		return sz
	}
	if s.Hooks {
		for i, h := range w.tour.hooks {
			status := "not found"
			if h.State == hookReady {
				status = "ready"
			}
			if h.State == hookNeedsFix {
				status = "needs setup"
			}
			put(h.Agent+": "+status, 12, th.ToastFG)
			if h.State == hookNeedsFix {
				button("Fix", i+1, image.Pt(r.Max.X-g.Dp(52), y-g.Dp(24)))
			}
		}
	}
	y = r.Max.Y - g.Dp(36)
	button(s.SkipLabel, -1, image.Pt(x, y))
	if w.tour.step > 0 {
		button("Back", -2, image.Pt(x+g.Dp(125), y))
	}
	_, ns := c.label(g, s.NextLabel, 12, th.ToastGo)
	button(s.NextLabel, 0, image.Pt(r.Max.X-g.Dp(18)-ns.X, y))
	cl.Pop()
	area := clip.Rect(bounds).Push(ops)
	event.Op(ops, &w.tour.tag)
	area.Pop()
}

var _ op.CallOp
