package render

import (
	"image/color"

	"github.com/beefysalad/kaze-terminal/internal/screen"
)

// MatchSpan is one find match's cells on one row. Line is a stable line
// (screen.Evicted() plus the absolute row), From and To inclusive columns;
// Match is the match's index, compared with Marks.Cur.
type MatchSpan struct {
	Line, From, To int
	Match          int32
}

// Marks are the find highlights the renderer draws over the grid.
type Marks struct {
	Spans []MatchSpan // sorted by Line, From
	Cur   int32       // the current match's index; -1 none
	Gen   uint64      // the caller bumps it whenever Spans or Cur change
}

// Per-cell highlight values in snapHit and drawnHit.
const (
	hitNone uint8 = iota
	hitMatch
	hitCur
)

// SetMarks sets the find highlights, nil for none. It only stores the pointer;
// the marks are read in Layout's snapshot, under the session lock, so the
// caller must not change them while Layout runs.
func (r *Renderer) SetMarks(m *Marks) { r.marks = m }

// queueMarkRows queues every visible row when the highlights changed or moved
// (the cell diff keeps that cheap), and reports whether the queued rows need
// their highlights recomputed. With no marks now or before it does nothing.
func (r *Renderer) queueMarkRows(top int) bool {
	m := r.marks
	if m == nil {
		if !r.hadMarks {
			return false
		}
		r.hadMarks = false // clear the tint once
	} else {
		if r.hadMarks && m.Gen == r.markGen && m.Cur == r.markCur && top == r.markTop {
			return true // unchanged: only rows queued for other reasons
		}
		r.hadMarks, r.markGen, r.markCur, r.markTop = true, m.Gen, m.Cur, top
	}
	r.snapRows = r.snapRows[:0]
	for row := 0; row < r.rows; row++ {
		r.snapRows = append(r.snapRows, row)
	}
	return true
}

// fillHits sets one visible row's highlight values from the marks; line is
// that row's stable line and cells its copied cells.
func (r *Renderer) fillHits(hits []uint8, cells []screen.Cell, line int) {
	clear(hits)
	m := r.marks
	if m == nil {
		return
	}
	sp := m.Spans
	lo, hi := 0, len(sp) // first span with Line >= line; no closure, so nothing escapes
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if sp[mid].Line < line {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	for i := lo; i < len(sp) && sp[i].Line == line; i++ {
		v := hitMatch
		if sp[i].Match == m.Cur {
			v = hitCur
		}
		for c := max(sp[i].From, 0); c <= min(sp[i].To, len(hits)-1); c++ {
			hits[c] = max(hits[c], v)
		}
	}
	for c := range cells {
		// A wide char is highlighted as a whole, as the selection does.
		if cells[c].Width == 2 && c+1 < len(cells) {
			v := max(hits[c], hits[c+1])
			hits[c], hits[c+1] = v, v
		}
	}
}

// findInk are the find colours this renderer draws with, worked out once.
type findInk struct {
	ready                bool
	match, cur, mark, on color.NRGBA // on: the text on the current match
}

func (r *Renderer) findColors() *findInk {
	if !r.find.ready {
		f := &r.find
		f.match, f.cur, f.mark = r.pal.matchColors()
		f.on = curText(f.cur, r.ink)
		f.ready = true
	}
	return &r.find
}

// hitColors are a highlighted cell's colours: the current match in one solid
// ink, a match with the cell's own text colour kept readable on the tint.
func (r *Renderer) hitColors(c screen.Cell, hit uint8) colors {
	f := r.findColors()
	if hit == hitCur {
		return colors{fg: f.on, bg: f.cur}
	}
	fg, _ := cellColors(r.pal, c)
	if fg == f.match {
		fg = r.ink // text in the tint's own colour is not hidden on purpose here
	}
	return colors{fg: r.readableCached(fg, f.match), bg: f.match}
}
