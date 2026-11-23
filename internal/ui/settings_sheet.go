package ui

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// sheetRect uses window coordinates, including a nonzero content origin.
func sheetRect(window image.Rectangle, metrics unit.Metric) image.Rectangle {
	if window.Dx() < metrics.Dp(900) || window.Dy() < metrics.Dp(680) {
		return window
	}
	size := image.Pt(metrics.Dp(880), metrics.Dp(640))
	at := window.Min.Add(window.Size().Sub(size).Div(2))
	return image.Rectangle{Min: at, Max: at.Add(size)}
}

func (win *window) drawSettingsSheet(gtx layout.Context, bounds image.Rectangle) {
	ops := gtx.Ops
	r := sheetRect(bounds, gtx.Metric)
	win.set.sheet = r
	alpha := uint8(140) // 55 percent on dark themes, 25 on light.
	if c := win.ui.pageColors().bg; int(c.R)+int(c.G)+int(c.B) > 3*128 {
		alpha = 64
	}
	paint.FillShape(ops, color.NRGBA{A: alpha}, clip.Rect(bounds).Op())
	area := clip.Rect(bounds).Push(ops)
	event.Op(ops, &win.set.tag) // blocks every underlying pointer target
	area.Pop()
	radius := 0
	if r != bounds {
		radius = gtx.Dp(12)
		// Layered translucent edges soften the shadow, offset below the sheet.
		for spread := 8; spread > 0; spread-- {
			shadow := r.Add(image.Pt(0, gtx.Dp(4))).Inset(-gtx.Dp(unit.Dp(spread)))
			paint.FillShape(ops, color.NRGBA{A: 5}, clip.UniformRRect(shadow, radius+gtx.Dp(unit.Dp(spread))).Op(ops))
		}
	}
	cl := clip.UniformRRect(r, radius).Push(ops)
	win.drawSettings(gtx, r)
	cl.Pop()
	if radius > 0 {
		stroke(ops, r, radius, win.ui.pageColors().line)
	}
}
