// Package screen is the terminal's cell grid with per-row dirty tracking.
package screen

// Screen is a flat row-major grid so rows are contiguous and nothing allocates per cell.
type Screen struct {
	cols, rows int
	cells      []Cell
	dirty      []bool
	curCol     int
	curRow     int

	pen         Cell // FG/BG/Attrs template for printed cells; Rune and Width are ignored
	autoWrap    bool
	wrapPending bool // cursor sits past the last column; the wrap happens on the next printable rune

	scrollTop, scrollBottom int // DECSTBM region, 0-based inclusive; the full screen by default
	saved                   savedCursor

	sb         scrollback
	viewOffset int // lines scrolled back from the bottom
}

// New returns a blank screen with every row dirty, so the first frame paints fully.
func New(cols, rows int) *Screen {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	s := &Screen{
		cols:  cols,
		rows:  rows,
		cells: make([]Cell, cols*rows),
		dirty: make([]bool, rows),
		pen:   blankCell,

		autoWrap:     true,
		scrollBottom: rows - 1,
		sb:           scrollback{max: defaultScrollback},
	}
	for i := range s.cells {
		s.cells[i] = blankCell
	}
	s.MarkAllDirty()
	return s
}

func (s *Screen) Size() (cols, rows int) { return s.cols, s.rows }

func (s *Screen) inBounds(col, row int) bool {
	return col >= 0 && col < s.cols && row >= 0 && row < s.rows
}

// Cell returns the cell at col,row, or a blank cell when out of range.
func (s *Screen) Cell(col, row int) Cell {
	if !s.inBounds(col, row) {
		return blankCell
	}
	return s.cells[row*s.cols+col]
}

// SetCell stores c and marks the row dirty only if the cell actually changed.
func (s *Screen) SetCell(col, row int, c Cell) {
	if !s.inBounds(col, row) {
		return
	}
	i := row*s.cols + col
	if s.cells[i] == c {
		return
	}
	s.cells[i] = c
	s.dirty[row] = true
}

func (s *Screen) Cursor() (col, row int) { return s.curCol, s.curRow }

// SetCursor moves the cursor, clamping to the grid.
func (s *Screen) SetCursor(col, row int) {
	s.curCol = clamp(col, 0, s.cols-1)
	s.curRow = clamp(row, 0, s.rows-1)
	s.wrapPending = false // any explicit cursor move cancels a pending wrap, as in xterm
}

// TakeDirty returns changed rows in ascending order and clears the flags.
func (s *Screen) TakeDirty() []int {
	var out []int
	for r, d := range s.dirty {
		if d {
			out = append(out, r)
			s.dirty[r] = false
		}
	}
	return out
}

func (s *Screen) MarkAllDirty() {
	for r := range s.dirty {
		s.dirty[r] = true
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
