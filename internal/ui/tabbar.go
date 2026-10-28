package ui

import (
	"image"
	"image/color"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/patrick-gogo/kaze-terminal/assets/fonts"
	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// Midnight chrome tokens (design spec 6.7).
var (
	colChrome   = rgb(0x0b0d12)
	colActive   = rgb(0x0f1117)
	colActiveFG = rgb(0xffffff)
	colMuted    = rgb(0x8a90a2)
	colBorder   = rgb(0x1d2230)
	colAccent   = rgb(0x8b7bff)
	colStatusFG = rgb(0x7d8394)
	colFolder   = rgb(0xc9cdd8)
	colBranch   = rgb(0x62d39a)
)

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

const chromeFace = "JetBrains Mono"

// chrome draws the tab bar and status bar with Gio's shaper over the bundled fonts.
type chrome struct {
	shaper    *text.Shaper
	hits      barHits
	toastHits []toastHit

	faces   []font.FontFace
	covered map[rune]bool   // rune -> some face has it
	loaded  map[string]bool // system font files already added
}

func newChrome() *chrome {
	var faces []font.FontFace
	for _, src := range []struct {
		data []byte
		name string
	}{{fonts.MonoRegular, chromeFace}, {fonts.CJK, "Noto Sans JP"}} {
		f, err := opentype.Parse(src.data)
		if err != nil {
			panic("ui: bundled font does not parse: " + err.Error()) // embedded at build time
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: font.Typeface(src.name)}, Face: f})
	}
	return &chrome{
		shaper:  text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces)),
		faces:   faces,
		covered: map[rune]bool{},
		loaded:  map[string]bool{},
	}
}

// fit makes s drawable: for each rune the bundled fonts lack, it adds the system font
// the terminal grid would use (once, only now), and drops runes no font has so a title never shows a box.
func (c *chrome) fit(s string) string {
	added := false
	var out *strings.Builder // nil until a rune is changed or dropped
	for i, r := range s {
		keep, w := true, r
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			w = ' '
		case !c.usable(r, &added):
			keep = false
		}
		if out == nil && (!keep || w != r) {
			out = &strings.Builder{}
			out.WriteString(s[:i])
		}
		if out != nil && keep {
			out.WriteRune(w)
		}
	}
	if added {
		c.shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(c.faces))
	}
	if out == nil {
		return s
	}
	return out.String()
}

// maxCovered bounds the rune cache; a hostile title stream cannot grow it forever.
const maxCovered = 4096

// maxFontFile keeps big collections and emoji fonts out of memory; their runes are trimmed.
var maxFontFile int64 = 8 << 20

// usable reports whether r can be drawn, loading its system font on first need.
func (c *chrome) usable(r rune, added *bool) bool {
	if ok, seen := c.covered[r]; seen {
		return ok
	}
	ok := false
	// Cf covers bidi overrides and zero-width marks; Zl/Zp would break the one-line label.
	if !unicode.IsControl(r) && !unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
		ok = render.BundledCovers(r)
		if !ok {
			if p := render.SystemFontFor(r); p != "" {
				var fresh bool
				ok, fresh = c.addFont(p)
				*added = *added || fresh
			}
		}
	}
	if len(c.covered) >= maxCovered {
		clear(c.covered)
	}
	c.covered[r] = ok
	return ok
}

// addFont parses a system font file into the collection. fresh is true only when this call
// added faces; a bad or oversized file adds nothing and is not retried.
func (c *chrome) addFont(path string) (ok, fresh bool) {
	if done, seen := c.loaded[path]; seen {
		return done, false
	}
	defer func() {
		if recover() != nil {
			ok, fresh = false, false
		}
		c.loaded[path] = ok
	}()
	if st, err := os.Stat(path); err != nil || st.Size() > maxFontFile {
		return false, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	ff, err := opentype.ParseCollection(data)
	if err != nil || len(ff) == 0 {
		return false, false
	}
	c.faces = append(c.faces, ff...)
	return true, true
}

// label records one line of text so its size is known before it is placed.
func (c *chrome) label(gtx layout.Context, s string, size unit.Sp, col color.NRGBA) (op.CallOp, image.Point) {
	s = c.fit(s)
	m := op.Record(gtx.Ops)
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	matc := mat.Stop()
	lg := gtx
	lg.Constraints = layout.Constraints{Max: image.Pt(1<<16, 1<<16)}
	d := widget.Label{MaxLines: 1}.Layout(lg, c.shaper, font.Font{Typeface: chromeFace}, size, s, matc)
	return m.Stop(), d.Size
}

func place(ops *op.Ops, call op.CallOp, at image.Point) {
	t := op.Offset(at).Push(ops)
	call.Add(ops)
	t.Pop()
}

// layoutTabBar fills the bar's background, draws the tabs and the + button left of limit,
// records where each one is for clicks, and returns where the strip ends.
// marks gives each tab's alert mark; nil means none.
func (c *chrome) layoutTabBar(gtx layout.Context, list []*tabs.Tab, active tabs.TabID, marks func(tabs.TabID) mark, def string, height, limit int) int {
	ops := gtx.Ops
	width := gtx.Constraints.Max.X
	paint.FillShape(ops, colChrome, clip.Rect{Max: image.Pt(width, height)}.Op())

	top := gtx.Dp(7)
	padX, gap, dot, radius := gtx.Dp(12), gtx.Dp(7), gtx.Dp(7), gtx.Dp(9)
	x := gtx.Dp(8)
	c.hits = barHits{tabs: c.hits.tabs[:0]}

	closeCall, closeSize := c.label(gtx, "×", 12, colMuted)
	plusCall, plusSize := c.label(gtx, "+", 13, colMuted)
	for _, t := range list {
		on := t.ID == active
		fg := colMuted
		if on {
			fg = colActiveFG
		}
		call, size := c.label(gtx, c.tabLabelFit(t, def), 12, fg)
		var mk mark
		if marks != nil {
			mk = marks(t.ID)
		}
		dc, hasDot := dotColor(on, mk)
		w := padX + size.X + gap + closeSize.X + padX
		if hasDot {
			w += dot + gap
		}
		if x+w > limit-plusSize.X-2*padX {
			break // overflow: the rest stay reachable with next/prev tab
		}
		r := image.Rect(x, top, x+w, height)
		if on {
			paint.FillShape(ops, colActive, clip.RRect{Rect: r, NW: radius, NE: radius}.Op(ops))
		}
		cx := x + padX
		mid := top + (height-top)/2
		if hasDot {
			d := image.Rect(cx, mid-dot/2, cx+dot, mid-dot/2+dot)
			paint.FillShape(ops, dc, clip.Ellipse(d).Op(ops))
			cx += dot + gap
		}
		place(ops, call, image.Pt(cx, mid-size.Y/2))
		cx += size.X + gap
		place(ops, closeCall, image.Pt(cx, mid-closeSize.Y/2))
		hitPad := gtx.Dp(4)
		closeRect := image.Rect(cx-hitPad, mid-closeSize.Y/2-hitPad, cx+closeSize.X+hitPad, mid+closeSize.Y/2+hitPad)
		c.hits.tabs = append(c.hits.tabs, tabHit{id: t.ID, rect: r, close: closeRect})
		x += w + gtx.Dp(4)
	}
	pr := image.Rect(x, top, x+plusSize.X+2*padX, height)
	if pr.Max.X > limit {
		return min(x, limit) // too narrow for +; the new-tab shortcut still works
	}
	place(ops, plusCall, image.Pt(x+padX, top+(height-top-plusSize.Y)/2))
	c.hits.plus = pr
	return pr.Max.X
}

// Caption button colors (design spec 6.7).
var (
	colCapGlyph   = rgb(0xa7adbd)
	colCapHover   = rgb(0x1a1f2c)
	colCapPress   = rgb(0x151925)
	colCloseHover = rgb(0xe5484d)
	colClosePress = rgb(0xc93c40)
)

// layoutCaptions draws the minimize, maximize/restore and close buttons.
func (c *chrome) layoutCaptions(gtx layout.Context, b barLayout, st captionState, maximized bool) {
	ops := gtx.Ops
	for i, r := range b.Buttons {
		if r.Empty() {
			continue
		}
		btn := captionButton(i + 1)
		fg := colCapGlyph
		if bg, ok := captionBG(btn, st); ok {
			paint.FillShape(ops, bg, clip.Rect(r).Op())
			fg = colActiveFG
		}
		cl := clip.Rect(r).Push(ops)
		paint.FillShape(ops, fg, captionGlyph(gtx, btn, r, maximized))
		cl.Pop()
	}
}

// captionBG is a button's background: pressed while held over it, hover when only hovered.
func captionBG(btn captionButton, st captionState) (color.NRGBA, bool) {
	hover, press := colCapHover, colCapPress
	if btn == capClose {
		hover, press = colCloseHover, colClosePress
	}
	switch {
	case st.pressed == btn && st.hover == btn:
		return press, true
	case st.pressed == capNone && st.hover == btn:
		return hover, true
	}
	return color.NRGBA{}, false
}

// captionGlyph is the 10dp line drawing for a button, centred in r. Lines sit half a
// stroke inside their box so a 1px stroke lands on whole pixels.
func captionGlyph(gtx layout.Context, btn captionButton, r image.Rectangle, maximized bool) clip.Op {
	sw := float32(max(gtx.Dp(1), 1))
	n := gtx.Dp(10)
	x0 := float32(r.Min.X + (r.Dx()-n)/2)
	y0 := float32(r.Min.Y + (r.Dy()-n)/2)
	s, h, d := float32(n), sw/2, float32(gtx.Dp(2))
	var p clip.Path
	p.Begin(gtx.Ops)
	box := func(x1, y1, x2, y2 float32) {
		p.MoveTo(f32.Pt(x1+h, y1+h))
		p.LineTo(f32.Pt(x2-h, y1+h))
		p.LineTo(f32.Pt(x2-h, y2-h))
		p.LineTo(f32.Pt(x1+h, y2-h))
		p.Close()
	}
	switch {
	case btn == capMinimize:
		y := y0 + float32(n/2) + h
		p.MoveTo(f32.Pt(x0, y))
		p.LineTo(f32.Pt(x0+s, y))
	case btn == capMaximize && !maximized:
		box(x0, y0, x0+s, y0+s)
	case btn == capMaximize:
		box(x0, y0+d, x0+s-d, y0+s) // front window
		// Back window: only its top and right edges show.
		p.MoveTo(f32.Pt(x0+d+h, y0+d))
		p.LineTo(f32.Pt(x0+d+h, y0+h))
		p.LineTo(f32.Pt(x0+s-h, y0+h))
		p.LineTo(f32.Pt(x0+s-h, y0+s-d-h))
		p.LineTo(f32.Pt(x0+s-d, y0+s-d-h))
	case btn == capClose:
		p.MoveTo(f32.Pt(x0, y0))
		p.LineTo(f32.Pt(x0+s, y0+s))
		p.MoveTo(f32.Pt(x0+s, y0))
		p.LineTo(f32.Pt(x0, y0+s))
	}
	return clip.Stroke{Path: p.End(), Width: sw}.Op()
}

// layoutStatus draws the status bar: folder and branch on the left, the rest on the right.
func (c *chrome) layoutStatus(gtx layout.Context, p statusParts, height int) {
	ops := gtx.Ops
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(14)
	paint.FillShape(ops, colChrome, clip.Rect{Max: image.Pt(width, height)}.Op())
	paint.FillShape(ops, colBorder, clip.Rect{Max: image.Pt(width, 1)}.Op())
	call, size := c.label(gtx, p.Right, 11, colStatusFG)
	place(ops, call, image.Pt(width-size.X-pad, (height-size.Y)/2))

	avail := width - size.X - 3*pad // room left of the right text, with a gap
	x := pad
	var bcall op.CallOp
	var bsize image.Point
	gap := gtx.Dp(12)
	if p.Branch != "" {
		bcall, bsize = c.label(gtx, c.branchLabel(p.Branch), 11, colBranch)
		avail -= bsize.X + gap
	}
	if p.Folder != "" && avail > 0 {
		folder := p.Folder
		fcall, fsize := c.label(gtx, folder, 11, colFolder)
		// Shorten from the left until it fits; bounded so a bad measure cannot spin.
		for i := 0; i < 8 && fsize.X > avail && folder != "…"; i++ {
			n := utf8.RuneCountInString(folder) * avail / fsize.X
			if n >= utf8.RuneCountInString(folder) {
				n = utf8.RuneCountInString(folder) - 1
			}
			folder = truncateLeft(folder, n)
			fcall, fsize = c.label(gtx, folder, 11, colFolder)
		}
		place(ops, fcall, image.Pt(x, (height-fsize.Y)/2))
		x += fsize.X + gap
	}
	if p.Branch != "" && x+bsize.X <= width-size.X-pad {
		place(ops, bcall, image.Pt(x, (height-bsize.Y)/2))
	}
}

// tabLabelFit is tabLabel with the title cleaned for the tab bar's fonts before it is shortened.
func (c *chrome) tabLabelFit(t *tabs.Tab, def string) string {
	return tabLabelWith(t, def, c.fit)
}
