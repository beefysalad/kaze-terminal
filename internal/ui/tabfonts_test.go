package ui

import (
	"testing"

	"gioui.org/font"
	"gioui.org/text"

	"github.com/patrick-gogo/kaze/internal/render"
)

// missingGlyphs counts glyphs the shaper drew as .notdef (ID 0) for s.
func missingGlyphs(c *chrome, s string) int {
	c.shaper.LayoutString(text.Parameters{Font: font.Font{Typeface: chromeFace}, PxPerEm: 16, MaxLines: 1, MaxWidth: 1 << 20}, s)
	n := 0
	for {
		g, ok := c.shaper.NextGlyph()
		if !ok {
			return n
		}
		if g.ID&0xffffffff == 0 { // low 32 bits hold the font glyph index; the face sits above
			n++
		}
	}
}

func TestFitKeepsJapaneseAndLatin(t *testing.T) {
	c := newChrome()
	if got := c.fit("日本語 abc"); got != "日本語 abc" {
		t.Fatalf("got %q", got)
	}
	if len(c.loaded) != 0 {
		t.Fatal("system fonts loaded for text the bundled fonts cover")
	}
}

func TestFitDrawsSymbolWithoutMissingGlyph(t *testing.T) {
	if render.BundledCovers('✳') || render.SystemFontFor('✳') == "" {
		t.Skip("no system font with the symbol")
	}
	c := newChrome()
	got := c.fit("✳ Claude Code")
	if got != "✳ Claude Code" {
		t.Fatalf("symbol dropped: %q", got)
	}
	if n := missingGlyphs(c, got); n != 0 {
		t.Fatalf("%d missing glyphs", n)
	}
}

func TestFitDropsRunesNoFontHas(t *testing.T) {
	c := newChrome()
	if got := c.fit("a\U0010FFFFb\x07c"); got != "abc" {
		t.Fatalf("got %q", got)
	}
}
