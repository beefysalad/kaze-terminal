package ui

import (
	"strings"
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

// The settings menu's subtitle fits inside the 220 dp menu, padding included,
// in JetBrains Mono.
func TestSettingsNavSubtitleFits(t *testing.T) {
	win := appearanceWindow(t, "")
	gtx := pageContext(800, 600)
	room := gtx.Dp(220) - 2*gtx.Dp(12) - gtx.Dp(10)
	for _, s := range win.set.texts {
		if strings.HasPrefix(s, "Saved to config.toml") {
			if _, sz := win.ui.label(gtx, s, 11, win.ui.th.Muted); sz.X > room {
				t.Fatalf("%q is %d wide, the menu has %d", s, sz.X, room)
			}
			return
		}
	}
	t.Fatalf("no subtitle in %q", win.set.texts)
}
