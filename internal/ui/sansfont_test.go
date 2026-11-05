package ui

import (
	"testing"

	"gioui.org/font"
)

// JetBrains Mono is kaze's only UI font; Noto Sans JP is there only as the
// fallback for Japanese characters JetBrains Mono does not have.
func TestChromeBundlesOnlyJetBrainsMono(t *testing.T) {
	c := newChrome()
	for _, f := range c.faces {
		if tf := string(f.Font.Typeface); tf != chromeFace && tf != "Noto Sans JP" {
			t.Errorf("bundled UI face %q: JetBrains Mono is the only UI font", tf)
		}
	}
}

// Settings text, at every weight it uses, is drawn in JetBrains Mono: as wide
// as the mono label, and Bold still reads heavier than Normal.
func TestSettingsTextUsesJetBrainsMono(t *testing.T) {
	c := newChrome()
	gtx := pageContext(800, 600)
	const s = "Appearance Settings"
	_, mono := c.label(gtx, s, 13, c.th.ActiveFG)
	for _, w := range []font.Weight{font.Normal, font.Medium, font.SemiBold, font.Bold} {
		if _, sz := c.labelW(gtx, s, 13, c.th.ActiveFG, w); sz.X != mono.X {
			t.Errorf("weight %v is %d wide, mono is %d: not drawn in JetBrains Mono", w, sz.X, mono.X)
		}
	}
	if chromeFace != "JetBrains Mono" {
		t.Fatalf("chromeFace = %q", chromeFace)
	}
}
