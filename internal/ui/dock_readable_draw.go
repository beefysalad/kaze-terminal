package ui

import (
	"image/color"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
)

// readInk is the readable Activity's colours (mockup
// docs/design/readable-activity-mockup.html), all from the theme, each text
// nudged to 4.5:1 on the surface it is drawn on.
type readInk struct {
	say, sayBar, step, strong, mono, meta color.NRGBA // on the panel
	add, del, ok, bad                     color.NRGBA // meta tones on the panel
	detailBG, detailLine                  color.NRGBA // an open step's box
	dim, list, detailMono                 color.NRGBA // on the box
	addBG, addFG, delBG, delFG            color.NRGBA // diff lines
}

// readInk works the colours out for panel background bg.
func (c *chrome) readInk(bg color.NRGBA) readInk {
	th := c.th
	r := func(fg, on color.NRGBA) color.NRGBA { return render.Readable(fg, on) }
	k := readInk{
		sayBar:     agentLogoColor("claude", th.Accent),
		say:        r(th.ActiveFG, bg),
		step:       r(mix(th.Muted, th.ActiveFG, 0.35), bg),
		strong:     r(th.ActiveFG, bg),
		mono:       r(th.ActiveFG, bg),
		meta:       r(th.Muted, bg),
		add:        r(th.DiffAddFG, bg),
		del:        r(th.DiffDelFG, bg),
		ok:         r(th.Done, bg),
		bad:        r(th.UsageHigh, bg),
		detailBG:   th.Chrome,
		detailLine: th.Border,
		addBG:      th.DiffAddBG,
		delBG:      th.DiffDelBG,
	}
	k.dim = r(th.Muted, k.detailBG)
	k.list = r(mix(th.Muted, th.ActiveFG, 0.35), k.detailBG)
	k.detailMono = r(th.ActiveFG, k.detailBG)
	k.addFG = r(th.DiffAddFG, k.addBG)
	k.delFG = r(th.DiffDelFG, k.delBG)
	return k
}
