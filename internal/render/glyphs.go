package render

import (
	"image"
	"sync"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// glyphCap bounds the cache; past it the cache is cleared and refilled from what is on screen.
const glyphCap = 4096

// notdefKey is the one key every missing rune maps to.
const notdefKey rune = -1

// notdefProbe is a noncharacter no font maps, so drawing it always yields .notdef.
const notdefProbe rune = 0x10FFFF

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

// glyphCache is shared by every renderer of one pixel size; renderers use it from
// the UI goroutine only.
type glyphCache struct {
	fs *fontSet
	m  map[glyphKey]*glyph
}

var shared struct {
	sync.Mutex
	bySize map[float32]*glyphCache
}

// sharedGlyphs returns the font set and glyph cache for px, so tabs don't each parse
// fonts and rasterize the same glyphs.
func sharedGlyphs(px float32) *glyphCache {
	shared.Lock()
	defer shared.Unlock()
	if c, ok := shared.bySize[px]; ok {
		return c
	}
	if shared.bySize == nil {
		shared.bySize = make(map[float32]*glyphCache)
	}
	c := &glyphCache{fs: newFontSet(float64(px)), m: make(map[glyphKey]*glyph)}
	shared.bySize[px] = c
	return c
}

func (c *glyphCache) len() int { return len(c.m) }

// get returns the cached mask, rasterizing it on first use.
func (c *glyphCache) get(r rune, bold, wide bool) *glyph {
	fs := c.fs
	var id faceID
	switch {
	case isBoxRune(r):
		id = faceBox
	case r > ' ' && r < 0x7f: // ASCII is always in the mono font; skip the cmap lookups
		id = faceMono
		if bold {
			id = faceMonoBold
		}
	default:
		id = fs.faceFor(r, bold)
	}
	if id != faceMono && id != faceMonoBold {
		bold = false // only the mono font has a bold face
	}
	k := glyphKey{r, bold, wide}
	if id == faceMissing {
		k.r = notdefKey
	}
	if g, ok := c.m[k]; ok {
		return g
	}
	if len(c.m) >= glyphCap {
		clear(c.m)
	}

	cells := 1
	if wide {
		cells = 2
	}
	w := cells * fs.cellW
	var mask *image.Alpha
	if id == faceBox {
		mask = drawBox(r, w, fs.cellH, fs.cellW)
	} else {
		mask = image.NewAlpha(image.Rect(0, 0, w, fs.cellH))
		face, draw := fs.faces[faceMono], r
		switch id {
		case faceMonoBold, faceCJK:
			face = fs.faces[id]
		case faceMissing:
			draw = notdefProbe
		}
		// Center the glyph in its span so a CJK glyph narrower than two cells sits evenly.
		x := 0
		if adv, ok := face.GlyphAdvance(draw); ok && adv.Ceil() < w {
			x = (w - adv.Ceil()) / 2
		}
		d := xfont.Drawer{Dst: mask, Src: image.Opaque, Face: face, Dot: fixed.P(x, fs.ascent)}
		d.DrawString(string(draw))
	}

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
