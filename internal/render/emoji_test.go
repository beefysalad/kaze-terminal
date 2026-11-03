package render

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/colorfont"
)

func TestEmojiTables(t *testing.T) {
	for _, r := range []rune{'✅', '❌', '🔄', '🟢', '🔴', '🟡', '⏳', '🚀', '📝', '⌚'} {
		if !isEmojiPresentation(r) {
			t.Errorf("isEmojiPresentation(%c %U) = false", r, r)
		}
	}
	for _, r := range []rune{'⚠', '✔', '❤', '©', '⏺', 'A', '✳'} {
		if isEmojiPresentation(r) {
			t.Errorf("isEmojiPresentation(%c %U) = true", r, r)
		}
	}
	for _, r := range []rune{'⚠', '✔', '❤', '⏺', '©'} {
		if !isTextEmoji(r) {
			t.Errorf("isTextEmoji(%c %U) = false", r, r)
		}
	}
	for _, r := range []rune{'✅', '🔄', '#', '7', '✳'} {
		if isTextEmoji(r) {
			t.Errorf("isTextEmoji(%c %U) = true", r, r)
		}
	}
	if a := testing.AllocsPerRun(100, func() { isEmojiPresentation('🔄'); isTextEmoji('⚠') }); a != 0 {
		t.Fatalf("emoji table lookups allocate %v times", a)
	}
}

// segoeEmoji skips unless Segoe UI Emoji is installed (Windows).
func segoeEmoji(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("uses Segoe UI Emoji")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "seguiemj.ttf")); err != nil {
		t.Skip("Segoe UI Emoji is not installed")
	}
}

// swapEmojiOpen makes the shared emoji source load through open from now on,
// and restores it (and its loaded font) when the test ends. Tests that use it
// must ask for a pixel size no other test uses, so their glyph cache starts fresh.
func swapEmojiOpen(t *testing.T, open func(path string) *emojiFont) {
	t.Helper()
	s := emojiFonts
	s.mu.Lock()
	oldOpen, oldDone, oldF := s.open, s.done, s.f
	s.open, s.done, s.f = open, false, nil
	s.mu.Unlock()
	t.Cleanup(func() {
		s.mu.Lock()
		s.open, s.done, s.f = oldOpen, oldDone, oldF
		s.mu.Unlock()
	})
}

func TestCheckMarkUsesEmojiFace(t *testing.T) {
	segoeEmoji(t)
	c := NewRenderer(14.5, DefaultPalette(), Options{}).glyphs
	for _, r := range []rune{'✅', '🔄'} {
		g := c.get(r, false, true)
		if g.face != faceEmoji {
			t.Fatalf("%c: face %d, want faceEmoji %d", r, g.face, faceEmoji)
		}
		if g.empty {
			t.Fatalf("%c drew no ink", r)
		}
		if g.mask != nil {
			if b := g.mask.Bounds(); b.Dx() != 2*c.fs.cellW || b.Dy() != c.fs.cellH {
				t.Fatalf("%c mask %v, want two cells %dx%d", r, b, 2*c.fs.cellW, c.fs.cellH)
			}
		}
	}
}

func TestTextEmojiUsesEmojiOutline(t *testing.T) {
	segoeEmoji(t)
	c := NewRenderer(14.5, DefaultPalette(), Options{}).glyphs
	for _, r := range []rune{'❤', '✔', '⏺'} {
		if g := c.get(r, false, false); g.face != faceEmojiText || g.empty {
			t.Fatalf("%c: face %d empty %v, want faceEmojiText %d with ink", r, g.face, g.empty, faceEmojiText)
		}
	}
	// ⚠ is in the bundled mono font, which comes before the emoji outline.
	if g := c.get('⚠', false, false); g.face != faceMono {
		t.Fatalf("⚠: face %d, want the bundled mono font", g.face)
	}
	if g := c.get('✻', false, false); g.face != faceSystem {
		t.Fatalf("✻: face %d, want faceSystem %d (Segoe UI Symbol, unchanged)", g.face, faceSystem)
	}
}

func TestNoEmojiFontKeepsOldOrder(t *testing.T) {
	swapEmojiOpen(t, func(string) *emojiFont { return nil })
	c := NewRenderer(14.75, DefaultPalette(), Options{}).glyphs
	for _, r := range []rune{'✅', '🔄', '⚠', '⏺'} {
		if id := c.fs.faceFor(r, false); id == faceEmoji || id == faceEmojiText {
			t.Errorf("%c: faceFor gave an emoji face %d with no emoji font", r, id)
		}
	}
	if runtime.GOOS == "windows" {
		segoeSymbol(t)
		if g := c.get('✅', false, true); g.face != faceSystem {
			t.Fatalf("✅ with no emoji font: face %d, want Segoe UI Symbol (faceSystem %d) as before", g.face, faceSystem)
		}
	}
}

// opaqueColours returns the distinct colours of fully opaque pixels.
func opaqueColours(img *image.RGBA) map[color.RGBA]bool {
	seen := map[color.RGBA]bool{}
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 0xff {
			seen[color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 0xff}] = true
		}
	}
	return seen
}

func TestCheckMarkIsColour(t *testing.T) {
	segoeEmoji(t)
	c := NewRenderer(15.5, DefaultPalette(), Options{}).glyphs
	g := c.get('✅', false, true)
	if g.color == nil {
		t.Fatal("✅ has no colour image")
	}
	if g.mask != nil {
		t.Fatal("a colour glyph keeps a mask too")
	}
	if dx, dy := g.color.Rect.Dx(), g.color.Rect.Dy(); dx != 2*c.fs.cellW || dy != c.fs.cellH {
		t.Fatalf("colour image %dx%d, want %dx%d", dx, dy, 2*c.fs.cellW, c.fs.cellH)
	}
	cols := opaqueColours(g.color)
	green := false
	for col := range cols {
		if col.G > col.R && col.G > col.B {
			green = true
		}
	}
	if len(cols) < 2 || !green {
		t.Fatalf("%d opaque colours, green %v; want the green box and the white check", len(cols), green)
	}
}

func TestColourGlyphIgnoresFG(t *testing.T) {
	segoeEmoji(t)
	h := newHarness(t, 10, 2)
	h.out("\x1b[31m✅\x1b[0m\r\n✅")
	h.frame()
	if g := h.r.glyphs.get('✅', false, true); g.color == nil {
		t.Fatal("✅ is not a colour glyph")
	}
	for dy := 0; dy < h.r.cellH; dy++ {
		for dx := 0; dx < 2*h.r.cellW; dx++ {
			if a, b := pixelAt(h.r, 0, 0, dx, dy), pixelAt(h.r, 0, 1, dx, dy); a != b {
				t.Fatalf("pixel %d,%d: red-FG emoji %v, default-FG emoji %v", dx, dy, a, b)
			}
		}
	}
}

func TestSelectionAndInverseShowBehindEmoji(t *testing.T) {
	segoeEmoji(t)
	h := newHarness(t, 10, 2)
	h.out("\x1b[7m✅\x1b[0m")
	h.frame()
	if g := h.r.glyphs.get('✅', false, true); g.color == nil {
		t.Fatal("✅ is not a colour glyph")
	}
	p := DefaultPalette()
	if got := pixelAt(h.r, 0, 0, 0, 0); got != toArr(p.FG) {
		t.Fatalf("inverse emoji corner %v, want the inverse background %v", got, toArr(p.FG))
	}
	h.frameSel(&Selection{Start: Point{0, 0}, End: Point{1, 0}, Active: true})
	if got := pixelAt(h.r, 0, 0, 0, 0); got != toArr(p.Selection) {
		t.Fatalf("selected emoji corner %v, want the selection colour %v", got, toArr(p.Selection))
	}
}

// realEmojiFont loads the installed emoji font, failing the test if it does not load.
func realEmojiFont(t *testing.T) *emojiFont {
	t.Helper()
	e := loadEmojiFont(emojiFontPath())
	if e == nil {
		t.Fatal("the emoji font did not load")
	}
	return e
}

func TestBrokenColourTablesDrawMonochrome(t *testing.T) {
	segoeEmoji(t)
	e := realEmojiFont(t)
	bad, err := colorfont.ParseCOLR([]byte{0, 0, 0xFF, 0xFF}, 10)
	if err == nil {
		t.Fatal("corrupt COLR parsed")
	}
	e.colr = bad
	swapEmojiOpen(t, func(string) *emojiFont { return e })
	c := &glyphCache{fs: newFontSet(15.25, 1), m: make(map[glyphKey]*glyph)}
	g := c.get('✅', false, true)
	if g.face != faceEmoji || g.color != nil || g.mask == nil || g.empty {
		t.Fatalf("✅ with a broken COLR: face %d colour %v ink %v; want the emoji outline", g.face, g.color != nil, !g.empty)
	}
	// A layer whose glyph cannot be loaded: no colour, no panic.
	if img := c.fs.drawCOLR(e, 1, []colorfont.Layer{{Glyph: 0xFFFE, Palette: 0}}, 2*c.fs.cellW); img != nil {
		t.Fatal("drawCOLR drew a layer glyph that does not exist")
	}
}

func TestColourGlyphMemory(t *testing.T) {
	segoeEmoji(t)
	c := &glyphCache{fs: newFontSet(14, 1), m: make(map[glyphKey]*glyph)}
	n, bytes := 0, 0
	for _, s := range emojiPresentationSpans {
		for r := s.lo; r <= s.hi && n < 200; r++ {
			if g := c.get(r, false, true); g.color != nil {
				n++
				bytes += len(g.color.Pix)
			} else {
				delete(c.m, glyphKey{r, false, true})
			}
		}
	}
	if n != 200 {
		t.Fatalf("found %d colour emoji, want 200", n)
	}
	if c.len() != 200 {
		t.Fatalf("%d glyphs cached, want 200", c.len())
	}
	t.Logf("200 colour emoji at 14 px (cell %dx%d): %d bytes (%.1f KiB) of colour images",
		c.fs.cellW, c.fs.cellH, bytes, float64(bytes)/1024)
	if bytes >= 4<<20 {
		t.Fatalf("200 colour emoji take %d bytes, want under 4 MiB", bytes)
	}
}

func TestColourGlyphRasterizedOnce(t *testing.T) {
	segoeEmoji(t)
	h := newHarness(t, 80, 24)
	h.r = NewRenderer(14.375, DefaultPalette(), Options{}) // a size no other test uses
	h.frame()
	for i := 0; i < 10; i++ {
		h.out(strings.Repeat("✅", 100))
		h.frame()
	}
	if n := h.r.Stats().GlyphsCached; n != 1 {
		t.Fatalf("GlyphsCached = %d after 1000 ✅, want 1", n)
	}
	if g := h.r.glyphs.get('✅', false, true); g.color == nil {
		t.Fatal("✅ is not a colour glyph")
	}
	if n := h.frame(); n != 0 {
		t.Fatalf("an unchanged frame redrew %d rows, want 0", n)
	}
}
