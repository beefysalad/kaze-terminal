package ui

import (
	"image/color"

	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// palInk is the palette's colours (docs/design/palette-mockup.html), all from
// the theme: layered surfaces with hairlines, every text nudged by
// render.Readable to 4.5:1 on each surface it is drawn on.
type palInk struct {
	bg, line, outer, raised, sel, selBar, foot, switchBG color.NRGBA // surfaces
	text, muted, faint, goInk, chipFG, chipBG, chipLine  color.NRGBA
	keyFG, keyBG, keyLine, badge                         color.NRGBA
	pillNeedsFG, pillNeedsBG, pillRunFG, pillRunBG       color.NRGBA
	pillIdleFG, pillIdleBG, pillDoneFG, pillDoneBG       color.NRGBA
	tileBG, tileFG                                       color.NRGBA    // browse and more tiles
	plugins                                              [6]color.NRGBA // tile colours: ansi 1..6, readable on bg
	pluginInk                                            [6]color.NRGBA // the initial on each
	hairline                                             color.NRGBA    // the top line's bright middle
	under                                                color.NRGBA    // the surface of the row being drawn
	audit                                                *[]palPair     // a test sets it to collect what is drawn
}

// palPair is a colour the drawing used, the surface it sat on and the
// contrast it needs (4.5 for text, 3 for marks).
type palPair struct {
	what   string
	fg, bg color.NRGBA
	min    float64
}

// note records a pair when a test is collecting them; otherwise it costs nothing.
func (k *palInk) note(what string, fg, bg color.NRGBA, min float64) {
	if k.audit != nil {
		*k.audit = append(*k.audit, palPair{what, fg, bg, min})
	}
}

// palInk is built once per theme: setTheme drops it.
func (c *chrome) palInk() *palInk {
	if c.palK == nil {
		c.palK = buildPalInk(c.th, c.ansi)
	}
	return c.palK
}

// readableOn is fg made readable on every one of bgs. Each nudge only moves
// away from the surfaces, so a second pass settles the earlier ones.
func readableOn(fg color.NRGBA, bgs ...color.NRGBA) color.NRGBA {
	for range 2 {
		for _, bg := range bgs {
			fg = render.Readable(fg, bg)
		}
	}
	return fg
}

func buildPalInk(th theme.Chrome, ansi [16]color.NRGBA) *palInk {
	bg := th.ToastBG
	k := &palInk{
		bg:       bg,
		line:     th.ToastLine,
		outer:    color.NRGBA{A: 0x59},
		raised:   mix(bg, th.ToastFG, 0.08), // Ruling 2
		sel:      mix(bg, th.Accent, 0x21/255.),
		selBar:   th.Accent,
		foot:     mix(bg, th.Chrome, 0.6),
		switchBG: th.Chrome,
		chipBG:   mix(bg, th.Accent, 0x29/255.),
		chipLine: mix(bg, th.Accent, 0x59/255.),
		keyLine:  th.ToastLine,
		tileBG:   mix(bg, th.Accent, 0.12),
		hairline: tint(th.ToastGo, 0x73),
	}
	k.keyBG = k.raised
	k.text = readableOn(th.ToastFG, k.sel, bg, k.raised)
	k.muted = readableOn(th.Muted, k.sel, bg, k.foot, k.switchBG)
	k.faint = readableOn(mix(th.Muted, bg, 0.15), k.sel, bg, k.switchBG)
	k.goInk = readableOn(th.ToastGo, k.sel, bg, k.raised)
	k.badge = k.muted
	k.chipFG = render.Readable(th.ToastGo, k.chipBG)
	k.keyFG = render.Readable(th.CapGlyph, k.keyBG)
	k.tileFG = render.Readable(k.goInk, k.tileBG)
	k.pillNeedsBG = mix(bg, th.Needs, 0.16)
	k.pillNeedsFG = render.Readable(th.Needs, k.pillNeedsBG)
	k.pillRunBG = mix(bg, th.Accent, 0.16)
	k.pillRunFG = render.Readable(th.ToastGo, k.pillRunBG)
	k.pillIdleBG = mix(bg, th.Muted, 0.12)
	k.pillIdleFG = render.Readable(th.Muted, k.pillIdleBG)
	k.pillDoneBG = mix(bg, th.Done, 0.16)
	k.pillDoneFG = render.Readable(th.Done, k.pillDoneBG)
	for i := range k.plugins {
		// render.Readable has one bar, 4.5:1, so the tiles clear the 3:1 one for marks.
		k.plugins[i] = render.Readable(ansi[i+1], bg)
		k.pluginInk[i] = render.Readable(th.Chrome, k.plugins[i])
	}
	return k
}

// palPluginColor is the plugin's tile colour, an index into palInk.plugins:
// FNV-1a 32 of the name, mod 6, so a plugin keeps its colour.
func palPluginColor(plugin string) int {
	h := uint32(2166136261)
	for i := 0; i < len(plugin); i++ {
		h ^= uint32(plugin[i])
		h *= 16777619
	}
	return int(h % 6)
}
