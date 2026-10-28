package render

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/image/font/sfnt"
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

func TestFallbackLookupIsMemoized(t *testing.T) {
	path := segoeSymbol(t)
	sys := &sysFonts{paths: []string{path}}
	fs := newFontSet(13)
	fs.sys = sys
	c := &glyphCache{fs: fs, m: make(map[glyphKey]*glyph)}
	for _, r := range []rune{'⏺', '✻', 0x10FFFD, '日'} {
		c.get(r, false, false)
		calls, lookups := sys.calls, fs.lookups
		for i := 0; i < 3; i++ {
			c.get(r, false, false)
			c.get(r, true, false)
		}
		if sys.calls != calls || fs.lookups != lookups {
			t.Errorf("%U redrawn: %d more sys.font calls, %d more cmap lookups, want 0",
				r, sys.calls-calls, fs.lookups-lookups)
		}
	}
}

func TestBadSystemFontIsSkipped(t *testing.T) {
	path := segoeSymbol(t)
	dir := t.TempDir()
	garbage := filepath.Join(dir, "garbage.ttf")
	if err := os.WriteFile(garbage, []byte("true\x00\x10not a font at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(dir, "truncated.ttf")
	if err := os.WriteFile(truncated, good[:min(len(good), 4096)], 0o644); err != nil {
		t.Fatal(err)
	}
	panicky := filepath.Join(dir, "panicky.ttf")
	if err := os.WriteFile(panicky, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sys := &sysFonts{paths: []string{garbage, truncated, panicky, path}}
	sys.parse = func(r io.ReaderAt) (*sfnt.Collection, error) {
		if f, ok := r.(*os.File); ok && filepath.Base(f.Name()) == "panicky.ttf" {
			panic("parser bug")
		}
		return sfnt.ParseCollectionReaderAt(r)
	}
	fs := newFontSet(13)
	fs.sys = sys
	c := &glyphCache{fs: fs, m: make(map[glyphKey]*glyph)}
	if g := c.get('⏺', false, false); g.face == faceMissing || g.empty {
		t.Fatalf("⏺ not drawn from the good font after bad ones: face %d", g.face)
	}
	if g := c.get(0x10FFFD, false, false); g.face != faceMissing {
		t.Fatalf("missing rune got face %d", g.face)
	}
}
