package theme

import (
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/render"
)

func hexc(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func lum(c color.NRGBA) float64 {
	f := func(u uint8) float64 {
		v := float64(u) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
}

func ratio(a, b color.NRGBA) float64 {
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestMidnightMatchesTodaysColors(t *testing.T) {
	m := Default()
	if m.Term != render.DefaultPalette() {
		t.Fatal("midnight Term differs from render.DefaultPalette()")
	}
	if m.UI.Accent != hexc(0x8b7bff) || m.UI.Chrome != hexc(0x0b0d12) ||
		m.UI.Needs != hexc(0xf2b84b) || m.UI.UsageHigh != hexc(0xff6b6b) {
		t.Fatalf("midnight chrome drifted: %+v", m.UI)
	}
}

func TestAllThemes(t *testing.T) {
	want := []string{"midnight", "sumi", "tide", "washi", "sakura", "matcha", "koyo", "hai", "fuji", "kiri"}
	all := All()
	if len(all) != len(want) {
		t.Fatalf("got %d themes, want %d", len(all), len(want))
	}
	for i, th := range all {
		if th.Key != want[i] {
			t.Errorf("theme %d key = %q, want %q", i, th.Key, want[i])
		}
		if r := ratio(th.Term.FG, th.Term.BG); r < 7 {
			t.Errorf("%s FG/BG contrast %.2f < 7", th.Key, r)
		}
	}
	if _, ok := ByKey("nope"); ok {
		t.Error("ByKey(nope) should be false")
	}
	// Two light themes, Washi and Kiri; the rest are dark.
	for _, th := range all {
		light := th.Key == "washi" || th.Key == "kiri"
		if l := lum(th.Term.BG); light && l <= 0.7 || !light && l >= 0.1 {
			t.Errorf("%s BG luminance %.2f, light = %v", th.Key, l, light)
		}
	}
}

// Every theme in the picker is one config.toml accepts without a warning.
func TestConfigAcceptsEveryTheme(t *testing.T) {
	for _, th := range All() {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(`theme = "`+th.Key+`"`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, warns, err := config.Load(path)
		if err != nil || len(warns) != 0 || cfg.Theme != th.Key {
			t.Errorf("theme %q: theme %q, warnings %v, err %v", th.Key, cfg.Theme, warns, err)
		}
	}
}

// The diff colours of the readable agent activity: set in every theme, and
// each foreground readable on its own background before any nudge.
func TestDiffTokensInEveryTheme(t *testing.T) {
	for _, th := range All() {
		u := th.UI
		for name, c := range map[string]color.NRGBA{"DiffAddBG": u.DiffAddBG, "DiffAddFG": u.DiffAddFG, "DiffDelBG": u.DiffDelBG, "DiffDelFG": u.DiffDelFG} {
			if c == (color.NRGBA{}) {
				t.Errorf("%s: %s unset", th.Key, name)
			}
		}
		if r := ratio(u.DiffAddFG, u.DiffAddBG); r < 4.5 {
			t.Errorf("%s: added line contrast %.2f", th.Key, r)
		}
		if r := ratio(u.DiffDelFG, u.DiffDelBG); r < 4.5 {
			t.Errorf("%s: removed line contrast %.2f", th.Key, r)
		}
		t.Logf("%s: add %.2f del %.2f", th.Key, ratio(u.DiffAddFG, u.DiffAddBG), ratio(u.DiffDelFG, u.DiffDelBG))
	}
}

func dist(a, b color.NRGBA) float64 {
	dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// The find highlight colours: set in every theme, a match tint apart from the
// selection, plain text readable on it as is, current-match text that can be
// made readable, and the scroll marks visible on the background.
func TestMatchTokensInEveryTheme(t *testing.T) {
	for _, th := range All() {
		p := th.Term
		for name, c := range map[string]color.NRGBA{"Match": p.Match, "MatchCur": p.MatchCur, "MatchMark": p.MatchMark} {
			if c == (color.NRGBA{}) {
				t.Errorf("%s: %s unset", th.Key, name)
			}
		}
		// Every theme, Midnight included (owner, 2026-11-04): a selection over a
		// match must stay easy to tell apart.
		if d := dist(p.Match, p.Selection); d < 60 {
			t.Errorf("%s: Match is %.0f from Selection, want >= 60", th.Key, d)
		}
		if d := dist(p.Match, p.MatchCur); d < 60 {
			t.Errorf("%s: Match is %.0f from MatchCur, want >= 60", th.Key, d)
		}
		if r := ratio(p.FG, p.Match); r < 4.5 {
			t.Errorf("%s: text on a match %.2f, want >= 4.5", th.Key, r)
		}
		ink := p.BG // the darker of FG and BG, which the renderer pulls text toward
		if lum(p.FG) < lum(p.BG) {
			ink = p.FG
		}
		cur := max(ratio(ink, p.MatchCur), ratio(hexc(0xffffff), p.MatchCur))
		if cur < 4.5 {
			t.Errorf("%s: best text on the current match %.2f, want >= 4.5", th.Key, cur)
		}
		if r := ratio(p.MatchMark, p.BG); r < 3 {
			t.Errorf("%s: scroll mark on the background %.2f, want >= 3", th.Key, r)
		}
		if r := ratio(p.MatchCur, p.BG); r < 3 {
			t.Errorf("%s: current scroll mark on the background %.2f, want >= 3", th.Key, r)
		}
		t.Logf("%s: text/match %.2f, text/current %.2f, mark %.2f, current mark %.2f",
			th.Key, ratio(p.FG, p.Match), cur, ratio(p.MatchMark, p.BG), ratio(p.MatchCur, p.BG))
	}
}
