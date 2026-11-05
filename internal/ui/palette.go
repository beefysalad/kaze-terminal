package ui

import (
	"image"
	"image/color"
	"strings"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// The palette's size in dp at 100% scale (mockup .pal).
const (
	palW          = 600
	palTop        = 60
	palRadius     = 14
	palFieldH     = 54
	palRowH       = 38
	palRowRadius  = 8
	palInset      = 6
	palHeadH      = 30
	palFootH      = 36
	palListMaxH   = 420
	palEmptyH     = 96
	palSwitchMinW = 440 // a narrower box hides the scope switch (Ruling 11)
)

// The field's placeholders, per scope.
const (
	palPlaceSessions = "Search sessions and Kaze actions"
	palPlaceSkills   = "Search skills by name or what they do"
)

// The empty list's direction line, per scope.
const (
	palEmptySessions = "Press / to search skills instead."
	palEmptySkills   = "Try part of what the skill does, like /review."
)

// palFootKeys is one footer hint: its keycaps, then its word.
type palFootKeys struct {
	caps []string
	word string
}

var (
	palFootPlain = []palFootKeys{{[]string{"↑", "↓"}, "move"}, {[]string{"↵"}, "open"}, {[]string{"/"}, "skills"}}
	palFootSkill = []palFootKeys{{[]string{"↑", "↓"}, "move"}, {[]string{"↵"}, "run here"}, {[]string{"Shift", "↵"}, "new session"}}
	palFootEsc   = palFootKeys{[]string{"Esc"}, "close"}
)

// palState is what the drawing needs from the window.
type palState struct {
	items       []palItem
	sel, scroll int
	hover       int
	reveal      bool // scroll the selected row into view
	ed          *widget.Editor
	skills      bool   // the Skills scope: the text starts with "/"
	query       string // the text, for the empty list's line
	footSkill   bool   // the selected row is a skill: the footer's skill keys
}

// palHits is where the last frame drew the palette, in window pixels.
type palHits struct {
	box, field, list image.Rectangle
	rows             []palRowHit // visible rows only
	headers          []string
	scroll           int // the scroll the list was drawn at
	scrollMax        int
	editor           image.Rectangle // where the query text sits
	// The scope switch's parts and the Skills chip; zero when not drawn.
	switchSessions, switchSkills, chip image.Rectangle
	lines                              []string // the empty list's two lines (a test seam)
}

type palRowHit struct {
	index int
	rect  image.Rectangle
}

// rowAt is the item index of the visible row under p.
func (h palHits) rowAt(p image.Point) (int, bool) {
	for _, r := range h.rows {
		if p.In(r.rect) {
			return r.index, true
		}
	}
	return 0, false
}

type palEntry struct {
	y, h   int
	header string
	index  int // item index; -1 for a header, palEmptyEntry for the empty lines
}

const palEmptyEntry = -2

// layoutPalette draws the palette over main, the main view, which it dims:
// the field row with the scope switch, the grouped list and the key help.
// Only rows inside the list's visible part are laid out.
func (c *chrome) layoutPalette(gtx layout.Context, st palState, main image.Rectangle) palHits {
	ops, th := gtx.Ops, c.th
	var h palHits
	if main.Dx() <= 2 || main.Dy() <= 2 {
		return h
	}
	ink := c.palInk()
	paint.FillShape(ops, tint(th.Chrome, 0x88), clip.Rect(main).Op())

	rowH, headH := gtx.Dp(palRowH), gtx.Dp(palHeadH)
	var entries []palEntry
	y := gtx.Dp(4) // the list's top padding (mockup .list)
	for i, it := range st.items {
		if i == 0 || it.Group != st.items[i-1].Group {
			entries = append(entries, palEntry{y: y, h: headH, header: it.Group, index: -1})
			y += headH
		}
		entries = append(entries, palEntry{y: y, h: rowH, index: i})
		y += rowH
	}
	if len(st.items) == 0 {
		entries = append(entries, palEntry{y: 0, h: gtx.Dp(palEmptyH), index: palEmptyEntry})
		y = gtx.Dp(palEmptyH)
	}
	total := y + gtx.Dp(8)

	w := min(gtx.Dp(palW), main.Dx()*92/100)
	x := main.Min.X + (main.Dx()-w)/2
	top := main.Min.Y + gtx.Dp(palTop)
	fieldH, footH := gtx.Dp(palFieldH), gtx.Dp(palFootH)
	listH := min(total, gtx.Dp(palListMaxH))
	if room := main.Max.Y - top - fieldH - 1 - footH - gtx.Dp(8); listH > room {
		// A short window: move the box up first, then shorten the list.
		top = max(main.Min.Y+gtx.Dp(4), top-(listH-room))
		listH = max(min(listH, main.Max.Y-top-fieldH-1-footH-gtx.Dp(4)), 0)
	}
	box := image.Rect(x, top, x+w, top+fieldH+1+listH+footH).Intersect(main)
	h.box = box
	radius, line := gtx.Dp(palRadius), max(gtx.Dp(1), 1)
	// No blur shadow: a dark line just outside the box gives the edge.
	paint.FillShape(ops, ink.outer, clip.UniformRRect(box.Inset(-line).Intersect(main), radius+line).Op(ops))
	paint.FillShape(ops, ink.line, clip.UniformRRect(box, radius).Op(ops))
	paint.FillShape(ops, ink.bg, clip.UniformRRect(box.Inset(line), max(radius-line, 0)).Op(ops))
	cl := clip.UniformRRect(box, radius).Push(ops)
	defer cl.Pop()
	c.palHairline(gtx, box, ink)

	h.field = image.Rect(box.Min.X, box.Min.Y, box.Max.X, min(box.Min.Y+fieldH, box.Max.Y))
	c.palField(gtx, st, &h, ink)
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(box.Min.X, h.field.Max.Y, box.Max.X, h.field.Max.Y+1)).Op())

	// The list.
	h.list = image.Rect(box.Min.X, h.field.Max.Y+1, box.Max.X, min(h.field.Max.Y+1+listH, box.Max.Y))
	h.scrollMax = max(total-h.list.Dy(), 0)
	sc := min(max(st.scroll, 0), h.scrollMax)
	if st.reveal {
		for _, e := range entries {
			if e.index == st.sel {
				top := e.y
				if e.index == firstInGroup(st.items, e.index) {
					top -= headH // keep the group's header in view with its first row
				}
				sc = max(min(sc, top), e.y+e.h-h.list.Dy())
				sc = min(max(sc, 0), h.scrollMax)
			}
		}
	}
	h.scroll = sc
	inset := gtx.Dp(palInset)
	lc := clip.Rect(h.list).Push(ops)
	for _, e := range entries {
		if e.header != "" {
			h.headers = append(h.headers, e.header)
		}
		ey := h.list.Min.Y + e.y - sc
		if ey+e.h <= h.list.Min.Y || ey >= h.list.Max.Y {
			continue
		}
		rr := image.Rect(box.Min.X+inset, ey, box.Max.X-inset, ey+e.h)
		switch e.index {
		case palEmptyEntry:
			c.palEmpty(gtx, st, &h, rr, ink)
		case -1:
			call, sz := c.fitSide(gtx, e.header, 10.5, rr.Dx()-2*gtx.Dp(10), ink.faint)
			place(ops, call, image.Pt(rr.Min.X+gtx.Dp(10), ey+e.h-sz.Y-gtx.Dp(5)))
		default:
			c.palRow(gtx, st.items[e.index], rr, e.index == st.sel)
			h.rows = append(h.rows, palRowHit{index: e.index, rect: rr.Intersect(h.list)})
		}
	}
	lc.Pop()

	c.palFoot(gtx, st, image.Rect(box.Min.X, h.list.Max.Y, box.Max.X, box.Max.Y), ink)
	return h
}

// palHairline is the 1 dp line across the box's top, bright in the middle
// and fading out toward both ends, inset 14 dp.
func (c *chrome) palHairline(gtx layout.Context, box image.Rectangle, ink *palInk) {
	ops := gtx.Ops
	x0, x1 := box.Min.X+gtx.Dp(14), box.Max.X-gtx.Dp(14)
	if x1-x0 < 2 {
		return
	}
	mid, y, hl := (x0+x1)/2, box.Min.Y, max(gtx.Dp(1), 1)
	clear := tint(ink.hairline, 0)
	half := func(a, b int, ca, cb color.NRGBA) {
		paint.LinearGradientOp{Stop1: f32.Pt(float32(a), float32(y)), Color1: ca, Stop2: f32.Pt(float32(b), float32(y)), Color2: cb}.Add(ops)
		r := clip.Rect(image.Rect(a, y, b, y+hl)).Push(ops)
		paint.PaintOp{}.Add(ops)
		r.Pop()
	}
	half(x0, mid, clear, ink.hairline)
	half(mid, x1, ink.hairline, clear)
}

// palField draws the field row: the magnifier, in Skills the "/ Skills" chip,
// the editor with its placeholder, and when the box is wide enough the
// Sessions | Skills switch at the right.
func (c *chrome) palField(gtx layout.Context, st palState, h *palHits, ink *palInk) {
	ops, f := gtx.Ops, h.field
	pad, gap := gtx.Dp(14), gtx.Dp(10)
	mid := f.Min.Y + f.Dy()/2
	g := gtx.Dp(16)
	c.icon(gtx, "search", image.Rect(f.Min.X+pad, mid-g/2, f.Min.X+pad+g, mid-g/2+g), ink.muted)
	x := f.Min.X + pad + g + gap
	right := f.Max.X - pad
	if h.box.Dx() >= gtx.Dp(palSwitchMinW) {
		right = c.palSwitch(gtx, st.skills, right, mid, h, ink) - gap
	}
	if st.skills {
		x = c.palChip(gtx, x, right, mid, h, ink)
	}

	const fieldSp unit.Sp = 16
	_, lh := c.sans(gtx, "Ag", fieldSp, ink.text, font.Normal)
	h.editor = image.Rect(x, mid-lh.Y/2, max(right, x), mid-lh.Y/2+lh.Y)
	if st.ed == nil {
		return
	}
	room := h.editor.Dx()
	switch {
	case st.ed.Len() == 0:
		call, _ := c.fitSideW(gtx, palPlaceSessions, fieldSp, room, ink.faint, font.Normal)
		place(ops, call, h.editor.Min)
	case st.skills && strings.TrimSpace(st.query) == "/":
		_, qs := c.sans(gtx, st.query, fieldSp, ink.text, font.Normal)
		at := qs.X + gtx.Dp(6)
		call, _ := c.fitSideW(gtx, palPlaceSkills, fieldSp, room-at, ink.faint, font.Normal)
		place(ops, call, h.editor.Min.Add(image.Pt(at, 0)))
	}
	rec := op.Record(ops)
	paint.ColorOp{Color: ink.text}.Add(ops)
	fg := rec.Stop()
	rec = op.Record(ops)
	paint.ColorOp{Color: tint(c.th.Accent, 0x55)}.Add(ops)
	sel := rec.Stop()
	eg := gtx
	eg.Constraints = layout.Exact(image.Pt(max(room, 1), max(lh.Y, 1)))
	t := op.Offset(h.editor.Min).Push(ops)
	st.ed.Layout(eg, c.shaper, font.Font{Typeface: sansFace}, fieldSp, fg, sel)
	t.Pop()
}

// palChip draws the "/ Skills" chip from x when it fits before right and
// returns where the editor starts.
func (c *chrome) palChip(gtx layout.Context, x, right, mid int, h *palHits, ink *palInk) int {
	ops := gtx.Ops
	slash, ssz := c.label(gtx, "/", 12, ink.chipFG)
	word, wsz := c.sans(gtx, "Skills", 12, ink.chipFG, font.SemiBold)
	ch := max(ssz.Y, wsz.Y) + gtx.Dp(4)
	cw := gtx.Dp(6) + ssz.X + gtx.Dp(4) + wsz.X + gtx.Dp(8)
	if x+cw > right {
		return x
	}
	chip := image.Rect(x, mid-ch/2, x+cw, mid-ch/2+ch)
	r, line := gtx.Dp(6), max(gtx.Dp(1), 1)
	paint.FillShape(ops, ink.chipLine, clip.UniformRRect(chip, r).Op(ops))
	paint.FillShape(ops, ink.chipBG, clip.UniformRRect(chip.Inset(line), max(r-line, 0)).Op(ops))
	place(ops, slash, image.Pt(chip.Min.X+gtx.Dp(6), mid-ssz.Y/2))
	place(ops, word, image.Pt(chip.Min.X+gtx.Dp(6)+ssz.X+gtx.Dp(4), mid-wsz.Y/2))
	h.chip = chip
	return chip.Max.X + gtx.Dp(10)
}

// palSwitch draws Sessions | Skills / ending at right, centred on mid, and
// returns its left edge.
func (c *chrome) palSwitch(gtx layout.Context, skills bool, right, mid int, h *palHits, ink *palInk) int {
	ops := gtx.Ops
	p2, px, py, line := gtx.Dp(2), gtx.Dp(10), gtx.Dp(4), max(gtx.Dp(1), 1)
	sFG, kFG, keyFG := ink.text, ink.muted, ink.faint
	if skills {
		sFG, kFG, keyFG = ink.muted, ink.text, ink.goInk
	}
	sess, ssz := c.sans(gtx, "Sessions", 12, sFG, font.Medium)
	sk, ksz := c.sans(gtx, "Skills", 12, kFG, font.Medium)
	slash, lsz := c.label(gtx, "/", 11, keyFG)
	ph := max(ssz.Y, ksz.Y) + 2*py
	sw := ssz.X + 2*px
	kw := ksz.X + gtx.Dp(6) + lsz.X + 2*px
	top := mid - ph/2
	outer := image.Rect(right-(p2+sw+p2+kw+p2), top-p2, right, top+ph+p2)
	rad := gtx.Dp(8)
	paint.FillShape(ops, c.th.Border, clip.UniformRRect(outer, rad).Op(ops))
	paint.FillShape(ops, ink.switchBG, clip.UniformRRect(outer.Inset(line), max(rad-line, 0)).Op(ops))
	h.switchSessions = image.Rect(outer.Min.X+p2, top, outer.Min.X+p2+sw, top+ph)
	h.switchSkills = image.Rect(h.switchSessions.Max.X+p2, top, h.switchSessions.Max.X+p2+kw, top+ph)
	onPart := h.switchSessions
	if skills {
		onPart = h.switchSkills
	}
	r := gtx.Dp(6)
	paint.FillShape(ops, ink.line, clip.UniformRRect(onPart, r).Op(ops))
	paint.FillShape(ops, ink.raised, clip.UniformRRect(onPart.Inset(line), max(r-line, 0)).Op(ops))
	place(ops, sess, image.Pt(h.switchSessions.Min.X+px, mid-ssz.Y/2))
	place(ops, sk, image.Pt(h.switchSkills.Min.X+px, mid-ksz.Y/2))
	place(ops, slash, image.Pt(h.switchSkills.Min.X+px+ksz.X+gtx.Dp(6), mid-lsz.Y/2))
	return outer.Min.X
}

// palKeycapW is the width of s drawn as a keycap.
func (c *chrome) palKeycapW(gtx layout.Context, s string, ink *palInk) int {
	_, sz := c.label(gtx, s, 11, ink.keyFG)
	return sz.X + 2*gtx.Dp(5) + 2*max(gtx.Dp(1), 1)
}

// palKeycap draws s as a keycap whose left edge is x, centred on mid, and
// returns its width: mono 11 sp, a 1 dp line with a 2 dp bottom, 5 dp corners.
func (c *chrome) palKeycap(gtx layout.Context, s string, x, mid int, ink *palInk) int {
	ops := gtx.Ops
	call, sz := c.label(gtx, s, 11, ink.keyFG)
	line := max(gtx.Dp(1), 1)
	kh := gtx.Dp(17) + 3*line
	kw := sz.X + 2*gtx.Dp(5) + 2*line
	r := image.Rect(x, mid-kh/2, x+kw, mid-kh/2+kh)
	rad := gtx.Dp(5)
	paint.FillShape(ops, ink.keyLine, clip.UniformRRect(r, rad).Op(ops))
	in := image.Rect(r.Min.X+line, r.Min.Y+line, r.Max.X-line, r.Max.Y-2*line)
	paint.FillShape(ops, ink.keyBG, clip.UniformRRect(in, max(rad-line, 0)).Op(ops))
	place(ops, call, image.Pt(in.Min.X+(in.Dx()-sz.X)/2, in.Min.Y+(in.Dy()-sz.Y)/2))
	return kw
}

// palFootWidth is the width hint k takes in the footer.
func (c *chrome) palFootWidth(gtx layout.Context, k palFootKeys, ink *palInk) int {
	w := 0
	for i, s := range k.caps {
		if i > 0 {
			w += gtx.Dp(3)
		}
		w += c.palKeycapW(gtx, s, ink)
	}
	_, ws := c.sans(gtx, k.word, 12, ink.muted, font.Normal)
	return w + gtx.Dp(6) + ws.X
}

// palFootHint draws k from x, centred on mid.
func (c *chrome) palFootHint(gtx layout.Context, k palFootKeys, x, mid int, ink *palInk) {
	for i, s := range k.caps {
		if i > 0 {
			x += gtx.Dp(3)
		}
		x += c.palKeycap(gtx, s, x, mid, ink)
	}
	call, ws := c.sans(gtx, k.word, 12, ink.muted, font.Normal)
	place(gtx.Ops, call, image.Pt(x+gtx.Dp(6), mid-ws.Y/2))
}

// palFoot is the key help: the selected row's keys from the left, "Esc
// close" at the right; a hint that would not fit is left out.
func (c *chrome) palFoot(gtx layout.Context, st palState, foot image.Rectangle, ink *palInk) {
	if foot.Dy() <= 0 {
		return
	}
	ops := gtx.Ops
	paint.FillShape(ops, ink.foot, clip.Rect(foot).Op())
	paint.FillShape(ops, c.th.Border, clip.Rect(image.Rect(foot.Min.X, foot.Min.Y, foot.Max.X, foot.Min.Y+1)).Op())
	pad, gap := gtx.Dp(14), gtx.Dp(16)
	mid := foot.Min.Y + (foot.Dy()+1)/2
	right := foot.Max.X - pad
	if ew := c.palFootWidth(gtx, palFootEsc, ink); right-ew >= foot.Min.X+pad {
		right -= ew
		c.palFootHint(gtx, palFootEsc, right, mid, ink)
		right -= gap
	}
	hints := palFootPlain
	if st.footSkill {
		hints = palFootSkill
	}
	x := foot.Min.X + pad
	for _, k := range hints {
		w := c.palFootWidth(gtx, k, ink)
		if x+w > right {
			break
		}
		c.palFootHint(gtx, k, x, mid, ink)
		x += w + gap
	}
}

// palEmpty is the empty list: No results for the text, then the scope's line.
func (c *chrome) palEmpty(gtx layout.Context, st palState, h *palHits, r image.Rectangle, ink *palInk) {
	q := strings.TrimPrefix(strings.TrimLeft(st.query, " "), "/")
	first, second := `No results for "`+q+`"`, palEmptySessions
	if st.skills {
		second = palEmptySkills
	}
	h.lines = []string{first, second}
	room := r.Dx() - 2*gtx.Dp(10)
	a, as := c.fitSideW(gtx, first, 14, room, ink.text, font.Normal)
	b, bs := c.fitSideW(gtx, second, 13, room, ink.muted, font.Normal)
	gap := gtx.Dp(4)
	y := r.Min.Y + (r.Dy()-as.Y-gap-bs.Y)/2
	place(gtx.Ops, a, image.Pt(r.Min.X+(r.Dx()-as.X)/2, y))
	place(gtx.Ops, b, image.Pt(r.Min.X+(r.Dx()-bs.X)/2, y+as.Y+gap))
}

// scopeAt is the scope a click at p asks for: the switch part it is on, or
// Sessions for the chip.
func (h palHits) scopeAt(p image.Point) (skills, ok bool) {
	switch {
	case p.In(h.switchSessions), p.In(h.chip):
		return false, true
	case p.In(h.switchSkills):
		return true, true
	}
	return false, false
}

// setPaletteScope puts the field in Skills (a "/" in front of the text) or
// Sessions (the "/" taken off), keeping the rest of the text and the
// keyboard in the field.
func (win *window) setPaletteScope(skills bool) {
	p := &win.palette
	win.wantFocus = true
	t := p.ed.Text()
	if palSkillScope(t) == skills {
		return
	}
	if skills {
		t = "/" + t
	} else {
		t = strings.TrimPrefix(strings.TrimLeft(t, " "), "/")
	}
	p.ed.SetText(t)
	n := utf8.RuneCountInString(t)
	p.ed.SetCaret(n, n)
	win.ui.fit(t)
	p.sel, p.scroll = "", 0
	win.w.Invalidate()
}

func firstInGroup(items []palItem, i int) int {
	for i > 0 && items[i-1].Group == items[i].Group {
		i--
	}
	return i
}

// palRow draws one item in r: a session's mark, the name, and at the right
// the session's state, the skill's description or the action's shortcut, in
// at most 60% of the row.
func (c *chrome) palRow(gtx layout.Context, it palItem, r image.Rectangle, selected bool) {
	ops, th := gtx.Ops, c.th
	if selected {
		paint.FillShape(ops, th.Active, clip.UniformRRect(r, gtx.Dp(6)).Op(ops))
	}
	const sz unit.Sp = 12.5
	pad := gtx.Dp(8)
	x := r.Min.X + pad
	mid := r.Min.Y + r.Dy()/2
	detail := it.Detail
	switch it.Kind {
	case palSession:
		dot := gtx.Dp(8)
		d := image.Rect(x, mid-dot/2, x+dot, mid+dot/2)
		if it.Mark == markNeeds {
			paint.FillShape(ops, th.Needs, clip.Ellipse(d).Op(ops))
		} else {
			c.sideDot(gtx, d, it.Mark, it.Running)
		}
		x += dot + gtx.Dp(10)
		detail = it.State
	case palKaze:
		detail = strings.Join(it.Keys, "+")
	}
	right := r.Max.X - pad
	detailW := 0
	if detail != "" {
		call, dsz := c.fitSide(gtx, detail, 11.5, (r.Dx()*60)/100, th.Muted)
		place(ops, call, image.Pt(right-dsz.X, mid-dsz.Y/2))
		detailW = dsz.X + gtx.Dp(16)
	}
	room := max(right-detailW-x, 0)
	fg := th.ToastFG
	switch {
	case it.Kind == palSkill:
		fg = c.memberColor("magenta")
	case it.Kind == palBrowse || it.Kind == palMoreMatches || it.Kind == palMoreGroup:
		fg = th.Muted
	case selected:
		fg = th.ActiveFG
	}
	call, nsz := c.fitSide(gtx, it.Name, sz, room, fg)
	place(ops, call, image.Pt(x, mid-nsz.Y/2))
}
