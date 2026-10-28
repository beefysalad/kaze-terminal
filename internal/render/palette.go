package render

import (
	"image/color"

	"github.com/patrick-gogo/kaze/internal/screen"
)

// Palette is the theme: default colors plus the 16 ANSI colors.
type Palette struct {
	FG, BG, Cursor, Selection color.NRGBA
	ANSI                      [16]color.NRGBA
}

// rgbFlag mirrors the truecolor bit that screen.RGB sets.
const rgbFlag screen.Color = 1 << 30

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

// DefaultPalette is the "Midnight" look.
func DefaultPalette() Palette {
	return Palette{
		FG:        hex(0xd7dae3),
		BG:        hex(0x0f1117),
		Cursor:    hex(0xb9a6ff),
		Selection: hex(0x2e2a52),
		ANSI: [16]color.NRGBA{
			hex(0x1d2230), hex(0xf07178), hex(0x62d39a), hex(0xf2b84b),
			hex(0x8ab4ff), hex(0xb9a6ff), hex(0x6fd3e0), hex(0xd7dae3),
			hex(0x5d6273), hex(0xff8b92), hex(0x8be3b4), hex(0xffd27a),
			hex(0xa9c7ff), hex(0xcdbfff), hex(0x95e6f0), hex(0xffffff),
		},
	}
}

// indexed maps 0-255: the theme's 16, then the xterm 6x6x6 cube and grayscale ramp.
func indexed(p Palette, n uint8) color.NRGBA {
	switch {
	case n < 16:
		return p.ANSI[n]
	case n < 232:
		n -= 16
		level := func(v uint8) uint8 {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return color.NRGBA{R: level(n / 36), G: level(n / 6 % 6), B: level(n % 6), A: 0xff}
	default:
		g := 8 + (n-232)*10
		return color.NRGBA{R: g, G: g, B: g, A: 0xff}
	}
}

func resolve(p Palette, c screen.Color, def color.NRGBA) color.NRGBA {
	switch {
	case c&screen.DefaultColor != 0:
		return def
	case c&rgbFlag != 0:
		return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
	default:
		return indexed(p, uint8(c))
	}
}

// resolveFG applies bold-is-bright to the 8 basic colors, as xterm does.
func resolveFG(p Palette, c screen.Cell) color.NRGBA {
	fg := c.FG
	if c.Attrs&screen.AttrBold != 0 && fg&(screen.DefaultColor|rgbFlag) == 0 && fg < 8 {
		fg += 8
	}
	return resolve(p, fg, p.FG)
}

// cellColors returns the colors a cell is drawn with after inverse and dim.
func cellColors(p Palette, c screen.Cell) (fg, bg color.NRGBA) {
	fg, bg = resolveFG(p, c), resolve(p, c.BG, p.BG)
	if c.Attrs&screen.AttrInverse != 0 {
		fg, bg = bg, fg
	}
	if c.Attrs&screen.AttrDim != 0 {
		fg = color.NRGBA{R: mid(fg.R, bg.R), G: mid(fg.G, bg.G), B: mid(fg.B, bg.B), A: 0xff}
	}
	return fg, bg
}

func mid(a, b uint8) uint8 { return uint8((int(a) + int(b)) / 2) }
