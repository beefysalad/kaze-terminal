package ui

import (
	"os"
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

func TestCodeDrawPathsSelectMono(t *testing.T) {
	for file, want := range map[string]string{
		"dock_readable_draw.go": "c.fitMono(gtx, it.obj",
		"doc_draw.go":           "face = monoFace",
	} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Fatalf("%s does not select mono for code", file)
		}
	}
	b, _ := os.ReadFile("dock_readable_draw.go")
	if !strings.Contains(string(b), "c.fitMono(gtx, ln.Text") {
		t.Fatal("command/output is not mono")
	}
}
