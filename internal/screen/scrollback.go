package screen

const defaultScrollback = 10000

// scrollback is a ring of fixed-width lines; once full, the oldest line's slice is reused for the newest.
type scrollback struct {
	lines [][]Cell
	start int // index of the oldest line
	count int
	max   int
	// evicted counts lines dropped off the top; it never goes back, so callers can diff it.
	evicted uint64
}

// push copies row in, reusing the evicted line's storage when the ring is full.
func (b *scrollback) push(row []Cell) {
	if b.max <= 0 {
		b.evicted++
		return
	}
	if b.count < b.max {
		b.lines = append(b.lines, append([]Cell(nil), row...))
		b.count++
		return
	}
	b.evicted++
	old := b.lines[b.start]
	if cap(old) >= len(row) {
		old = old[:len(row)]
	} else {
		old = make([]Cell, len(row))
	}
	copy(old, row)
	b.lines[b.start] = old
	b.start = (b.start + 1) % b.max
}

func (b *scrollback) at(i int) []Cell { return b.lines[(b.start+i)%len(b.lines)] }

// NewWithScrollback is New with an explicit history cap in lines.
func NewWithScrollback(cols, rows, maxLines int) *Screen {
	s := New(cols, rows)
	if maxLines < 0 {
		maxLines = 0
	}
	s.sb.max = maxLines
	return s
}

// ScrollbackLen is the number of history lines currently held.
func (s *Screen) ScrollbackLen() int { return s.sb.count }

// ScrollbackLine returns history line i (0 = oldest) at the width it had when scrolled off. Callers must not modify it.
func (s *Screen) ScrollbackLine(i int) []Cell {
	if i < 0 || i >= s.sb.count {
		return nil
	}
	return s.sb.at(i)
}

// SetViewOffset scrolls the view n lines back from the bottom, clamped to the history size.
func (s *Screen) SetViewOffset(n int) {
	s.viewOffset = clamp(n, 0, s.sb.count)
	s.MarkAllDirty()
}

func (s *Screen) ViewOffset() int { return s.viewOffset }

// VisibleCell is the cell the renderer draws at col,row once the view offset is applied.
func (s *Screen) VisibleCell(col, row int) Cell {
	if !s.inBounds(col, row) {
		return blankCell
	}
	idx := s.sb.count - s.viewOffset + row // virtual line: history first, then the live screen
	if idx >= s.sb.count {
		return s.Cell(col, idx-s.sb.count)
	}
	line := s.sb.at(idx)
	if col >= len(line) {
		return blankCell
	}
	return line[col]
}

// recordScrolled saves the top n rows before scrollUp discards them, keeping a scrolled-back view anchored.
func (s *Screen) recordScrolled(n int) {
	for r := 0; r < n; r++ {
		s.sb.push(s.cells[r*s.cols : (r+1)*s.cols])
	}
	if s.viewOffset > 0 {
		s.viewOffset = min(s.viewOffset+n, s.sb.count) // a full ring evicts one line per push, so count alone can't track it
	}
}

// Evicted counts history lines dropped off the top since the screen was made:
// ring overflow, a zero-size ring, and ED 3. Absolute rows shift down by its growth.
func (s *Screen) Evicted() uint64 { return s.sb.evicted }
