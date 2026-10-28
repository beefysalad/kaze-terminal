package ui

import (
	"image"
	"image/color"
	"os"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/patrick-gogo/kaze/assets/fonts"
	"github.com/patrick-gogo/kaze/internal/render"
	"github.com/patrick-gogo/kaze/internal/tabs"
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
)

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

const chromeFace = "JetBrains Mono"

// chrome draws the tab bar and status bar with Gio's shaper over the bundled fonts.
type chrome struct {
	shaper *text.Shaper
	hits   barHits

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
const maxFontFile = 8 << 20

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

// layoutTabBar draws the tabs across a bar of the given height and records where each one is for clicks.
func (c *chrome) layoutTabBar(gtx layout.Context, list []*tabs.Tab, active tabs.TabID, def string, height int) {
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
		w := padX + size.X + gap + closeSize.X + padX
		if on {
			w += dot + gap
		}
		if x+w > width-plusSize.X-2*padX {
			break // overflow: the rest stay reachable with next/prev tab
		}
		r := image.Rect(x, top, x+w, height)
		if on {
			paint.FillShape(ops, colActive, clip.RRect{Rect: r, NW: radius, NE: radius}.Op(ops))
		}
		cx := x + padX
		mid := top + (height-top)/2
		if on {
			d := image.Rect(cx, mid-dot/2, cx+dot, mid-dot/2+dot)
			paint.FillShape(ops, colAccent, clip.Ellipse(d).Op(ops))
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
	place(ops, plusCall, image.Pt(x+padX, top+(height-top-plusSize.Y)/2))
	c.hits.plus = pr
}

// layoutStatus draws the status bar with its text on the right.
func (c *chrome) layoutStatus(gtx layout.Context, s string, height int) {
	ops := gtx.Ops
	width := gtx.Constraints.Max.X
	paint.FillShape(ops, colChrome, clip.Rect{Max: image.Pt(width, height)}.Op())
	paint.FillShape(ops, colBorder, clip.Rect{Max: image.Pt(width, 1)}.Op())
	call, size := c.label(gtx, s, 11, colStatusFG)
	place(ops, call, image.Pt(width-size.X-gtx.Dp(14), (height-size.Y)/2))
}

// tabLabelFit is tabLabel with the title cleaned for the tab bar's fonts before it is shortened.
func (c *chrome) tabLabelFit(t *tabs.Tab, def string) string {
	return tabLabelWith(t, def, c.fit)
}
