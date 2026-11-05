package ui

import (
	"image"
	"image/color"
	"strings"
	"unicode"
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
	// Test seams: the badge, pill and keycap rects of visible rows, how many
	// rows showed "go to session" and how many headers drew a plugin tile.
	badges         []image.Rectangle
	goShown, tiles int
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
	of     int // a header's first item
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
	h.badges = c.palBadges[:0] // reused: a seam should cost no allocations
	defer func() { c.palBadges = h.badges }()
	paint.FillShape(ops, tint(th.Chrome, 0x88), clip.Rect(main).Op())

	rowH, headH := gtx.Dp(palRowH), gtx.Dp(palHeadH)
	var entries []palEntry
	y := gtx.Dp(4) // the list's top padding (mockup .list)
	for i, it := range st.items {
		if i == 0 || it.Group != st.items[i-1].Group {
			entries = append(entries, palEntry{y: y, h: headH, header: it.Group, index: -1, of: i})
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
			c.palHeader(gtx, st.items[e.of], rr, ink, &h)
		default:
			c.palRow(gtx, st.items[e.index], rr, e.index == st.sel, ink, &h)
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
	ink.note("search icon", ink.muted, ink.bg, 3)
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
		ink.note("placeholder", ink.faint, ink.bg, 4.5)
		call, _ := c.fitSideW(gtx, palPlaceSessions, fieldSp, room, ink.faint, font.Normal)
		place(ops, call, h.editor.Min)
	case st.skills && strings.TrimSpace(st.query) == "/":
		ink.note("skills placeholder", ink.faint, ink.bg, 4.5)
		_, qs := c.sans(gtx, st.query, fieldSp, ink.text, font.Normal)
		at := qs.X + gtx.Dp(6)
		call, _ := c.fitSideW(gtx, palPlaceSkills, fieldSp, room-at, ink.faint, font.Normal)
		place(ops, call, h.editor.Min.Add(image.Pt(at, 0)))
	}
	ink.note("field text", ink.text, ink.bg, 4.5)
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
	ink.note("chip", ink.chipFG, ink.chipBG, 4.5)
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
	sessBG, skillBG := ink.raised, ink.switchBG // the chosen part is raised
	if skills {
		sessBG, skillBG = skillBG, sessBG
	}
	ink.note("switch Sessions", sFG, sessBG, 4.5)
	ink.note("switch Skills", kFG, skillBG, 4.5)
	ink.note("switch slash", keyFG, skillBG, 4.5)
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
	ink.note("keycap", ink.keyFG, ink.keyBG, 4.5)
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
	ink.note("footer word", ink.muted, ink.foot, 4.5)
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
	ink.note("empty line", ink.text, ink.bg, 4.5)
	ink.note("empty hint", ink.muted, ink.bg, 4.5)
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
	win.wantFocus = true
	if palSkillScope(win.palette.ed.Text()) == skills {
		return
	}
	// The same switch as Tab, so an opened plugin group and the selection reset too.
	win.togglePaletteScope()
	if win.w != nil {
		win.w.Invalidate()
	}
}

func firstInGroup(items []palItem, i int) int {
	for i > 0 && items[i-1].Group == items[i].Group {
		i--
	}
	return i
}

// palHeader draws the header of it's group in r (mockup .hd): the name in
// small caps, bottom-aligned, with a plugin group's tile before it and the
// group's count at the right.
func (c *chrome) palHeader(gtx layout.Context, it palItem, r image.Rectangle, ink *palInk, h *palHits) {
	ops := gtx.Ops
	pad := gtx.Dp(10)
	x, right := r.Min.X+pad, r.Max.X-pad
	head, hs := c.sans(gtx, "Ag", 11, ink.faint, font.SemiBold)
	base := r.Max.Y - gtx.Dp(5) // the text's bottom
	mid := base - hs.Y/2
	ink.note("header", ink.faint, ink.bg, 4.5)
	if it.GroupCount != "" { // skill groups only (Ruling 8)
		call, cs := c.fitSide(gtx, it.GroupCount, 11, max(right-x, 0)/3, ink.faint)
		place(ops, call, image.Pt(right-cs.X, base-cs.Y))
		right -= cs.X + gtx.Dp(8)
	}
	if it.Plugin != "" && (it.Kind == palSkill || it.Kind == palMoreGroup) && strings.EqualFold(it.Group, it.Plugin) {
		ts := gtx.Dp(16)
		if x+ts <= right {
			tile := image.Rect(x, mid-ts/2, x+ts, mid-ts/2+ts)
			i := palPluginColor(it.Plugin)
			paint.FillShape(ops, ink.plugins[i], clip.UniformRRect(tile, gtx.Dp(4)).Op(ops))
			ink.note("plugin tile", ink.plugins[i], ink.bg, 3)
			ink.note("plugin initial", ink.pluginInk[i], ink.plugins[i], 4.5)
			call, is := c.labelW(gtx, palInitial(it.Plugin), 9, ink.pluginInk[i], font.Bold)
			place(ops, call, image.Pt(tile.Min.X+(ts-is.X)/2, tile.Min.Y+(ts-is.Y)/2))
			h.tiles++
			x = tile.Max.X + gtx.Dp(8)
		}
	}
	head, hs = c.fitSideW(gtx, it.Group, 11, max(right-x, 0), ink.faint, font.SemiBold)
	place(ops, head, image.Pt(x, base-hs.Y))
}

const palUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// palInitial is the upper-case first letter of a plugin's name.
func palInitial(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	switch {
	case n == 0:
		return ""
	case r >= 'a' && r <= 'z':
		return palUpper[r-'a' : r-'a'+1]
	case r < utf8.RuneSelf:
		return s[:1]
	}
	return string(unicode.ToUpper(r))
}

// palRow draws one item in r (mockup .row): the selected fill and bar, then
// the kind's own parts. Every text is cut to its room; nothing leaves r.
func (c *chrome) palRow(gtx layout.Context, it palItem, r image.Rectangle, selected bool, ink *palInk, h *palHits) {
	ops := gtx.Ops
	if selected {
		paint.FillShape(ops, ink.sel, clip.UniformRRect(r, gtx.Dp(palRowRadius)).Op(ops))
		bi := gtx.Dp(9)
		if bar := image.Rect(r.Min.X, r.Min.Y+bi, r.Min.X+max(gtx.Dp(2), 1), r.Max.Y-bi); bar.Dy() > 0 {
			paint.FillShape(ops, ink.selBar, clip.UniformRRect(bar, gtx.Dp(1)).Op(ops))
		}
	}
	ink.under = ink.bg
	if selected {
		ink.under = ink.sel
	}
	pad := gtx.Dp(10)
	in := image.Rect(r.Min.X+pad, r.Min.Y, r.Max.X-pad, r.Max.Y)
	if in.Dx() <= 0 {
		return
	}
	switch it.Kind {
	case palSession:
		c.palSessionRow(gtx, it, in, selected, ink, h)
	case palSkill:
		c.palSkillRow(gtx, it, in, ink)
	case palKaze:
		c.palActionRow(gtx, it, in, ink, h)
	default:
		c.palBrowseRow(gtx, it, in, ink)
	}
}

// palPill is the state pill's colours: needs you, running, done, or idle
// (idle and shell); ok is false for a state with no pill.
func (k *palInk) palPill(state string) (fg, bg color.NRGBA, ok bool) {
	switch state {
	case "needs you":
		return k.pillNeedsFG, k.pillNeedsBG, true
	case "running":
		return k.pillRunFG, k.pillRunBG, true
	case "done":
		return k.pillDoneFG, k.pillDoneBG, true
	case "idle", "shell":
		return k.pillIdleFG, k.pillIdleBG, true
	}
	return color.NRGBA{}, color.NRGBA{}, false
}

// palSessionRow: the state mark, the title, the agent badge; at the right
// the state pill and, on the selected row, "go to session ↵".
func (c *chrome) palSessionRow(gtx layout.Context, it palItem, in image.Rectangle, selected bool, ink *palInk, h *palHits) {
	ops := gtx.Ops
	mid := in.Min.Y + in.Dy()/2
	x, right := in.Min.X, in.Max.X
	dot := gtx.Dp(8)
	if x+dot > right {
		return
	}
	d := image.Rect(x, mid-dot/2, x+dot, mid-dot/2+dot)
	if it.Running && it.Mark == markNone {
		c.runningMark(gtx, centeredMarkBox(d, gtx.Dp(10)))
	} else {
		c.stateDot(gtx, d, it.Mark, it.Running, false)
	}
	x += dot + gtx.Dp(10)

	if fg, bg, ok := ink.palPill(it.State); ok {
		ink.note("pill "+it.State, fg, bg, 4.5)
		call, ps := c.sans(gtx, it.State, 11, fg, font.SemiBold)
		pw, ph := ps.X+2*gtx.Dp(8), max(gtx.Dp(20), ps.Y)
		if right-pw >= x+gtx.Dp(40) {
			pill := image.Rect(right-pw, mid-ph/2, right, mid-ph/2+ph)
			paint.FillShape(ops, bg, clip.UniformRRect(pill, min(gtx.Dp(9), ph/2)).Op(ops))
			place(ops, call, image.Pt(pill.Min.X+(pw-ps.X)/2, mid-ps.Y/2))
			h.badges = append(h.badges, pill)
			right = pill.Min.X - gtx.Dp(8)
		}
	}
	if selected {
		ink.note("go to session", ink.faint, ink.under, 4.5)
		call, gs := c.sans(gtx, "go to session ↵", 12, ink.faint, font.Normal)
		if right-gs.X >= x+gtx.Dp(60) {
			place(ops, call, image.Pt(right-gs.X, mid-gs.Y/2))
			right -= gs.X + gtx.Dp(8)
			h.goShown++
		}
	}
	right -= gtx.Dp(4) // the state's own padding (mockup .st)

	bw := 0
	var badge op.CallOp
	var bs image.Point
	if it.Agent != "" {
		ink.note("agent badge", ink.badge, ink.under, 4.5)
		badge, bs = c.label(gtx, it.Agent, 11, ink.badge)
		bw = bs.X + 2*gtx.Dp(6)
		if right-bw-gtx.Dp(10) < x+gtx.Dp(24) {
			bw = 0 // no room: the title comes first
		}
	}
	room := right - x
	if bw > 0 {
		room -= bw + gtx.Dp(10)
	}
	ink.note("session title", ink.text, ink.under, 4.5)
	call, ts := c.fitSideW(gtx, it.Name, 13.5, max(room, 0), ink.text, font.Normal)
	place(ops, call, image.Pt(x, mid-ts.Y/2))
	if bw > 0 {
		bx := x + ts.X + gtx.Dp(10)
		bh, line := gtx.Dp(18), max(gtx.Dp(1), 1)
		b := image.Rect(bx, mid-bh/2, bx+bw, mid-bh/2+bh)
		rad := gtx.Dp(5)
		paint.FillShape(ops, ink.line, clip.UniformRRect(b, rad).Op(ops))
		fill := ink.bg
		if selected {
			fill = ink.sel
		}
		paint.FillShape(ops, fill, clip.UniformRRect(b.Inset(line), max(rad-line, 0)).Op(ops))
		place(ops, badge, image.Pt(b.Min.X+(bw-bs.X)/2, mid-bs.Y/2))
		h.badges = append(h.badges, b)
	}
}

// palSkillRow: the name in mono, its plugin prefix dim, and the description
// right-aligned in at most 56% of the row, 16 dp from the name.
func (c *chrome) palSkillRow(gtx layout.Context, it palItem, in image.Rectangle, ink *palInk) {
	ops := gtx.Ops
	mid := in.Min.Y + in.Dy()/2
	x, right := in.Min.X, in.Max.X
	gap := gtx.Dp(16)
	ink.note("skill name", ink.goInk, ink.under, 4.5)
	_, nsz, name := c.fitSideText(gtx, it.Name, 13.5, right-x, ink.goInk, font.Normal)
	if it.Detail != "" {
		room := min(in.Dx()*56/100, right-x-nsz.X-gap)
		if floor := in.Dx() * 30 / 100; room < floor {
			room = min(floor, in.Dx()*56/100)
		}
		ink.note("skill description", ink.muted, ink.under, 4.5)
		call, ds := c.fitSideW(gtx, it.Detail, 12.5, max(room, 0), ink.muted, font.Normal)
		if ds.X > 0 {
			place(ops, call, image.Pt(right-ds.X, mid-ds.Y/2))
			right -= ds.X + gap
		}
	}
	if x+nsz.X > right {
		_, nsz, name = c.fitSideText(gtx, it.Name, 13.5, max(right-x, 0), ink.goInk, font.Normal)
	}
	dim := min(it.Dim, len(name))
	if dim < len(name) && !utf8.RuneStart(name[dim]) {
		dim = len(name) // the cut fell inside the prefix
	}
	if dim > 0 {
		ink.note("skill prefix", ink.faint, ink.under, 4.5)
		call, ps := c.label(gtx, name[:dim], 13.5, ink.faint)
		place(ops, call, image.Pt(x, mid-ps.Y/2))
		x += ps.X
	}
	if dim < len(name) {
		call, rs := c.label(gtx, name[dim:], 13.5, ink.goInk)
		place(ops, call, image.Pt(x, mid-rs.Y/2))
	}
}

// palTile is the 22 dp tile at the start of action, browse and more rows.
func (c *chrome) palTile(gtx layout.Context, x, mid int, bg, line color.NRGBA) image.Rectangle {
	ts := gtx.Dp(22)
	tile := image.Rect(x, mid-ts/2, x+ts, mid-ts/2+ts)
	rad := gtx.Dp(6)
	if line != bg {
		paint.FillShape(gtx.Ops, line, clip.UniformRRect(tile, rad).Op(gtx.Ops))
		lw := max(gtx.Dp(1), 1)
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(tile.Inset(lw), max(rad-lw, 0)).Op(gtx.Ops))
	} else {
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(tile, rad).Op(gtx.Ops))
	}
	return tile
}

// palActionRow: the icon tile (the slot kept when there is no icon, Ruling
// 9), the name, and the shortcut as keycaps at the right when they fit.
func (c *chrome) palActionRow(gtx layout.Context, it palItem, in image.Rectangle, ink *palInk, h *palHits) {
	ops := gtx.Ops
	mid := in.Min.Y + in.Dy()/2
	x, right := in.Min.X, in.Max.X
	ts := gtx.Dp(22)
	if x+ts > right {
		return
	}
	if it.Icon != "" {
		tile := c.palTile(gtx, x, mid, ink.raised, ink.line)
		ink.note("action icon", ink.muted, ink.raised, 3)
		is := gtx.Dp(14)
		c.icon(gtx, it.Icon, image.Rect(tile.Min.X+(ts-is)/2, tile.Min.Y+(ts-is)/2, tile.Min.X+(ts-is)/2+is, tile.Min.Y+(ts-is)/2+is), ink.muted)
	}
	x += ts + gtx.Dp(10)
	if len(it.Keys) > 0 {
		kw := 0
		for i, k := range it.Keys {
			if i > 0 {
				kw += gtx.Dp(3)
			}
			kw += c.palKeycapW(gtx, k, ink)
		}
		if right-kw >= x+gtx.Dp(60) {
			kx := right - kw
			kh := gtx.Dp(17) + 3*max(gtx.Dp(1), 1)
			for i, k := range it.Keys {
				if i > 0 {
					kx += gtx.Dp(3)
				}
				w := c.palKeycap(gtx, k, kx, mid, ink)
				h.badges = append(h.badges, image.Rect(kx, mid-kh/2, kx+w, mid-kh/2+kh))
				kx += w
			}
			right -= kw + gtx.Dp(12)
		}
	}
	ink.note("action name", ink.text, ink.under, 4.5)
	call, ns := c.fitSideW(gtx, it.Name, 13.5, max(right-x, 0), ink.text, font.Normal)
	place(ops, call, image.Pt(x, mid-ns.Y/2))
}

const palBrowsePrefix = "Browse all "

// palBrowseRow: Browse and the more rows: a "/" or "+" tile, then the text
// in muted, Browse's count in the accent.
func (c *chrome) palBrowseRow(gtx layout.Context, it palItem, in image.Rectangle, ink *palInk) {
	ops := gtx.Ops
	mid := in.Min.Y + in.Dy()/2
	x, right := in.Min.X, in.Max.X
	ts := gtx.Dp(22)
	if x+ts > right {
		return
	}
	tile := c.palTile(gtx, x, mid, ink.tileBG, ink.tileBG)
	glyph := "+"
	if it.Kind == palBrowse {
		glyph = "/"
	}
	ink.note("browse tile", ink.tileFG, ink.tileBG, 4.5)
	call, gs := c.labelW(gtx, glyph, 12, ink.tileFG, font.Bold)
	place(ops, call, image.Pt(tile.Min.X+(ts-gs.X)/2, tile.Min.Y+(ts-gs.Y)/2))
	x += ts + gtx.Dp(10)
	lead, count := it.Name, ""
	if it.Kind == palBrowse && strings.HasPrefix(it.Name, palBrowsePrefix) {
		lead, count = palBrowsePrefix, it.Name[len(palBrowsePrefix):]
	}
	ink.note("browse text", ink.muted, ink.under, 4.5)
	call, ls := c.fitSideW(gtx, lead, 13.5, max(right-x, 0), ink.muted, font.Normal)
	place(ops, call, image.Pt(x, mid-ls.Y/2))
	if count != "" {
		ink.note("browse count", ink.goInk, ink.under, 4.5)
		call, cs := c.fitSideW(gtx, count, 13.5, max(right-x-ls.X, 0), ink.goInk, font.SemiBold)
		place(ops, call, image.Pt(x+ls.X, mid-cs.Y/2))
	}
}
