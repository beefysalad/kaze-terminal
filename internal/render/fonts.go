package render

import (
	"image"
	"os"
	"sync"

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
	faceSystem  // first system fallback font; faceSystem+i is systemFonts font i
)

// sysFonts parses the system fallback fonts lazily, each file at most once, for every size.
type sysFonts struct {
	mu     sync.Mutex
	paths  []string
	next   int // paths[next:] are not opened yet
	fonts  []*sfnt.Font
	parses int // files parsed, read by tests
}

var systemFonts = &sysFonts{paths: systemFontPaths()}

// maxSystemFonts keeps faceSystem+i inside faceID; a .ttc can hold many fonts.
const maxSystemFonts = 32

// font returns fallback font i, opening more files until it exists; nil once the list is exhausted.
func (s *sysFonts) font(i int) *sfnt.Font {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i >= len(s.fonts) && s.next < len(s.paths) && len(s.fonts) < maxSystemFonts {
		path := s.paths[s.next]
		s.next++
		f, err := os.Open(path)
		if err != nil {
			continue // not installed on this machine
		}
		s.parses++
		// Read through the open file rather than into memory: Segoe UI Emoji alone is ~10MB.
		c, err := sfnt.ParseCollectionReaderAt(f)
		added := 0
		for j := 0; err == nil && j < c.NumFonts() && len(s.fonts) < maxSystemFonts; j++ {
			if ft, ferr := c.Font(j); ferr == nil {
				s.fonts = append(s.fonts, ft)
				added++
			}
		}
		if added == 0 {
			f.Close()
		}
	}
	if i < len(s.fonts) {
		return s.fonts[i]
	}
	return nil
}

// fontSet holds the parsed bundled fonts at one pixel size.
type fontSet struct {
	mono, cjk *sfnt.Font
	faces     [3]xfont.Face
	buf       sfnt.Buffer
	sys       *sysFonts
	px        float64

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
	fs := &fontSet{mono: mustParse(fonts.MonoRegular), cjk: mustParse(fonts.CJK), sys: systemFonts, px: px}
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

// faceFor picks the mono face when it has the rune, else the CJK face, else the first
// system font that has it (loading them only now), else faceMissing.
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
	for i := 0; ; i++ {
		f := fs.sys.font(i)
		if f == nil {
			return faceMissing
		}
		if g, _ := f.GlyphIndex(&fs.buf, r); g != 0 {
			return faceSystem + faceID(i)
		}
	}
}

// drawFallback draws r from a system font into a w-wide cell mask, shrunk to fit and
// centered: their metrics differ from the mono font's, and ink must not spill into neighbors.
func (fs *fontSet) drawFallback(f *sfnt.Font, r rune, w int) *image.Alpha {
	h := fs.cellH
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: fs.px, DPI: 72, Hinting: xfont.HintingFull})
	if err != nil {
		return mask
	}
	b, _, ok := face.GlyphBounds(r)
	if !ok {
		return mask
	}
	gw, gh := float64(b.Max.X-b.Min.X)/64, float64(b.Max.Y-b.Min.Y)/64
	if gw > float64(w) || gh > float64(h) {
		s := 0.95 * min(float64(w)/max(gw, 1), float64(h)/max(gh, 1))
		if smaller, err := opentype.NewFace(f, &opentype.FaceOptions{Size: fs.px * s, DPI: 72, Hinting: xfont.HintingFull}); err == nil {
			face = smaller
			b, _, _ = face.GlyphBounds(r)
		}
	}
	x := (fixed.I(w) - b.Min.X - b.Max.X) / 2
	y := fixed.I(fs.ascent) // keep the mono baseline when the ink fits around it
	if y+b.Min.Y < 0 || y+b.Max.Y > fixed.I(h) {
		y = (fixed.I(h) - b.Min.Y - b.Max.Y) / 2
	}
	d := xfont.Drawer{Dst: mask, Src: image.Opaque, Face: face, Dot: fixed.P(x.Round(), y.Round())}
	d.DrawString(string(r))
	return mask
}
