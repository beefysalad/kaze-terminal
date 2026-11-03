package render

import (
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"os"
	"sync"

	xdraw "golang.org/x/image/draw"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/patrick-gogo/kaze-terminal/internal/colorfont"
)

//go:generate go run gen_emoji.go

// runeSpan is an inclusive range of code points.
type runeSpan struct{ lo, hi rune }

func inSpans(spans []runeSpan, r rune) bool {
	lo, hi := 0, len(spans)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if spans[m].hi < r {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo < len(spans) && spans[lo].lo <= r
}

// isEmojiPresentation reports whether r is drawn as an emoji by default (✅ 🔄 🚀).
func isEmojiPresentation(r rune) bool {
	return r >= 0x231A && inSpans(emojiPresentationSpans[:], r)
}

// emojiFont is the system colour emoji font. sf is set when its outlines are
// usable (Segoe UI Emoji); an sbix font (Apple Color Emoji) is drawn from its
// bitmaps only and looks runes up through cmap.
type emojiFont struct {
	sf   *sfnt.Font
	colr *colorfont.COLR
	pal  colorfont.Palette
	sbix *colorfont.Sbix
	cmap colorfont.Cmap
	src  io.ReaderAt
}

// glyph returns r's glyph id in the emoji font, 0 when absent (or its cmap panics).
func (e *emojiFont) glyph(b *sfnt.Buffer, r rune) (gid uint16) {
	if e.sf == nil {
		return e.cmap.Glyph(r)
	}
	defer func() {
		if recover() != nil {
			gid = 0
		}
	}()
	i, _ := e.sf.GlyphIndex(b, r)
	return uint16(i)
}

// outline reports whether glyphs can be drawn from outlines.
func (e *emojiFont) outline() bool { return e.sf != nil }

// emojiSource loads the emoji font at most once, on the first emoji rune.
type emojiSource struct {
	mu   sync.Mutex
	path string
	done bool
	f    *emojiFont
	open func(path string) *emojiFont // nil means loadEmojiFont; tests swap it
}

var emojiFonts = &emojiSource{path: emojiFontPath()}

func (s *emojiSource) font() *emojiFont {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.done = true
		open := s.open
		if open == nil {
			open = loadEmojiFont
		}
		s.f = open(s.path)
	}
	return s.f
}

// Table size limits: Segoe UI Emoji's COLR is about 0.2 MB, cmaps are far smaller.
const (
	maxColorTable = 8 << 20
	maxCmapTable  = 2 << 20
)

// loadEmojiFont opens path. Tables that fail to parse are left nil, so a broken
// colour table costs colour only; a font with nothing drawable, or a parser panic,
// gives nil and the old fallback order.
func loadEmojiFont(path string) *emojiFont {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	st, err := f.Stat()
	var e *emojiFont
	if err == nil {
		e = parseEmojiFont(f, st.Size())
	}
	if e == nil {
		f.Close()
	}
	return e
}

// parseEmojiFont reads an emoji font of size bytes from f, which it keeps.
func parseEmojiFont(f io.ReaderAt, size int64) (e *emojiFont) {
	defer func() {
		if recover() != nil {
			e = nil
		}
	}()
	d, err := colorfont.ReadDirectory(f, size, 0)
	if err != nil {
		return nil
	}
	maxp, err := d.Table(f, "maxp", 64)
	if err != nil {
		return nil
	}
	n, err := colorfont.NumGlyphs(maxp)
	if err != nil {
		return nil
	}
	e = &emojiFont{src: f}
	if rec, ok := d["sbix"]; ok {
		e.sbix, _ = colorfont.ParseSbix(f, rec, n)
		if b, err := d.Table(f, "cmap", maxCmapTable); err == nil {
			e.cmap, _ = colorfont.ParseCmap(b)
		}
	} else if c, err := sfnt.ParseCollectionReaderAt(f); err == nil {
		e.sf, _ = c.Font(0)
	}
	if e.sf != nil {
		colr, err1 := d.Table(f, "COLR", maxColorTable)
		cpal, err2 := d.Table(f, "CPAL", maxColorTable)
		if err1 == nil && err2 == nil {
			c, err1 := colorfont.ParseCOLR(colr, n)
			p, err2 := colorfont.ParseCPAL(cpal)
			if err1 == nil && err2 == nil {
				e.colr, e.pal = c, p
			}
		}
	}
	if e.sf == nil && e.sbix == nil {
		return nil
	}
	return e
}

// emojiDrawable reports whether the emoji font can draw r at this size: from its
// outline, or from a PNG in the strike nearest the cell height.
func (fs *fontSet) emojiDrawable(e *emojiFont, r rune) bool {
	gid := e.glyph(&fs.buf, r)
	if gid == 0 {
		return false
	}
	if e.outline() {
		return true
	}
	if e.sbix == nil {
		return false
	}
	st, _ := e.sbix.Nearest(fs.cellH)
	return e.sbix.Has(e.src, st, gid)
}

// emojiFace returns the emoji font's outline face at px, made once per size.
func (fs *fontSet) emojiFace(px float64) xfont.Face {
	if face, ok := fs.emojiFaces[px]; ok {
		return face
	}
	if fs.emoji == nil {
		return nil
	}
	e := fs.emoji.font()
	if e == nil || e.sf == nil {
		return nil
	}
	face, err := opentype.NewFace(e.sf, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: xfont.HintingFull})
	if err != nil {
		return nil
	}
	if fs.emojiFaces == nil || len(fs.emojiFaces) >= sysFaceCap {
		fs.emojiFaces = make(map[float64]xfont.Face)
	}
	fs.emojiFaces[px] = face
	return face
}

// textColorNeutral is drawn for palette index 0xFFFF ("text colour") and for indices
// past the palette: a fixed mid grey, readable on dark and light themes, so a colour
// glyph never depends on the cell's FG and stays one cached image.
const textColorNeutral = 0x9a9a9a

// layerColor is palette entry idx, or the neutral for TextColor and bad indices.
func layerColor(pal colorfont.Palette, idx uint16) color.NRGBA {
	if idx == colorfont.TextColor || int(idx) >= len(pal) {
		return color.NRGBA{R: textColorNeutral >> 16 & 0xff, G: textColorNeutral >> 8 & 0xff, B: textColorNeutral & 0xff, A: 0xff}
	}
	return pal[idx]
}

// colrBounds is the union of the layers' ink bounds at ppem, in pixels (y down).
func (fs *fontSet) colrBounds(sf *sfnt.Font, layers []colorfont.Layer, ppem fixed.Int26_6) (fixed.Rectangle26_6, bool) {
	var u fixed.Rectangle26_6
	any := false
	for _, l := range layers {
		b, _, err := sf.GlyphBounds(&fs.buf, sfnt.GlyphIndex(l.Glyph), ppem, xfont.HintingNone)
		if err != nil {
			return u, false
		}
		if b.Empty() {
			continue
		}
		if !any {
			u, any = b, true
		} else {
			u = u.Union(b)
		}
	}
	return u, any
}

// drawCOLR composites gid's COLR layers, each glyph outline filled in its palette
// colour, into a premultiplied w×cellH image, scaled to fit and centred. It returns
// nil on any error or panic; the caller then draws the outline.
func (fs *fontSet) drawCOLR(e *emojiFont, gid uint16, layers []colorfont.Layer, w int) (img *image.RGBA) {
	defer func() {
		if recover() != nil {
			img = nil
		}
	}()
	if e == nil || e.sf == nil || len(layers) == 0 {
		return nil
	}
	h := fs.cellH
	ppem := fixed.Int26_6(fs.px * 64)
	u, ok := fs.colrBounds(e.sf, layers, ppem)
	if !ok {
		return nil
	}
	gw, gh := float64(u.Max.X-u.Min.X)/64, float64(u.Max.Y-u.Min.Y)/64
	if gw > float64(w) || gh > float64(h) {
		s := 0.95 * min(float64(w)/max(gw, 1), float64(h)/max(gh, 1))
		ppem = fixed.Int26_6(float64(ppem) * s)
		if u, ok = fs.colrBounds(e.sf, layers, ppem); !ok {
			return nil
		}
	}
	dx := (float32(w) - float32(u.Min.X+u.Max.X)/64) / 2
	dy := (float32(h) - float32(u.Min.Y+u.Max.Y)/64) / 2

	img = image.NewRGBA(image.Rect(0, 0, w, h))
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	var z vector.Rasterizer
	pt := func(p fixed.Point26_6) (float32, float32) {
		return float32(p.X)/64 + dx, float32(p.Y)/64 + dy
	}
	for _, l := range layers {
		segs, err := e.sf.LoadGlyph(&fs.buf, sfnt.GlyphIndex(l.Glyph), ppem, nil)
		if err != nil {
			return nil
		}
		z.Reset(w, h)
		open := false
		for _, s := range segs {
			switch s.Op {
			case sfnt.SegmentOpMoveTo:
				if open {
					z.ClosePath()
				}
				z.MoveTo(pt(s.Args[0]))
				open = true
			case sfnt.SegmentOpLineTo:
				z.LineTo(pt(s.Args[0]))
			case sfnt.SegmentOpQuadTo:
				x1, y1 := pt(s.Args[0])
				x2, y2 := pt(s.Args[1])
				z.QuadTo(x1, y1, x2, y2)
			case sfnt.SegmentOpCubeTo:
				x1, y1 := pt(s.Args[0])
				x2, y2 := pt(s.Args[1])
				x3, y3 := pt(s.Args[2])
				z.CubeTo(x1, y1, x2, y2, x3, y3)
			}
		}
		if open {
			z.ClosePath()
		}
		clear(mask.Pix)
		z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
		draw.DrawMask(img, img.Bounds(), image.NewUniform(layerColor(e.pal, l.Palette)), image.Point{}, mask, image.Point{}, draw.Over)
	}
	return img
}

// drawColor draws r in colour from the emoji font, nil when it has no colour for r.
func (fs *fontSet) drawColor(r rune, w int) *image.RGBA {
	e := fs.emojiFont()
	if e == nil {
		return nil
	}
	gid := e.glyph(&fs.buf, r)
	if gid == 0 {
		return nil
	}
	if e.colr != nil {
		if layers := e.colr.Layers(gid); layers != nil {
			return fs.drawCOLR(e, gid, layers, w)
		}
	}
	if e.sbix != nil {
		return fs.drawSbix(e, gid, w)
	}
	return nil
}

// drawSbix draws gid's PNG from the strike nearest the cell height, scaled with its
// aspect kept into the largest rectangle that fits w×cellH, centred. The PNG may be at
// most twice the strike's size on each side. It returns nil on any error or panic.
func (fs *fontSet) drawSbix(e *emojiFont, gid uint16, w int) (img *image.RGBA) {
	defer func() {
		if recover() != nil {
			img = nil
		}
	}()
	st, ppem := e.sbix.Nearest(fs.cellH)
	data, err := e.sbix.PNG(e.src, st, gid)
	if err != nil {
		return nil
	}
	src, err := colorfont.DecodePNG(data, 2*ppem)
	if err != nil {
		return nil
	}
	h := fs.cellH
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	s := min(float64(w)/float64(sw), float64(h)/float64(sh))
	dw, dh := max(int(math.Round(float64(sw)*s)), 1), max(int(math.Round(float64(sh)*s)), 1)
	x, y := (w-dw)/2, (h-dh)/2
	img = image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(img, image.Rect(x, y, x+dw, y+dh), src, src.Bounds(), xdraw.Src, nil)
	return img
}
