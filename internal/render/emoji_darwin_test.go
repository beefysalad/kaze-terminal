package render

import "testing"

// Runs on macOS CI: Apple Color Emoji is drawn from its sbix bitmaps.
func TestAppleColorEmojiIsColour(t *testing.T) {
	c := &glyphCache{fs: newFontSet(15.75, 1), m: make(map[glyphKey]*glyph)}
	g := c.get('✅', false, true)
	if g.color == nil {
		t.Fatalf("✅: face %d, no colour image", g.face)
	}
	if n := len(opaqueColours(g.color)); n < 2 {
		t.Fatalf("✅ has %d opaque colours, want at least 2", n)
	}
	if g := c.get('⏺', false, false); g.face < faceSystem {
		t.Fatalf("⏺: face %d, want the system list as before (faceSystem %d or later)", g.face, faceSystem)
	}
}
