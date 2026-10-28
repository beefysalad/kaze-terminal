// Package term connects the escape-sequence parser to the screen.
package term

import (
	"io"

	"github.com/patrick-gogo/kaze/internal/screen"
)

// Applier is a vt.Handler that drives a Screen.
type Applier struct {
	s     *screen.Screen
	reply io.Writer // terminal responses; written to by later tasks
}

func NewApplier(s *screen.Screen, reply io.Writer) *Applier {
	return &Applier{s: s, reply: reply}
}

func (a *Applier) Print(r rune) { a.s.Print(r) }

// Execute handles the C0 controls it knows and ignores the rest (NUL, BEL, ...).
func (a *Applier) Execute(b byte) {
	s := a.s
	switch b {
	case 0x08:
		c, r := s.Cursor()
		s.SetCursor(c-1, r)
	case 0x09:
		s.TabForward(1)
	case 0x0A, 0x0B, 0x0C:
		s.LineFeed()
	case 0x0D:
		s.CarriageReturn()
	}
}

// param returns params[i], or def when it is missing or zero.
func param(params []int, i, def int) int {
	if i >= len(params) || params[i] <= 0 {
		return def
	}
	return params[i]
}

// selector is the erase mode, where a missing parameter means 0.
func selector(params []int) int {
	if len(params) == 0 || params[0] < 0 {
		return 0
	}
	return params[0]
}

func (a *Applier) CSI(params []int, intermediates []byte, private byte, final byte) {
	if private != 0 || len(intermediates) != 0 {
		return // modes and other extensions belong to later tasks
	}
	s := a.s
	col, row := s.Cursor()
	_, rows := s.Size()
	n := param(params, 0, 1)
	switch final {
	case 'A':
		s.SetCursor(col, row-n)
	case 'B':
		s.SetCursor(col, row+n)
	case 'C':
		s.SetCursor(col+n, row)
	case 'D':
		s.SetCursor(col-n, row)
	case 'E':
		s.SetCursor(0, row+n)
	case 'F':
		s.SetCursor(0, row-n)
	case 'G':
		s.SetCursor(n-1, row)
	case 'd':
		s.SetCursor(col, n-1)
	case 'H', 'f':
		s.SetCursor(param(params, 1, 1)-1, param(params, 0, 1)-1)
	case 'J':
		s.EraseInDisplay(selector(params))
	case 'K':
		s.EraseInLine(selector(params))
	case 'L':
		s.InsertLines(n)
	case 'M':
		s.DeleteLines(n)
	case 'P':
		s.DeleteChars(n)
	case '@':
		s.InsertChars(n)
	case 'X':
		s.EraseChars(n)
	case 'S':
		s.ScrollUp(n)
	case 'T':
		s.ScrollDown(n)
	case 'r':
		s.SetScrollRegion(param(params, 0, 1)-1, param(params, 1, rows)-1)
	case 's':
		s.SaveCursor()
	case 'u':
		s.RestoreCursor()
	}
}

func (a *Applier) ESC(intermediates []byte, final byte) {
	if len(intermediates) != 0 {
		return
	}
	s := a.s
	switch final {
	case '7':
		s.SaveCursor()
	case '8':
		s.RestoreCursor()
	case 'D':
		s.LineFeed()
	case 'E':
		s.CarriageReturn()
		s.LineFeed()
	case 'M':
		s.ReverseIndex()
	case 'c':
		s.Reset()
	}
}

func (a *Applier) OSC(data []byte) {}
