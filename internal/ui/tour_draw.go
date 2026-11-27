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
	target := w.tour.targets[s.Target]
	if s.Target == tourStatusBar {
		target = image.Rect(0, w.stat.y, bounds.Max.X, bounds.Max.Y)
	}
	if s.Target == tourAgentsButton {
		target = w.bar.Agents
	}
	if s.Target != tourNone && !target.Empty() {
		stroke(ops, target, 4, th.Accent)
	}
	measurePage := &page{gtx: g, c: c, st: &w.set}
	width := min(g.Dp(440), max(1, bounds.Dx()-g.Dp(32)))
	lines := measurePage.wrap(s.Body, 13, max(1, width-g.Dp(36)))
	_, lineSize := c.label(g, "Ag", 13, th.ToastFG)
	height := g.Dp(135) + len(lines)*(lineSize.Y+g.Dp(7))
	if s.Hooks {
		height += g.Dp(64)
	}
	r := tourCardRect(bounds, target, image.Pt(width, min(g.Dp(370), height)))
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
	for i := 0; i < 6; i++ {
		dot := th.Muted
		if i == w.tour.step {
			dot = th.Accent
		}
		dx := r.Max.X - g.Dp(unit.Dp(18+(5-i)*10))
		dy := r.Min.Y + g.Dp(22)
		paint.FillShape(ops, dot, clip.Ellipse(image.Rect(dx, dy, dx+g.Dp(4), dy+g.Dp(4))).Op(ops))
	}
	call, sz := c.labelW(g, s.Title, 20, th.ToastTitle, font.Bold)
	place(ops, call, image.Pt(x, y))
	y += sz.Y + g.Dp(12)
	// Reuse the settings word wrapper, including bounded splitting of long words.
	bodyRect := image.Rect(r.Min.X, y, r.Max.X, max(y, r.Max.Y-g.Dp(48)))
	bodyClip := clip.Rect(bodyRect).Push(ops)
	y -= w.tour.scroll
	p := &page{gtx: g, c: c, st: &w.set}
	for _, line := range p.wrap(s.Body, 13, max(r.Dx()-g.Dp(36), 1)) {
		put(line, 13, th.ToastFG)
	}
	w.tour.hits = w.tour.hits[:0]
	button := func(text string, action int, at image.Point) image.Point {
		call, sz := c.label(g, text, 12, th.ToastGo)
		place(ops, call, at)
		rr := image.Rectangle{Min: at.Sub(image.Pt(4, 4)), Max: at.Add(sz).Add(image.Pt(4, 4))}
		if action > 0 {
			rr = rr.Intersect(bodyRect)
		}
		if !rr.Empty() {
			w.tour.hits = append(w.tour.hits, tourHit{rr, action})
		}
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
	w.tour.scrollMax = max(0, y+w.tour.scroll-(r.Max.Y-g.Dp(48)))
	bodyClip.Pop()
	if w.tour.scrollMax > 0 {
		track := image.Rect(r.Max.X-g.Dp(8), r.Min.Y+g.Dp(85), r.Max.X-g.Dp(5), r.Max.Y-g.Dp(52))
		paint.FillShape(ops, th.Border, clip.Rect(track).Op())
		thumb := track
		thumb.Min.Y += w.tour.scroll * max(0, track.Dy()-g.Dp(20)) / w.tour.scrollMax
		thumb.Max.Y = min(track.Max.Y, thumb.Min.Y+g.Dp(20))
		paint.FillShape(ops, th.Muted, clip.Rect(thumb).Op())
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
