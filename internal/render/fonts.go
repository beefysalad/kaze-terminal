package render

import (
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/patrick-gogo/kaze/assets/fonts"
)

type faceID uint8

const (
	faceMono faceID = iota
	faceMonoBold
	faceCJK
	faceMissing // in neither font: every such rune shares one .notdef mask
	faceBox     // box drawing and block elements, drawn procedurally
)

// fontSet holds the parsed bundled fonts at one pixel size.
type fontSet struct {
	mono, cjk *sfnt.Font
	faces     [3]xfont.Face
	buf       sfnt.Buffer

	cellW, cellH int
	ascent       int // baseline offset from the top of a cell
	underlineY   int
	strikeY      int
}

func mustParse(b []byte) *sfnt.Font {
	f, err := opentype.Parse(b)
	if err != nil {
		panic("render: bundled font does not parse: " + err.Error()) // embedded at build time, so this is a build bug
	}
	return f
}

func mustFace(f *sfnt.Font, px float64) xfont.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: xfont.HintingFull})
	if err != nil {
		panic("render: " + err.Error())
	}
	return face
}

// newFontSet sizes everything from the mono font; px is the font size in pixels.
func newFontSet(px float64) *fontSet {
	fs := &fontSet{mono: mustParse(fonts.MonoRegular), cjk: mustParse(fonts.CJK)}
	fs.faces[faceMono] = mustFace(fs.mono, px)
	fs.faces[faceMonoBold] = mustFace(mustParse(fonts.MonoBold), px)

	m := fs.faces[faceMono].Metrics()
	adv, _ := fs.faces[faceMono].GlyphAdvance('M')
	fs.cellW = max(adv.Ceil(), 1)
	fs.cellH = max((m.Ascent + m.Descent).Ceil(), 1)
	fs.ascent = m.Ascent.Ceil()
	fs.underlineY = min(fs.ascent+1, fs.cellH-1)
	fs.strikeY = max(fs.ascent-m.XHeight.Round()/2, 0)

	// Shrink the CJK face if its full-width glyphs would overflow two cells.
	cjkPx := px
	probe := mustFace(fs.cjk, px)
	if a, ok := probe.GlyphAdvance('日'); ok && a > fixed.I(2*fs.cellW) {
		cjkPx = px * float64(2*fs.cellW) / (float64(a) / 64)
	}
	fs.faces[faceCJK] = mustFace(fs.cjk, cjkPx)
	return fs
}

// faceFor picks the mono face when it has the rune, else the CJK face, else faceMissing.
func (fs *fontSet) faceFor(r rune, bold bool) faceID {
	mono := faceMono
	if bold {
		mono = faceMonoBold
	}
	if i, _ := fs.mono.GlyphIndex(&fs.buf, r); i != 0 {
		return mono
	}
	if i, _ := fs.cjk.GlyphIndex(&fs.buf, r); i != 0 {
		return faceCJK
	}
	return faceMissing
}
