package ui

import (
	"image/color"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// Every text of the readable Activity reaches 4.5:1 on the surface it is
// drawn on, in every theme, the diff lines on their diff backgrounds too.
func TestReadInkContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		bg := th.Term.BG
		ink := c.readInk(bg)
		low := 99.0
		check := func(label string, fg, on color.NRGBA) {
			got := contrast(fg, on)
			low = min(low, got)
			if got < 4.5 {
				t.Errorf("%s: %s contrast %.2f", th.Key, label, got)
			}
		}
		for label, fg := range map[string]color.NRGBA{"say": ink.say, "step": ink.step, "strong": ink.strong, "mono": ink.mono,
			"meta": ink.meta, "add": ink.add, "del": ink.del, "ok": ink.ok, "bad": ink.bad} {
			check(label+" on the panel", fg, bg)
		}
		for label, fg := range map[string]color.NRGBA{"dim": ink.dim, "mono": ink.detailMono, "list": ink.list} {
			check(label+" on the detail", fg, ink.detailBG)
		}
		check("added line", ink.addFG, ink.addBG)
		check("removed line", ink.delFG, ink.delBG)
		if ink.sayBar != claudeCoral {
			t.Errorf("%s: note bar %v, want Claude's coral", th.Key, ink.sayBar)
		}
		t.Logf("%s: lowest readable contrast %.2f", th.Key, low)
	}
	_ = render.Readable
}
