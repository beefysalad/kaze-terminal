package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	kinput "github.com/beefysalad/kaze-terminal/internal/input"
)

type emptyState struct {
	newSession widget.Clickable
	button     image.Rectangle
	shortcut   string
	shown      bool
}

// Empty terminal windows stay open. Only the opt-in benchmark requests an
// automatic close; ordinary window-close and app-quit events remain native.
func (win *window) shouldClose() bool {
	return win.bench != nil && win.bench.ShouldQuit()
}

// A final plain shell exiting has the same result as closing its session.
// Keep exited workspace members: their stopped state and output remain useful.
func (win *window) closeExitedFinalSession() {
	final := win.mgr.Active()
	if final == nil || !final.Exited || final.Group != nil {
		return
	}
	for _, t := range win.mgr.Tabs() {
		if t.ID != final.ID {
			return
		}
	}
	win.closeTab(final.ID)
}

func (win *window) emptyEvents(gtx layout.Context, cw, ch int) {
	// Configured actions take precedence over button activation, including a
	// user's explicit Enter binding. Register them for both old and empty focus.
	var filters []event.Filter
	for sc := range win.km {
		for _, tag := range []event.Tag{win.termTag, &win.empty.newSession} {
			filters = append(filters, key.Filter{Focus: tag, Name: sc.Name, Required: sc.Mods})
		}
	}
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		if e, ok := e.(key.Event); ok && e.State == key.Press && !win.bench.IgnoreInput() {
			win.runShortcut(gtx, win.km[kinput.Shortcut{Name: e.Name, Mods: e.Modifiers}])
			if win.emptyFollowup(gtx, cw, ch) {
				return
			}
		}
	}
	for win.empty.newSession.Clicked(gtx) {
		if !win.bench.IgnoreInput() {
			win.newTab()
			win.emptyFollowup(gtx, cw, ch)
			return
		}
	}
}

// Install the newly opened view's input filters before the frame gives it
// focus. Gio otherwise drops that focus command because the handler is new.
func (win *window) emptyFollowup(gtx layout.Context, cw, ch int) bool {
	switch {
	case win.palette.open:
		win.paletteEvents(gtx)
	case win.set.shown:
		win.settingsEvents(gtx)
	case win.board.open:
		win.boardEvents(gtx)
	case win.side.launch.open:
		win.branchEvents(gtx)
	default:
		if t := win.mgr.Active(); t != nil {
			win.termEvents(gtx, win.mainView(t), cw, ch)
		} else {
			return false
		}
	}
	return true
}

func (win *window) drawEmpty(gtx layout.Context, r image.Rectangle) {
	c, ops := win.ui, gtx.Ops
	col := c.pageColors()
	area := clip.Rect(r).Push(ops)
	defer area.Pop()
	paint.FillShape(ops, col.bg, clip.Rect(r).Op())
	if !win.empty.shown {
		win.empty.shown = true
		win.wantFocus = true
	}
	width := min(gtx.Dp(360), max(r.Dx()-gtx.Dp(40), 1))
	label := func(s string, size unit.Sp, fg color.NRGBA, weight font.Weight) (op.CallOp, image.Point) {
		rec := op.Record(ops)
		mat := op.Record(ops)
		paint.ColorOp{Color: fg}.Add(ops)
		material := mat.Stop()
		lg := gtx
		lg.Constraints = layout.Constraints{Max: image.Pt(width, 1<<16)}
		dims := widget.Label{Alignment: text.Middle}.Layout(lg, c.shaper, font.Font{Typeface: sansFace, Weight: weight}, size, c.fit(s), material)
		return rec.Stop(), dims.Size
	}
	title, ts := label("No open sessions", 22, col.title, font.SemiBold)
	hint, hs := label("Start a session to get going.", 13, c.emptyMuted(), font.Normal)
	win.empty.shortcut = win.keyText("new_tab")
	var shortcut op.CallOp
	var ss image.Point
	if win.empty.shortcut != "" {
		shortcut, ss = label(win.empty.shortcut, 11.5, c.emptyMuted(), font.Medium)
	}
	buttonLabel, bs := c.sans(gtx, "New session", 12.5, col.title, font.SemiBold)
	buttonSize := image.Pt(min(bs.X+gtx.Dp(32), width), gtx.Dp(36))
	total := ts.Y + gtx.Dp(8) + hs.Y + gtx.Dp(24) + buttonSize.Y
	if win.empty.shortcut != "" {
		total += gtx.Dp(10) + ss.Y
	}
	y := r.Min.Y + max((r.Dy()-total)/2, gtx.Dp(16))
	centered := func(call op.CallOp, size image.Point) { place(ops, call, image.Pt(r.Min.X+(r.Dx()-size.X)/2, y)) }
	centered(title, ts)
	y += ts.Y + gtx.Dp(8)
	centered(hint, hs)
	y += hs.Y + gtx.Dp(24)
	at := image.Pt(r.Min.X+(r.Dx()-buttonSize.X)/2, y)
	win.empty.button = image.Rectangle{Min: at, Max: at.Add(buttonSize)}
	off := op.Offset(at).Push(ops)
	bg := col.raised
	if win.empty.newSession.Hovered() || win.empty.newSession.Pressed() {
		bg = mix(col.raised, col.accent, 0.15)
	}
	box := image.Rectangle{Max: buttonSize}
	paint.FillShape(ops, bg, clip.UniformRRect(box, gtx.Dp(8)).Op(ops))
	stroke(ops, box, gtx.Dp(8), col.line2)
	if gtx.Focused(&win.empty.newSession) {
		stroke(ops, box.Inset(-gtx.Dp(2)), gtx.Dp(10), col.accent)
	}
	bgx := gtx
	bgx.Constraints = layout.Exact(buttonSize)
	win.empty.newSession.Layout(bgx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("New session").Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		place(gtx.Ops, buttonLabel, image.Pt((buttonSize.X-bs.X)/2, (buttonSize.Y-bs.Y)/2))
		return layout.Dimensions{Size: buttonSize}
	})
	off.Pop()
	if win.empty.shortcut != "" {
		y += buttonSize.Y + gtx.Dp(10)
		centered(shortcut, ss)
	}
}

func (c *chrome) emptyMuted() color.NRGBA {
	col := c.pageColors()
	return mix(col.muted, col.title, 0.2)
}
