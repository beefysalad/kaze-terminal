package ui

import (
	"strings"
	"testing"

	"gioui.org/font"
)

func TestChromeBundlesApprovedFonts(t *testing.T) {
	c := newChrome()
	for _, f := range c.faces {
		if tf := string(f.Font.Typeface); tf != chromeFace && tf != monoFace && tf != "Noto Sans JP" {
			t.Fatal(tf)
		}
	}
	if chromeFace != "Geist" || monoFace != "JetBrains Mono" {
		t.Fatal("font split")
	}
	for _, w := range []font.Weight{font.Normal, font.SemiBold, font.Bold} {
		_, a := c.labelW(pageContext(800, 600), "iiiiiiii", 13, c.th.ActiveFG, w)
		_, b := c.labelW(pageContext(800, 600), "WWWWWWWW", 13, c.th.ActiveFG, w)
		if a.X >= b.X {
			t.Fatal(w)
		}
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
