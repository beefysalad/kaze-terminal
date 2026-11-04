package ui

import (
	"image/color"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func paletteInks(p dockPalette) map[string]color.NRGBA {
	return map[string]color.NRGBA{"text": p.text, "muted": p.muted, "accent": p.accent, "needs": p.needs, "done": p.done, "type": p.kind}
}

// Every board text reaches 4.5:1 on the surface it is drawn on, in every theme.
func TestBoardContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		ink := c.boardInk()
		low := 99.0
		check := func(where, label string, fg, bg color.NRGBA) {
			got := contrast(fg, bg)
			low = min(low, got)
			if got < 4.5 {
				t.Errorf("%s: %s %s contrast %.2f", th.Key, where, label, got)
			}
		}
		for _, s := range []struct {
			where string
			p     dockPalette
			bgs   map[string]color.NRGBA
		}{
			{"page", ink.page, map[string]color.NRGBA{"bg": ink.bg, "raised": ink.raised, "field": ink.field}},
			{"card", ink.onCard, map[string]color.NRGBA{"card": ink.card, "raised": ink.raised}},
			{"needs", ink.onNeeds, map[string]color.NRGBA{"needsBG": ink.needsBG, "needsRaised": ink.needsRaised}},
		} {
			for bgName, bg := range s.bgs {
				for label, fg := range paletteInks(s.p) {
					check(s.where+" on "+bgName, label, fg, bg)
				}
			}
		}
		check("card text on field", "text", ink.onCard.text, ink.field)
		check("primary button", "label", ink.priFG, ink.priBG)
		t.Logf("%s: lowest board contrast %.2f", th.Key, low)
	}
}
