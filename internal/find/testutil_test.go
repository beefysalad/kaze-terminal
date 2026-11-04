package find

import (
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/screen"
)

// screenWith prints lines (joined by newlines) on a cols x rows screen with a sb-line
// history. A '\n' is CR+LF; text past the width wraps as the terminal would.
func screenWith(cols, rows, sb int, lines ...string) *screen.Screen {
	scr := screen.NewWithScrollback(cols, rows, sb)
	printTo(scr, strings.Join(lines, "\n"))
	return scr
}

// printTo prints s on scr, a '\n' as CR+LF.
func printTo(scr *screen.Screen, s string) {
	for _, r := range s {
		if r == '\n' {
			scr.CarriageReturn()
			scr.LineFeed()
			continue
		}
		scr.Print(r)
	}
}

// settle steps s with the full budget until it settles, failing after 64 steps.
func settle(t testing.TB, s *Search, scr *screen.Screen) {
	t.Helper()
	for i := 0; i < 64; i++ {
		if s.Step(scr, StepBudget) {
			return
		}
	}
	t.Fatal("search did not settle in 64 steps")
}

// cellAt reads the cell at a stable position, or false when that line is not held.
func cellAt(scr *screen.Screen, p Pos) (screen.Cell, bool) {
	abs := p.Line - int(scr.Evicted())
	if abs < 0 {
		return screen.Cell{}, false
	}
	n := scr.ScrollbackLen()
	if abs < n {
		line := scr.ScrollbackLine(abs)
		if p.Col < len(line) {
			return line[p.Col], true
		}
		return screen.Cell{Rune: ' ', Width: 1}, true
	}
	abs -= n
	_, rows := scr.Size()
	if abs >= rows {
		return screen.Cell{}, false
	}
	return scr.Cell(p.Col, abs), true
}

// lineText is a stable line's text, trailing spaces trimmed ("" when not held).
func lineText(scr *screen.Screen, line int) string {
	cols, _ := scr.Size()
	width := cols
	if abs := line - int(scr.Evicted()); abs >= 0 && abs < scr.ScrollbackLen() {
		width = scr.ScrollbackWidth(abs)
	}
	var b strings.Builder
	for c := 0; c < width; c++ {
		cell, ok := cellAt(scr, Pos{line, c})
		if !ok {
			return ""
		}
		if cell.Width == 0 {
			continue
		}
		r := cell.Rune
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

// matchText is the text a single-row match covers.
func matchText(scr *screen.Screen, m Match) string {
	var b strings.Builder
	for c := m.Start.Col; c <= m.End.Col; c++ {
		cell, _ := cellAt(scr, Pos{m.Start.Line, c})
		if cell.Width != 0 {
			b.WriteRune(cell.Rune)
		}
	}
	return b.String()
}
