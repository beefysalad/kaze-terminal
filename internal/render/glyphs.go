package render

import (
	"image"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type glyphKey struct {
	r    rune
	bold bool
	wide bool
}

// glyph is a coverage mask the size of its cell span; colors are applied at copy time,
// so one rasterization serves every color combination.
type glyph struct {
	mask  *image.Alpha
	face  faceID
	empty bool // no ink at all: the copy can skip the mask
}

type glyphCache struct {
	fs *fontSet
	m  map[glyphKey]*glyph
}

func newGlyphCache(fs *fontSet) *glyphCache {
	return &glyphCache{fs: fs, m: make(map[glyphKey]*glyph)}
}

func (c *glyphCache) len() int { return len(c.m) }

// get returns the cached mask, rasterizing it on first use.
func (c *glyphCache) get(r rune, bold, wide bool) *glyph {
	k := glyphKey{r, bold, wide}
	if g, ok := c.m[k]; ok {
		return g
	}
	fs := c.fs
	cells := 1
	if wide {
		cells = 2
	}
	w := cells * fs.cellW
	id := fs.faceFor(r, bold)
	face := fs.faces[id]
	mask := image.NewAlpha(image.Rect(0, 0, w, fs.cellH))

	// Center the glyph in its span so a CJK glyph narrower than two cells sits evenly.
	x := 0
	if adv, ok := face.GlyphAdvance(r); ok && adv.Ceil() < w {
		x = (w - adv.Ceil()) / 2
	}
	d := xfont.Drawer{Dst: mask, Src: image.Opaque, Face: face, Dot: fixed.P(x, fs.ascent)}
	d.DrawString(string(r))

	g := &glyph{mask: mask, face: id, empty: true}
	for _, p := range mask.Pix {
		if p != 0 {
			g.empty = false
			break
		}
	}
	c.m[k] = g
	return g
}
