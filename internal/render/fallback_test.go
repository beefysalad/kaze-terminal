package render

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func segoeSymbol(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("the fallback symbol test uses the Windows font list")
	}
	p := filepath.Join(os.Getenv("WINDIR"), "Fonts", "seguisym.ttf")
	if _, err := os.Stat(p); err != nil {
		t.Skip("Segoe UI Symbol is not installed")
	}
	return p
}

func TestFallbackDrawsClaudeSymbols(t *testing.T) {
	segoeSymbol(t)
	c := NewRenderer(15, DefaultPalette()).glyphs
	w, h := c.fs.cellW, c.fs.cellH
	notdef := c.get(notdefProbe, false, false)
	for _, r := range []rune{'⏺', '⎿', '✻', '✔'} {
		g := c.get(r, false, false)
		if g.face == faceMissing || g == notdef {
			t.Errorf("%U resolved to .notdef", r)
			continue
		}
		if g.empty {
			t.Errorf("%U drew no ink", r)
		}
		if b := g.mask.Bounds(); b.Dx() != w || b.Dy() != h {
			t.Errorf("%U mask is %v, want one %dx%d cell", r, b, w, h)
		}
	}
}

func TestRuneMissingEverywhereIsNotdef(t *testing.T) {
	c := NewRenderer(16, DefaultPalette()).glyphs
	a := c.get(0x10FFFD, false, false) // plane 16 private use: no font maps it
	b := c.get(0xE01EF, false, false)  // variation selector supplement: never drawn by these fonts
	if a.face != faceMissing || a != b {
		t.Fatalf("missing runes: faces %d/%d, same mask %v", a.face, b.face, a == b)
	}
}

func TestSystemFontParsedOnlyOnMiss(t *testing.T) {
	path := segoeSymbol(t)
	sys := &sysFonts{paths: []string{filepath.Join(t.TempDir(), "absent.ttf"), path}}
	fs := newFontSet(13)
	fs.sys = sys
	c := &glyphCache{fs: fs, m: make(map[glyphKey]*glyph)}
	for _, r := range []rune{'A', '日', '─', '→'} {
		c.get(r, false, false)
	}
	if sys.parses != 0 {
		t.Fatalf("%d system fonts parsed before any rune missed the bundled fonts", sys.parses)
	}
	if g := c.get('⏺', false, false); g.face == faceMissing {
		t.Fatal("⏺ not found in Segoe UI Symbol")
	}
	c.get('✻', false, false)
	c.get(0x10FFFD, false, false)
	if sys.parses != 1 {
		t.Fatalf("parsed %d system fonts, want exactly 1 (the absent file is skipped)", sys.parses)
	}
}
