package render

import (
	"io"
	"os"
	"sync"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

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

// isTextEmoji reports whether r is an emoji drawn as text by default (⚠ ✔ ❤ ⏺).
func isTextEmoji(r rune) bool { return r > 0x7F && inSpans(textEmojiSpans[:], r) }

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
func loadEmojiFont(path string) (e *emojiFont) {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			e = nil
		}
		if e == nil {
			f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil {
		return nil
	}
	d, err := colorfont.ReadDirectory(f, st.Size(), 0)
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
