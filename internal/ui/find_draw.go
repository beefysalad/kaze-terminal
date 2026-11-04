package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/beefysalad/kaze-terminal/internal/render"
)

// The bar's buttons, in findState.hover.
const (
	findBtnNone = iota
	findBtnCase
	findBtnPrev
	findBtnNext
	findBtnClose
)

// findInk is the bar's colours from the chrome tokens (Ruling 20), each text
// and icon made readable on the fill it sits on.
type findInk struct {
	bg, line, hover, on, sep            color.NRGBA
	text, placeholder, count, none, sel color.NRGBA
	btnOn, icon, iconHover              color.NRGBA
}

func (c *chrome) findInk() findInk {
	th := c.th
	bg := th.ToastBG
	// CapHover is the title bar's hover and can be the bar's own fill (Midnight):
	// a light step of the text over the bar keeps the hover visible everywhere.
	hover := mix(bg, th.ToastFG, 0.08)
	on := mix(bg, th.Accent, float32(0x30)/0xff) // tint(Accent, 0x30) over the bar
	r := render.Readable
	return findInk{
		bg: bg, line: th.ToastLine, hover: hover, on: on, sep: th.Border,
		text: r(th.ToastFG, bg), placeholder: r(th.Muted, bg), count: r(th.Muted, bg), none: r(th.DiffDelFG, bg),
		sel:   tint(th.Accent, 0x55),
		btnOn: r(th.ToastGo, on), icon: r(th.CapGlyph, bg), iconHover: r(th.ToastFG, hover),
	}
}

// findCountText is the count the bar shows: "N of M", "N of 1000+" when more
// were found than kept, "No results", or nothing for an empty field.
func findCountText(cur, total int, capped, none bool) string {
	switch {
	case none:
		return "No results"
	case total == 0:
		return ""
	case capped:
		return fmt.Sprintf("%d of %d+", cur, total)
	}
	return fmt.Sprintf("%d of %d", cur, total)
}

// findCountKey is what the count text was last built from.
type findCountKey struct {
	cur, total   int
	capped, none bool
}

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

// drawFind draws the bar at the top right of pane while it is open (mockup
// .find): search icon, field, count, separator, Aa, previous, next, close.
// A narrow pane shrinks the field to 96 dp, then drops the count, then keeps
// only the field and × (Ruling 21).
func (win *window) drawFind(gtx layout.Context, pane image.Rectangle) {
	f := &win.find
	f.pane = pane
	f.texts = f.texts[:0]
	if !f.open || pane.Empty() {
		f.hits = findHits{}
		return
	}
	c, ops, dp := win.ui, gtx.Ops, gtx.Dp
	ink := c.findInk()
	padL, padR, gap, iconW := dp(10), dp(4), dp(4), dp(14)
	countW, sepW, btn := dp(64), max(dp(1), 1), dp(26)
	fieldW := dp(220)
	tools := sepW + gap + 4*btn + 3*gap // the separator and the four buttons
	width := func() int {
		w := padL + iconW + gap + fieldW + gap + tools + padR
		if countW > 0 {
			w += countW + gap
		}
		return w
	}
	room := pane.Dx() - dp(16)
	if w := width(); w > room {
		fieldW = max(dp(96), fieldW-(w-room)) // the field gives way first
	}
	if width() > room {
		countW = 0
	}
	w, compact := width(), false
	if w > room {
		compact = true
		w = max(room, 0)
		fieldW = max(w-padL-gap-btn-padR, 0)
	}
	top := pane.Min.Y + dp(8)
	if b := win.wsr.rect; !f.member && !b.Empty() && win.wsr.hitTab == f.tab && b.Overlaps(pane) {
		top = b.Max.Y + dp(8) // under the workspace banner
	}
	in := min(dp(18), max((pane.Dx()-w)/2, 0))
	bar := image.Rect(pane.Max.X-in-w, top, pane.Max.X-in, top+dp(34))
	h := findHits{bar: bar}
	radius, line := dp(10), max(dp(1), 1)
	paint.FillShape(ops, ink.line, clip.UniformRRect(bar, radius).Op(ops))
	paint.FillShape(ops, ink.bg, clip.UniformRRect(bar.Inset(line), max(radius-line, 0)).Op(ops))
	// The bar's own area first, so a press on it never reaches the terminal;
	// the field's editor registers its area after, on top.
	area := clip.Rect(bar).Push(ops)
	event.Op(ops, &f.tag)
	area.Pop()

	mid := func(x, size int) image.Rectangle {
		y := bar.Min.Y + (bar.Dy()-size)/2
		return image.Rect(x, y, x+size, y+size)
	}
	x := bar.Min.X + padL
	if !compact {
		c.icon(gtx, "search", mid(x, iconW), ink.icon)
		x += iconW + gap
	}
	h.field = image.Rect(x, bar.Min.Y+dp(4), x+fieldW, bar.Max.Y-dp(4))
	x += fieldW + gap
	if countW > 0 && !compact {
		h.count = image.Rect(x, bar.Min.Y, x+countW, bar.Max.Y)
		x += countW + gap
	}
	if compact {
		h.close = mid(bar.Max.X-padR-btn, btn)
	} else {
		s := mid(x, dp(18))
		paint.FillShape(ops, ink.sep, clip.Rect(image.Rect(s.Min.X, s.Min.Y, s.Min.X+sepW, s.Max.Y)).Op())
		x += sepW + gap
		h.caseBtn = mid(x, btn)
		h.prev = mid(x+btn+gap, btn)
		h.next = mid(x+2*(btn+gap), btn)
		h.close = mid(x+3*(btn+gap), btn)
	}
	f.hits = h

	win.findField(gtx, h.field, ink)
	if !h.count.Empty() {
		win.findCountDraw(gtx, h.count, ink)
	}
	button := func(r image.Rectangle, id int, on bool) color.NRGBA {
		rad := dp(6)
		switch {
		case on:
			paint.FillShape(ops, ink.on, clip.UniformRRect(r, rad).Op(ops))
			return ink.btnOn
		case f.hover == id:
			paint.FillShape(ops, ink.hover, clip.UniformRRect(r, rad).Op(ops))
			return ink.iconHover
		}
		return ink.icon
	}
	if !h.caseBtn.Empty() {
		col := button(h.caseBtn, findBtnCase, f.matchCase)
		call, sz := c.labelW(gtx, "Aa", 11.5, col, font.SemiBold)
		place(ops, call, h.caseBtn.Min.Add(h.caseBtn.Size().Sub(sz).Div(2)))
	}
	is := dp(14)
	icon := func(r image.Rectangle, id int, name string) {
		if r.Empty() {
			return
		}
		col := button(r, id, false)
		o := r.Min.Add(r.Size().Sub(image.Pt(is, is)).Div(2))
		c.icon(gtx, name, image.Rectangle{Min: o, Max: o.Add(image.Pt(is, is))}, col)
	}
	icon(h.prev, findBtnPrev, "chevup")
	icon(h.next, findBtnNext, "down")
	icon(h.close, findBtnClose, "close")
}

// findCountDraw draws the count right-aligned in r; its text is built only
// when the count changes, and it is the status a screen reader reads.
func (win *window) findCountDraw(gtx layout.Context, r image.Rectangle, ink findInk) {
	f, ops := &win.find, gtx.Ops
	var k findCountKey
	k.cur, k.total, k.capped, k.none = win.findCount()
	if k != f.countKey {
		f.countKey, f.countText = k, findCountText(k.cur, k.total, k.capped, k.none)
		f.countBuilds++
	}
	if f.countText == "" {
		return
	}
	col := ink.count
	if k.none {
		col = ink.none
	}
	call, sz := win.ui.label(gtx, f.countText, 11.5, col)
	place(ops, call, image.Pt(r.Max.X-gtx.Dp(6)-sz.X, r.Min.Y+(r.Dy()-sz.Y)/2))
	cl := clip.Rect(r).Push(ops)
	semantic.LabelOp(f.countText).Add(ops)
	cl.Pop()
	f.texts = append(f.texts, f.countText)
}

// findField lays the bar's editor out in r, with its placeholder while empty.
func (win *window) findField(gtx layout.Context, r image.Rectangle, ink findInk) {
	f, c, ops := &win.find, win.ui, gtx.Ops
	if r.Dx() <= 0 {
		return
	}
	if f.phTab != f.tab || f.phMember != f.member || f.placeholder == "" {
		f.phTab, f.phMember = f.tab, f.member
		f.placeholder = "Find in terminal"
		if f.member {
			if name := win.memberTabLabel(f.tab).name; name != "" {
				f.placeholder = "Find in " + name
			}
		}
	}
	_, lineSz := c.label(gtx, "Ag", 12.5, ink.text)
	at := image.Pt(r.Min.X, r.Min.Y+(r.Dy()-lineSz.Y)/2)
	if f.ed.Len() == 0 {
		call, _ := c.fitSide(gtx, f.placeholder, 12.5, r.Dx(), ink.placeholder)
		place(ops, call, at)
		f.texts = append(f.texts, f.placeholder)
	}
	cl := clip.Rect(r).Push(ops)
	semantic.LabelOp("Find").Add(ops)
	rec := op.Record(ops)
	paint.ColorOp{Color: ink.text}.Add(ops)
	fg := rec.Stop()
	rec = op.Record(ops)
	paint.ColorOp{Color: ink.sel}.Add(ops)
	sel := rec.Stop()
	eg := gtx
	eg.Constraints = layout.Exact(image.Pt(r.Dx(), lineSz.Y))
	t := op.Offset(at).Push(ops)
	f.ed.Layout(eg, c.shaper, font.Font{Typeface: chromeFace}, 12.5, fg, sel)
	t.Pop()
	cl.Pop()
}

// findPointer handles the bar's buttons: hover, and a press runs one. A
// press anywhere on the bar leaves the keyboard where it is.
func (win *window) findPointer(e pointer.Event) {
	f := &win.find
	p, h := e.Position.Round(), f.hits
	at := findBtnNone
	switch {
	case e.Kind == pointer.Leave:
	case p.In(h.caseBtn):
		at = findBtnCase
	case p.In(h.prev):
		at = findBtnPrev
	case p.In(h.next):
		at = findBtnNext
	case p.In(h.close):
		at = findBtnClose
	}
	if at != f.hover {
		f.hover = at
		win.invalidate()
	}
	if e.Kind != pointer.Press || !e.Buttons.Contain(pointer.ButtonPrimary) {
		return
	}
	switch at {
	case findBtnCase:
		f.matchCase = !f.matchCase // the search reruns, picks again and reveals it
		win.invalidate()
	case findBtnPrev:
		win.findStep(-1)
	case findBtnNext:
		win.findStep(1)
	case findBtnClose:
		win.closeFind(true)
	}
}
