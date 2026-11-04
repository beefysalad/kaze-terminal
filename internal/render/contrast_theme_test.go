package render_test

import (
	"image/color"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// Text on a find match and on the current match reaches 4.5:1 in every theme,
// whatever colour of the theme the program printed it in, even the very
// colour of the highlight.
func TestMatchedTextIsReadable(t *testing.T) {
	for _, th := range theme.All() {
		p := th.Term
		ink := render.InkOfForTest(p)
		fgs := append([]color.NRGBA{p.FG}, p.ANSI[:]...)
		for _, bg := range []struct {
			name string
			c    color.NRGBA
		}{{"match", p.Match}, {"current", p.MatchCur}} {
			low := 21.0
			for i, fg := range fgs {
				got := render.MatchTextForTest(fg, bg.c, ink)
				if bg.name == "current" {
					got = render.CurTextForTest(bg.c, ink)
				}
				r := render.ContrastForTest(got, bg.c)
				if r < 4.5 { // fg == bg too: yellow text on the theme-yellow current match must show
					t.Errorf("%s: colour %d on the %s is %.2f, want >= 4.5", th.Key, i-1, bg.name, r)
				}
				low = min(low, r)
			}
			t.Logf("%s: lowest text on the %s %.2f", th.Key, bg.name, low)
		}
		// As the mockup: the current match's text is one solid ink, the darkest
		// the theme has (or white where that reads better), not a nudged grey.
		white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		want := ink
		if render.ContrastForTest(white, p.MatchCur) > render.ContrastForTest(ink, p.MatchCur) {
			want = white
		}
		if cur := render.CurTextForTest(p.MatchCur, ink); cur != want || render.ContrastForTest(cur, p.MatchCur) < 4.5 {
			t.Errorf("%s: current-match text %v, want %v at >= 4.5", th.Key, cur, want)
		}
	}
}
