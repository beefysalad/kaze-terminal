package screen

import "strings"

// Text reads the live grid and up to history recent scrollback rows. Callers
// sharing a screen must hold its session lock. ViewOffset does not affect it.
func Text(s *Screen, history int) string {
	if s.OnAltScreen() {
		history = 0
	}
	history = min(max(history, 0), s.ScrollbackLen())
	_, rows := s.Size()
	var out, line strings.Builder
	for row := s.ScrollbackLen() - history; row < s.ScrollbackLen()+rows; row++ {
		var cells []Cell
		if row < s.ScrollbackLen() {
			cells = s.ScrollbackLine(row)
		} else {
			cells = s.row(row - s.ScrollbackLen())
		}
		wrapped := len(cells) > 0 && cells[len(cells)-1].Attrs&AttrWrapped != 0
		for _, c := range cells {
			if c.Width == 0 {
				continue
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			line.WriteRune(r)
		}
		if !wrapped || row == s.ScrollbackLen()+rows-1 {
			out.WriteString(strings.TrimRight(line.String(), " "))
			line.Reset()
			if row < s.ScrollbackLen()+rows-1 {
				out.WriteByte('\n')
			}
		}
	}
	return out.String()
}
