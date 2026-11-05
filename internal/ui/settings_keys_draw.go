package ui

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/render"
)

// The Shortcuts page's rows, messages and reset controls
// (docs/design/shortcuts-edit-mockup.html), in Settings' own tokens.

// shortcutInk is the Shortcuts page's colours for one theme: every text nudged
// to at least 4.5:1 on what it sits on, every mark at least 3:1 on the card.
type shortcutInk struct {
	label, action, none, noneFocus, capText, capFace, capEdge, tagText, tagBG,
	btnFocusLine, btnFocusBG, recRow, recLabel, recAction, recLine, recBG, recText, recDot,
	resetIcon, card color.NRGBA
	strip                                         [4]struct{ bg, line, text color.NRGBA } // by msgKind; [msgNone] unused
	btnText, btnBG, btnLine, strongText, strongBG color.NRGBA
}

// flatten is c (with its alpha) laid over the opaque bg.
func flatten(c, bg color.NRGBA) color.NRGBA {
	a := float32(c.A) / 255
	l := func(x, y uint8) uint8 { return uint8(float32(y) + (float32(x)-float32(y))*a + 0.5) }
	return color.NRGBA{R: l(c.R, bg.R), G: l(c.G, bg.G), B: l(c.B, bg.B), A: 0xff}
}

// The strips' fill and border alphas (Ruling 7).
const stripFill, stripLine = 0x1a, 0x59

// recRowFill is the recording row's accent tint, about 8% as in the mockup.
const recRowFill = 0x14

func (c *chrome) shortcutInk() shortcutInk {
	col := c.pageColors()
	card := col.card
	on := render.Readable
	k := shortcutInk{card: card, capFace: col.raised, capEdge: col.line2, btnFocusLine: col.line, btnFocusBG: c.th.CapHover}
	k.label = on(col.title, card)
	k.action = on(col.dim, card)
	k.none = on(col.muted, card)
	k.noneFocus = on(col.muted, k.btnFocusBG) // "None" on a focused button
	k.capText = on(col.title, col.raised)
	k.tagBG = flatten(tint(col.accent, 0x24), card)
	k.tagText = on(col.accent, k.tagBG)
	k.recRow = flatten(tint(col.accent, recRowFill), card) // the mockup's .row.rec
	k.recLabel = on(col.title, k.recRow)
	k.recAction = on(col.dim, k.recRow)
	k.recBG = flatten(tint(col.accent, 0x1f), k.recRow)
	k.recText = on(col.accent, k.recBG)
	k.recLine = markInk(col.accent, 0xff, card)
	k.recDot = k.recLine
	k.resetIcon = on(col.muted, card)
	for kind, tok := range map[msgKind]color.NRGBA{msgRefused: c.th.DiffDelFG, msgClash: col.accent, msgWarn: col.needs} {
		s := &k.strip[kind]
		s.bg = flatten(tint(tok, stripFill), card)
		s.line = markInk(tok, stripLine, card)
		s.text = on(col.title, s.bg)
	}
	k.btnBG, k.btnLine = col.raised, col.line2
	k.btnText = on(col.title, col.raised)
	k.strongBG = col.accent
	k.strongText = on(col.nav, col.accent)
	return k
}

// markInk is tok laid over bg at alpha, the alpha raised in steps until it
// stands 3:1 off bg (Ruling 7 nudges alphas, never hexes); a token too faint
// even at full strength goes through Readable.
func markInk(tok color.NRGBA, alpha uint8, bg color.NRGBA) color.NRGBA {
	for a := int(alpha); a < 0xff+0x10; a += 0x10 {
		if m := flatten(tint(tok, uint8(min(a, 0xff))), bg); contrastRatio(m, bg) >= 3 {
			return m
		}
	}
	return render.Readable(tok, bg)
}

// contrastRatio is the WCAG contrast of two opaque colours.
func contrastRatio(a, b color.NRGBA) float64 {
	lum := func(c color.NRGBA) float64 {
		ch := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= 0.03928 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
	}
	l1, l2 := lum(a)+0.05, lum(b)+0.05
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return l1 / l2
}

// keyRowGeom is one shortcut row's layout this frame, for tests.
type keyRowGeom struct {
	action      string          // "" for a mouse gesture
	label, name image.Point     // where the label and the action name start
	keys, reset image.Rectangle // the keys button and the reset slot
	tint        image.Rectangle // the recording row's accent fill; empty on other rows
}

// keyStripGeom is the shown message strip's layout this frame, for tests.
type keyStripGeom struct {
	box        image.Rectangle
	textBottom int
}

// shortcutCard draws rows in one card: the label with its action name under
// it, the "changed" tag, the keys button and the reset slot; a row's message
// strip sits under it.
func (p *page) shortcutCard(r image.Rectangle, y int, rows []shortcutRow) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	ink := p.c.shortcutInk()
	rec := op.Record(ops)
	top := y
	padL, padR, gap := gtx.Dp(16), gtx.Dp(10), gtx.Dp(12)
	rs := gtx.Dp(28)
	for i, row := range rows {
		if i > 0 {
			paint.FillShape(ops, col.line, clip.Rect(image.Rect(r.Min.X, y, r.Max.X, y+1)).Op())
		}
		editable := row.idx >= 0
		editing := editable && p.st.keysErr == ""
		recording := editable && row.config == p.st.keyRec
		labelInk, actionInk := ink.label, ink.action
		if recording {
			labelInk, actionInk = ink.recLabel, ink.recAction
		}
		changed := editing && p.win.keyChanged(row.config)
		right := r.Max.X - padR
		n0 := len(p.st.texts)
		kc, ks, from := p.keysButton(row, ink, editing)
		to := len(p.st.controls)
		// The page reads label, action name, keys: the keys' texts go after the label's.
		keyTexts := append([]string(nil), p.st.texts[n0:]...)
		p.st.texts = p.st.texts[:n0]
		lx := r.Min.X + padL
		room := max(right-rs-gap-ks.X-gap-lx, 1) // the label's room left of the keys
		tagW := 0
		var tagC op.CallOp
		var tagS image.Point
		if changed {
			tagC, tagS = p.c.labelW(gtx, "changed", 10.5, ink.tagText, font.SemiBold)
			tagW = gtx.Dp(8) + tagS.X + 2*gtx.Dp(7)
		}
		// A label too long for its room wraps, the tag after its last line.
		var lcs []op.CallOp
		var lss []image.Point
		labelH, lastW := 0, 0
		for _, l := range p.wrap(row.label, 13, max(room-tagW, 1)) {
			c, s := p.text(l, 13, labelInk)
			lcs, lss = append(lcs, c), append(lss, s)
			labelH, lastW = labelH+s.Y, s.X
		}
		var nc op.CallOp
		var ns image.Point
		if editable {
			nc, ns = p.mono(row.config, 11.5, actionInk)
		}
		h := max(labelH+ns.Y+gtx.Dp(12), gtx.Dp(44), ks.Y+gtx.Dp(8))
		g := keyRowGeom{action: row.config}
		if recording {
			// Under the separator line; the card's round corners clip the first and last rows.
			g.tint = image.Rect(r.Min.X, y, r.Max.X, y+h)
			if i > 0 {
				g.tint.Min.Y++
			}
			var top, bottom int
			if i == 0 {
				top = gtx.Dp(12)
			}
			if i == len(rows)-1 {
				bottom = gtx.Dp(12)
			}
			paint.FillShape(ops, ink.recRow, clip.RRect{Rect: g.tint, NW: top, NE: top, SW: bottom, SE: bottom}.Op(ops))
		}
		ly := y + (h-labelH-ns.Y)/2
		g.label, g.name = image.Pt(lx, ly), image.Pt(lx, ly+labelH)
		for i, c := range lcs {
			place(ops, c, image.Pt(lx, ly))
			ly += lss[i].Y
		}
		if editable {
			place(ops, nc, g.name)
		}
		p.st.texts = append(p.st.texts, keyTexts...)

		g.reset = image.Rect(right-rs, y+(h-rs)/2, right, y+(h+rs)/2)
		at := image.Pt(right-rs-gap-ks.X, y+(h-ks.Y)/2)
		place(ops, kc, at)
		p.st.shift(from, to, at)
		g.keys = image.Rectangle{Min: at, Max: at.Add(ks)}

		if changed {
			p.st.texts = append(p.st.texts, "changed")
			last := lss[len(lss)-1]
			th := tagS.Y + gtx.Dp(2)
			tag := image.Rect(0, 0, tagS.X+2*gtx.Dp(7), th).Add(image.Pt(lx+lastW+gtx.Dp(8), g.name.Y-last.Y+(last.Y-th)/2))
			paint.FillShape(ops, ink.tagBG, clip.UniformRRect(tag, th/2).Op(ops))
			place(ops, tagC, tag.Min.Add(image.Pt(gtx.Dp(7), gtx.Dp(1))))
			p.resetButton(g.reset, row.idx, ink)
		}
		p.st.keyRows = append(p.st.keyRows, g)
		y += h
		if m := p.st.keyMsg; m.kind != msgNone && m.action == row.config && editable {
			y = p.keyStrip(r, y, m, row.idx, ink)
		}
	}
	call := rec.Stop()
	card := image.Rect(r.Min.X, top, r.Max.X, y)
	paint.FillShape(ops, ink.card, clip.UniformRRect(card, gtx.Dp(12)).Op(ops))
	call.Add(ops)
	stroke(ops, card, gtx.Dp(12), col.line)
	p.st.cards = append(p.st.cards, card)
	return y
}

// keysButton draws a row's keys button at the origin: caps, "None", the
// recording prompt or a mouse gesture's words, right-aligned inside. An
// editable row's button is a key_rec control, from the returned index on.
func (p *page) keysButton(row shortcutRow, ink shortcutInk, editing bool) (op.CallOp, image.Point, int) {
	gtx, ops := p.gtx, p.gtx.Ops
	from := len(p.st.controls)
	macro := op.Record(ops)
	padX, padY := gtx.Dp(8), gtx.Dp(5)
	recording := row.idx >= 0 && row.config == p.st.keyRec
	focused := row.idx >= 0 && len(p.st.controls) == p.st.focus
	type piece struct {
		call op.CallOp
		size image.Point
	}
	var parts []piece
	w, h, dot := 0, 0, 0
	add := func(call op.CallOp, size image.Point) {
		if len(parts) > 0 {
			w += gtx.Dp(4)
		}
		parts = append(parts, piece{call, size})
		w += size.X
		h = max(h, size.Y)
	}
	switch {
	case recording:
		dot = gtx.Dp(7)
		call, size := p.text(recordingText, 13, ink.recText)
		add(call, size)
		w += dot + gtx.Dp(8)
	case row.idx < 0:
		add(p.text(row.keys, 13, ink.none))
	case row.keys == "":
		none := ink.none
		if focused {
			none = ink.noneFocus
		}
		add(p.oblique("None", 13, none))
	default:
		for _, k := range capsOf(row.keys) {
			add(p.keyCap(k, ink))
		}
	}
	box := image.Rect(0, 0, max(w+2*padX, gtx.Dp(150)), h+2*padY)
	radius := gtx.Dp(8)
	switch {
	case recording:
		paint.FillShape(ops, ink.recBG, clip.UniformRRect(box, radius).Op(ops))
		stroke(ops, box, radius, ink.recLine)
	case focused:
		// No hover in Settings, so the button's hover look shows on keyboard focus (Ruling 5).
		paint.FillShape(ops, ink.btnFocusBG, clip.UniformRRect(box, radius).Op(ops))
		stroke(ops, box, radius, ink.btnFocusLine)
	}
	x := box.Max.X - padX - w
	if dot > 0 {
		cy := box.Min.Y + box.Dy()/2
		paint.FillShape(ops, ink.recDot, clip.Ellipse(image.Rect(x, cy-dot/2, x+dot, cy-dot/2+dot)).Op(ops))
		x += dot + gtx.Dp(8)
	}
	for _, pc := range parts {
		place(ops, pc.call, image.Pt(x, box.Min.Y+(box.Dy()-pc.size.Y)/2))
		x += pc.size.X + gtx.Dp(4)
	}
	if row.idx >= 0 {
		p.hit(ctlButton, box, "key_rec", row.idx, !editing)
	}
	return macro.Stop(), box.Size(), from
}

// capsOf splits "Ctrl+Shift+K" into its keys; "Ctrl++" ends in a "+" key.
func capsOf(keys string) []string {
	if strings.HasSuffix(keys, "++") {
		return append(strings.Split(strings.TrimSuffix(keys, "++"), "+"), "+")
	}
	return strings.Split(keys, "+")
}

// resetButton draws the ↺ button in slot, a key_reset control.
func (p *page) resetButton(slot image.Rectangle, idx int, ink shortcutInk) {
	gtx := p.gtx
	if len(p.st.controls) == p.st.focus {
		paint.FillShape(gtx.Ops, ink.btnFocusBG, clip.UniformRRect(slot, gtx.Dp(7)).Op(gtx.Ops))
		stroke(gtx.Ops, slot, gtx.Dp(7), ink.btnFocusLine)
	}
	in := (slot.Dx() - gtx.Dp(16)) / 2
	p.c.icon(gtx, "reset", slot.Inset(in), ink.resetIcon)
	p.hit(ctlButton, slot, "key_reset", idx, false)
}

// oblique is text slanted, for "None": the bundled JetBrains Mono has no italic,
// and a font file for one word would cost more than it gives.
func (p *page) oblique(s string, size unit.Sp, col color.NRGBA) (op.CallOp, image.Point) {
	call, sz := p.text(s, size, col)
	m := op.Record(p.gtx.Ops)
	mid := float32(sz.Y) / 2
	// x' = x - 0.2 (y - mid): a 11 degree slant about the middle of the line.
	t := op.Affine(f32.NewAffine2D(1, -0.2, 0.2*mid, 0, 1, 0)).Push(p.gtx.Ops)
	call.Add(p.gtx.Ops)
	t.Pop()
	return m.Stop(), sz
}

// keyCap draws one key as a small cap at the origin, a thicker edge at its bottom.
func (p *page) keyCap(k string, ink shortcutInk) (op.CallOp, image.Point) {
	gtx := p.gtx
	rec := op.Record(gtx.Ops)
	tc, ts := p.mono(k, 11.5, ink.capText)
	h := ts.Y + gtx.Dp(8)
	w := max(ts.X+gtx.Dp(14), gtx.Dp(26))
	box := image.Rect(0, 0, w, h)
	radius := gtx.Dp(6)
	paint.FillShape(gtx.Ops, ink.capEdge, clip.UniformRRect(box, radius).Op(gtx.Ops))
	face := image.Rect(0, 0, w, h-gtx.Dp(2))
	paint.FillShape(gtx.Ops, ink.capFace, clip.UniformRRect(face, radius).Op(gtx.Ops))
	stroke(gtx.Ops, box, radius, ink.capEdge)
	place(gtx.Ops, tc, image.Pt((w-ts.X)/2, (face.Dy()-ts.Y)/2))
	return rec.Stop(), box.Size()
}

// keyStrip draws the message under a shortcut row from y and returns where
// the next row starts: the text (its keys in SemiBold), then OK, or Cancel and
// Replace; under the text when the row is too narrow for both.
func (p *page) keyStrip(r image.Rectangle, y int, m keyMessage, idx int, ink shortcutInk) int {
	gtx, ops := p.gtx, p.gtx.Ops
	sk := ink.strip[m.kind]
	box := image.Rect(r.Min.X+gtx.Dp(16), y, r.Max.X-gtx.Dp(10), y)
	padX, padY, gap := gtx.Dp(12), gtx.Dp(10), gtx.Dp(10)
	type btn struct {
		id, text string
		strong   bool
	}
	btns := []btn{{"key_ok", "OK", false}}
	if m.kind == msgClash {
		btns = []btn{{"key_cancel", "Cancel", false}, {"key_replace", "Replace", true}}
	}
	type laid struct {
		call op.CallOp
		size image.Point
	}
	var bs []laid
	bw, bh := 0, 0
	for i, b := range btns {
		call, size := p.stripButton(b.text, b.strong, ink)
		bs = append(bs, laid{call, size})
		if i > 0 {
			bw += gtx.Dp(8)
		}
		bw += size.X
		bh = max(bh, size.Y)
	}
	inner := box.Dx() - 2*padX
	room := inner - bw - gap
	below := room < gtx.Dp(200) // spec 9: too narrow, so the buttons go under the text
	if below {
		room = inner
	}
	bold := ""
	if m.keys.Name != "" {
		bold = formatShortcut(runtime.GOOS, m.keys)
	}
	lines, th := p.richLines(m.text, bold, max(room, 1), sk.text)
	h := max(th, bh)
	if below {
		h = th + gap + bh
	}
	box.Max.Y = y + h + 2*padY
	radius := gtx.Dp(9)
	paint.FillShape(ops, sk.bg, clip.UniformRRect(box, radius).Op(ops))
	stroke(ops, box, radius, sk.line)
	ty := y + padY + (h-th)/2
	if below {
		ty = y + padY
	}
	for _, l := range lines {
		place(ops, l.call, image.Pt(box.Min.X+padX, ty))
		ty += l.size.Y
	}
	p.st.keyStrip = keyStripGeom{box: box, textBottom: ty}
	x, by := box.Max.X-padX-bw, y+padY+(h-bh)/2
	if below {
		x, by = box.Min.X+padX, ty+gap
	}
	for i, b := range bs {
		at := image.Pt(x, by+(bh-b.size.Y)/2)
		place(ops, b.call, at)
		p.hit(ctlButton, image.Rectangle{Min: at, Max: at.Add(b.size)}, btns[i].id, idx, false)
		x += b.size.X + gtx.Dp(8)
	}
	return box.Max.Y + gtx.Dp(10)
}

type richLine struct {
	call op.CallOp
	size image.Point
}

// richLines wraps s at spaces into lines no wider than w, drawing the first
// word that reads bold (trailing punctuation aside) in SemiBold. Each line goes
// into the page's texts whole.
func (p *page) richLines(s, bold string, w int, col color.NRGBA) ([]richLine, int) {
	gtx := p.gtx
	words := strings.Fields(s)
	boldAt := -1
	for i, wd := range words {
		if bold != "" && strings.TrimRight(wd, ".,?") == bold {
			boldAt = i
			break
		}
	}
	weight := func(i int) font.Weight {
		if i == boldAt {
			return font.SemiBold
		}
		return font.Normal
	}
	_, space := p.c.labelW(gtx, " ", 13, col, font.Normal)
	var lines []richLine
	total := 0
	for i := 0; i < len(words); {
		// Take words while they fit; a word wider than w gets its own line.
		j, x := i, 0
		for j < len(words) {
			_, sz := p.c.labelW(gtx, words[j], 13, col, weight(j))
			add := sz.X
			if j > i {
				add += space.X
			}
			if j > i && x+add > w {
				break
			}
			x += add
			j++
		}
		m := op.Record(gtx.Ops)
		lx, lh := 0, 0
		for k := i; k < j; k++ {
			if k > i {
				lx += space.X
			}
			call, sz := p.c.labelW(gtx, words[k], 13, col, weight(k))
			place(gtx.Ops, call, image.Pt(lx, 0))
			lx += sz.X
			lh = max(lh, sz.Y)
		}
		lines = append(lines, richLine{m.Stop(), image.Pt(lx, lh)})
		p.st.texts = append(p.st.texts, strings.Join(words[i:j], " "))
		total += lh
		i = j
	}
	return lines, total
}

// stripButton draws a strip's button at the origin: Cancel and OK raised with
// a border, Replace filled with the accent.
func (p *page) stripButton(text string, strong bool, ink shortcutInk) (op.CallOp, image.Point) {
	gtx := p.gtx
	fg := ink.btnText
	if strong {
		fg = ink.strongText
	}
	m := op.Record(gtx.Ops)
	tc, ts := p.textW(text, 12.5, fg, font.SemiBold)
	box := image.Rect(0, 0, ts.X+2*gtx.Dp(11), ts.Y+2*gtx.Dp(5))
	radius := gtx.Dp(7)
	if strong {
		paint.FillShape(gtx.Ops, ink.strongBG, clip.UniformRRect(box, radius).Op(gtx.Ops))
	} else {
		paint.FillShape(gtx.Ops, ink.btnBG, clip.UniformRRect(box, radius).Op(gtx.Ops))
		stroke(gtx.Ops, box, radius, ink.btnLine)
	}
	place(gtx.Ops, tc, image.Pt(gtx.Dp(11), gtx.Dp(5)))
	return m.Stop(), box.Size()
}
