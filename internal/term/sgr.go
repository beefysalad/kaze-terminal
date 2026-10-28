package term

import "github.com/patrick-gogo/kaze/internal/screen"

// sgr applies an SGR parameter list to the pen. Missing (-1) params mean 0.
func (a *Applier) sgr(params []int) {
	pen := a.s.Pen()
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
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

// extendedColor parses the operands after 38/48/58 and returns the color,
// how many params it consumed, and whether it was valid.
func extendedColor(rest []int) (screen.Color, int, bool) {
	if len(rest) == 0 {
		return 0, 0, false
	}
	switch rest[0] {
	case 5:
		if len(rest) < 2 {
			return 0, len(rest), false
		}
		if rest[1] < 0 || rest[1] > 255 {
			return 0, 2, false
		}
		return screen.Indexed(uint8(rest[1])), 2, true
	case 2:
		off := 1
		// The colon form 38:2:<colorspace>:r:g:b has one extra slot that looks
		// identical to the semicolon form once flattened. If at least 4 params
		// follow the 2 and the first is empty (-1), treat it as the colorspace.
		// A non-empty colorspace id is not detectable and is not supported.
		if len(rest)-1 >= 4 && rest[1] == -1 {
			off = 2
		}
		if len(rest) < off+3 {
			return 0, len(rest), false
		}
		r, g, b := rest[off], rest[off+1], rest[off+2]
		used := off + 3
		for _, v := range []int{r, g, b} {
			if v < 0 || v > 255 {
				return 0, used, false
			}
		}
		return screen.RGB(uint8(r), uint8(g), uint8(b)), used, true
	}
	return 0, 1, false
}
