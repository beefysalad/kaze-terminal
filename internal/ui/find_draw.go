package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// findPane is the pane the bar is drawn over this frame: the member tab's body
// or the session's terminal area (term, the part above any panel).
func (win *window) findPane(term image.Rectangle) image.Rectangle {
	if _, ok := win.findView(); !ok {
		return image.Rectangle{}
	}
	if win.find.member {
		return win.dock.hits.body
	}
	return term
}

// drawFind draws the bar at the top right of pane while it is open.
func (win *window) drawFind(gtx layout.Context, pane image.Rectangle) {
	f := &win.find
	f.pane = pane
	if !f.open || pane.Empty() {
		f.hits = findHits{}
		return
	}
	ops, dp, th := gtx.Ops, gtx.Dp, win.ui.th
	w := min(dp(470), pane.Dx()-dp(16))
	right := pane.Max.X - dp(18)
	bar := image.Rect(right-w, pane.Min.Y+dp(8), right, pane.Min.Y+dp(8)+dp(34))
	field := image.Rect(bar.Min.X+dp(28), bar.Min.Y, min(bar.Min.X+dp(28)+dp(220), bar.Max.X), bar.Max.Y)
	f.hits = findHits{bar: bar, field: field}
	paint.FillShape(ops, th.ToastLine, clip.UniformRRect(bar, dp(10)).Op(ops))
	paint.FillShape(ops, th.ToastBG, clip.UniformRRect(bar.Inset(dp(1)), dp(9)).Op(ops))
	win.ui.lineEditor(gtx, &f.ed, field, "Find in terminal", th.Muted, 0)
}
