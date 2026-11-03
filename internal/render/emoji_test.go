package render

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
