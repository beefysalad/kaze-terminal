package ui

import (
	"image/color"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
)

// boardInk is the agents board's colours (mockup docs/design/agents-board-mockup.html),
// all from the theme: layered surfaces with hairlines, and text palettes nudged
// to 4.5:1 on the surface they are drawn on.
type boardInk struct {
	bg, card, raised, field, line   color.NRGBA
	needsBG, needsRaised, needsLine color.NRGBA // the Needs you card, its hovered row and border
	meterDone, priBG, priFG         color.NRGBA
	page, onCard, onNeeds           dockPalette
}

func (c *chrome) boardInk() boardInk {
	th := c.th
	k := boardInk{
		bg:        th.Active,
		card:      mix(th.Active, th.ActiveFG, 0.035),
		raised:    mix(th.Active, th.ActiveFG, 0.07),
		field:     th.Chrome,
		line:      th.Border,
		needsLine: mix(th.Border, th.Needs, 0.4),
		meterDone: mix(th.Border, th.Muted, 0.3),
		priBG:     th.Accent,
		priFG:     render.Readable(th.ActiveFG, th.Accent),
	}
	k.needsBG = mix(k.card, th.Needs, 0.07)
	k.needsRaised = mix(k.needsBG, th.Needs, 0.07)
	k.page = c.boardColors(k.bg, k.raised, k.field)
	k.onCard = c.boardColors(k.card, k.raised, k.field)
	k.onNeeds = c.boardColors(k.needsBG, k.needsRaised)
	return k
}

// boardColors is dockColors readable on every one of bgs: the surface a text sits
// on plus the hover, selection or field fills it can sit on too. Each nudge only
// moves away from the surfaces, so a second pass settles the earlier ones.
func (c *chrome) boardColors(bgs ...color.NRGBA) dockPalette {
	r := func(fg color.NRGBA) color.NRGBA {
		for range 2 {
			for _, bg := range bgs {
				fg = render.Readable(fg, bg)
			}
		}
		return fg
	}
	th := c.th
	return dockPalette{text: r(th.ActiveFG), muted: r(th.Muted), accent: r(th.Accent), needs: r(th.Needs), done: r(th.Done), kind: r(c.memberColor("magenta"))}
}
