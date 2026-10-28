package term

import "github.com/patrick-gogo/kaze/internal/screen"

// sgr applies an SGR parameter list to the pen. Missing (-1) params mean 0.
// Bit i of colon marks params[i] as a ':' sub-parameter of the code before it.
func (a *Applier) sgr(params []int, colon uint32) {
	pen := a.s.Pen()
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		// A group is a code plus the colon-marked params that follow it.
		end := i + 1
		for end < len(params) && end < 32 && colon&(1<<uint(end)) != 0 {
			end++
		}
		if end > i+1 {
			a.sgrGroup(pen, params[i:end])
			i = end - 1
			continue
		}
		p := params[i]
		switch {
		case p <= 0:
			pen.FG, pen.BG, pen.Attrs = screen.DefaultColor, screen.DefaultColor, 0
		case p == 1:
			pen.Attrs |= screen.AttrBold
		case p == 2:
			pen.Attrs |= screen.AttrDim
		case p == 3:
			pen.Attrs |= screen.AttrItalic
		case p == 4:
			pen.Attrs |= screen.AttrUnderline
		case p == 7:
			pen.Attrs |= screen.AttrInverse
		case p == 9:
			pen.Attrs |= screen.AttrStrike
		case p == 22:
			pen.Attrs &^= screen.AttrBold | screen.AttrDim
		case p == 23:
			pen.Attrs &^= screen.AttrItalic
		case p == 24:
			pen.Attrs &^= screen.AttrUnderline
		case p == 27:
			pen.Attrs &^= screen.AttrInverse
		case p == 29:
			pen.Attrs &^= screen.AttrStrike
		case p >= 30 && p <= 37:
			pen.FG = screen.Indexed(uint8(p - 30))
		case p == 39:
			pen.FG = screen.DefaultColor
		case p >= 40 && p <= 47:
			pen.BG = screen.Indexed(uint8(p - 40))
		case p == 49:
			pen.BG = screen.DefaultColor
		case p >= 90 && p <= 97:
			pen.FG = screen.Indexed(uint8(p - 90 + 8))
		case p >= 100 && p <= 107:
			pen.BG = screen.Indexed(uint8(p - 100 + 8))
		case p == 38 || p == 48:
			c, used, ok := extendedColor(params[i+1:])
			i += used
			if !ok {
				continue
			}
			if p == 38 {
				pen.FG = c
			} else {
				pen.BG = c
			}
		case p == 58:
			// Underline color is not rendered; consume its operands so they
			// are not read as SGR codes.
			_, used, _ := extendedColor(params[i+1:])
			i += used
		}
	}
}

// sgrGroup applies one colon group. Unknown groups, and 58 (underline
// color, not rendered), are skipped whole.
func (a *Applier) sgrGroup(pen *screen.Cell, g []int) {
	if len(g) < 2 {
		return
	}
	switch g[0] {
	case 4:
		switch {
		case g[1] <= 0: // 4: with an empty sub-param (-1) means 4:0
			pen.Attrs &^= screen.AttrUnderline
		case g[1] >= 1 && g[1] <= 5:
			pen.Attrs |= screen.AttrUnderline // every style draws as plain underline
		}
	case 38, 48:
		c, ok := colonColor(g[1:])
		if !ok {
			return
		}
		if g[0] == 38 {
			pen.FG = c
		} else {
			pen.BG = c
		}
	}
}

// extendedColor parses the semicolon-form operands after 38/48/58 and returns
// the color, how many params it consumed, and whether it was valid.
func extendedColor(rest []int) (screen.Color, int, bool) {
	if len(rest) == 0 {
		return 0, 0, false
	}
	switch rest[0] {
	case 5:
		if len(rest) < 2 {
			return 0, len(rest), false
		}
		return indexedColor(rest[1]), 2, rest[1] >= 0 && rest[1] <= 255
	case 2:
		if len(rest) < 4 {
			return 0, len(rest), false
		}
		c, ok := rgbColor(rest[1], rest[2], rest[3])
		return c, 4, ok
	}
	return 0, 1, false
}

func indexedColor(n int) screen.Color {
	if n < 0 || n > 255 {
		return 0
	}
	return screen.Indexed(uint8(n))
}

func rgbColor(r, g, b int) (screen.Color, bool) {
	for _, v := range [3]int{r, g, b} {
		if v < 0 || v > 255 {
			return 0, false
		}
	}
	return screen.RGB(uint8(r), uint8(g), uint8(b)), true
}

// colonColor parses a colon group after 38/48: 5:n, 2:r:g:b or 2:cs:r:g:b.
// The colorspace id is ignored.
func colonColor(sub []int) (screen.Color, bool) {
	switch {
	case len(sub) == 2 && sub[0] == 5 && sub[1] >= 0 && sub[1] <= 255:
		return screen.Indexed(uint8(sub[1])), true
	case len(sub) == 4 && sub[0] == 2:
		return rgbColor(sub[1], sub[2], sub[3])
	case len(sub) == 5 && sub[0] == 2:
		return rgbColor(sub[2], sub[3], sub[4])
	}
	return 0, false
}
