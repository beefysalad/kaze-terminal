package screen

// Color packs a palette index (0-255), a truecolor value or the default color.
type Color uint32

const (
	// DefaultColor means "use the theme's foreground/background".
	DefaultColor Color = 1 << 31
	rgbFlag      Color = 1 << 30
)

// Indexed returns palette color n.
func Indexed(n uint8) Color { return Color(n) }

// RGB returns a truecolor value.
func RGB(r, g, b uint8) Color {
	return rgbFlag | Color(r)<<16 | Color(g)<<8 | Color(b)
}

// Attr is a bit set of text attributes.
type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrItalic
	AttrUnderline
	AttrInverse
	AttrDim
	AttrStrike
)

// Cell is one grid position. Width is 1, 2 (lead half of a wide char) or 0 (continuation half).
type Cell struct {
	Rune   rune
	Width  uint8
	FG, BG Color
	Attrs  Attr
}

// blankCell is what an erased position looks like.
var blankCell = Cell{Rune: ' ', Width: 1, FG: DefaultColor, BG: DefaultColor}
