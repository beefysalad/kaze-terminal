package ui

import (
	"hash/fnv"
	"image"
	"time"

	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/screen"
)

const (
	// multiClick is the longest gap between clicks of a double or triple click.
	multiClick = 400 * time.Millisecond
	// autoScrollEvery caps how fast a drag past the grid's edge scrolls the view.
	autoScrollEvery = 40 * time.Millisecond
)

// gridCell maps a pixel inside the grid to its cell, clamped to the grid.
func gridCell(p image.Point, cw, ch, cols, rows int) (col, row int) {
	if cw <= 0 || ch <= 0 {
		return 0, 0
	}
	return min(max(p.X/cw, 0), cols-1), min(max(p.Y/ch, 0), rows-1)
}

// absPoint maps a pixel inside the grid to an absolute point. base is the
// absolute row of visible row 0 (ScrollbackLen - ViewOffset), as the renderer uses.
func absPoint(p image.Point, cw, ch, cols, rows, base int) render.Point {
	col, row := gridCell(p, cw, ch, cols, rows)
	return render.Point{Col: col, Row: base + row}
}

// clicks counts quick presses on one cell: 1, 2, 3, then back to 1.
type clicks struct {
	at  time.Duration
	pt  render.Point
	n   int
	any bool
}

func (c *clicks) press(t time.Duration, p render.Point) int {
	if c.any && p == c.pt && t >= c.at && t-c.at <= multiClick {
		c.n = c.n%3 + 1
	} else {
		c.n = 1
	}
	c.at, c.pt, c.any = t, p, true
	return c.n
}

// xtermButton is the report number of a single button: 0 left, 1 middle, 2 right.
func xtermButton(b pointer.Buttons) int {
	switch {
	case b.Contain(pointer.ButtonPrimary):
		return 0
	case b.Contain(pointer.ButtonTertiary):
		return 1
	case b.Contain(pointer.ButtonSecondary):
		return 2
	}
	return -1
}

// pressedButton is the button that went down between two button sets.
func pressedButton(prev, now pointer.Buttons) int { return xtermButton(now &^ prev) }

// releasedButton is the button that went up; Gio's release lists only the buttons still held.
func releasedButton(prev, now pointer.Buttons) int { return xtermButton(prev &^ now) }

// edgeDir is -1 above the grid, 1 below it, 0 inside; a drag outside scrolls that way.
func edgeDir(y, height int) int {
	switch {
	case y < 0:
		return -1
	case y >= height:
		return 1
	}
	return 0
}

type selMode int

const (
	selCell selMode = iota
	selWord
	selLine
)

// tabSel is one tab's selection plus what it was made against, so check can
// tell when the screen moved or changed under it.
type tabSel struct {
	sel      render.Selection
	mode     selMode
	anchor   render.Selection // the cell, word or line the gesture started on
	dragging bool

	evicted    uint64 // screen.Evicted when last stamped
	alt        bool
	cols, rows int
	liveFrom   int    // first absolute row that was on the live screen; older rows never change
	sum        uint64 // hash of the selected text on rows >= liveFrom
}

func (ts *tabSel) clear() {
	ts.sel = render.Selection{}
	ts.dragging = false
}

// begin starts a gesture with the n-th quick click at p. One click only clears;
// two select the word, three the line.
func (ts *tabSel) begin(scr *screen.Screen, p render.Point, n int) {
	ts.dragging = true
	switch n {
	case 2:
		ts.mode, ts.anchor = selWord, render.WordAt(scr, p)
		ts.sel = ts.anchor
	case 3:
		ts.mode, ts.anchor = selLine, render.LineAt(scr, p)
		ts.sel = ts.anchor
	default:
		ts.mode = selCell
		ts.anchor = render.Selection{Start: p, End: p}
		ts.sel = render.Selection{}
	}
	ts.stamp(scr)
}

func less(a, b render.Point) bool { return a.Row < b.Row || (a.Row == b.Row && a.Col < b.Col) }

func ordered(s render.Selection) (lo, hi render.Point) {
	if less(s.End, s.Start) {
		return s.End, s.Start
	}
	return s.Start, s.End
}

// extend moves the dragged end to p, by cells, words or lines as the gesture began.
func (ts *tabSel) extend(scr *screen.Screen, p render.Point) {
	if !ts.dragging {
		return
	}
	var unit render.Selection
	switch ts.mode {
	case selWord:
		unit = render.WordAt(scr, p)
	case selLine:
		unit = render.LineAt(scr, p)
	default:
		if !ts.sel.Active && p == ts.anchor.Start {
			return // still on the pressed cell: a click, not a drag
		}
		ts.sel = render.Selection{Start: ts.anchor.Start, End: p, Active: true}
		ts.stamp(scr)
		return
	}
	alo, ahi := ordered(ts.anchor)
	ulo, uhi := ordered(unit)
	if less(ulo, alo) {
		alo = ulo
	}
	if less(ahi, uhi) {
		ahi = uhi
	}
	ts.sel = render.Selection{Start: alo, End: ahi, Active: true}
	ts.stamp(scr)
}

func (ts *tabSel) end() { ts.dragging = false }

// stamp records the screen state the selection now belongs to.
func (ts *tabSel) stamp(scr *screen.Screen) {
	ts.evicted = scr.Evicted()
	ts.alt = scr.OnAltScreen()
	ts.cols, ts.rows = scr.Size()
	ts.liveFrom = scr.ScrollbackLen()
	ts.sum = ts.hash(scr)
}

// check keeps the selection on its text: it shifts rows by lines evicted from
// history and clears the selection on an alt-screen switch, a resize, or a
// change to the selected text. It reports whether the selection changed.
func (ts *tabSel) check(scr *screen.Screen) bool {
	if !ts.sel.Active && !ts.dragging {
		return false
	}
	before := ts.sel
	if ev := scr.Evicted(); ev != ts.evicted {
		d := ev - ts.evicted
		ts.evicted = ev
		if !ts.shift(d) {
			ts.clear()
			return before.Active
		}
	}
	cols, rows := scr.Size()
	if scr.OnAltScreen() != ts.alt || cols != ts.cols || rows != ts.rows || ts.hash(scr) != ts.sum {
		ts.clear()
		return before.Active
	}
	return ts.sel != before
}

// shift moves the stored rows up by d evicted lines. It reports false when
// the selection, or a drag's anchor, fell off the top entirely.
func (ts *tabSel) shift(d uint64) bool {
	if d > 1<<30 { // far more than history plus a screen holds
		return false
	}
	n := int(d)
	move := func(s *render.Selection) bool {
		lo, hi := ordered(*s)
		lo.Row -= n
		hi.Row -= n
		if hi.Row < 0 {
			return false
		}
		if lo.Row < 0 {
			lo = render.Point{}
		}
		s.Start, s.End = lo, hi
		return true
	}
	if ts.sel.Active && !move(&ts.sel) {
		return false
	}
	if ts.dragging && !move(&ts.anchor) {
		return false
	}
	ts.liveFrom = max(ts.liveFrom-n, 0)
	return true
}

// absCell reads the cell at an absolute row; outside a history line's width it is blank.
func absCell(scr *screen.Screen, col, row int) screen.Cell {
	sb := scr.ScrollbackLen()
	if row >= sb {
		return scr.Cell(col, row-sb)
	}
	line := scr.ScrollbackLine(row)
	if col < 0 || col >= len(line) {
		return screen.Cell{Rune: ' ', Width: 1}
	}
	return line[col]
}

// hash covers the selected cells on rows that were live when stamped. Older
// rows sit in history, which never changes in place, so this is at most one screen of cells.
func (ts *tabSel) hash(scr *screen.Screen) uint64 {
	if !ts.sel.Active {
		return 0
	}
	h := fnv.New64a()
	lo, hi := ordered(ts.sel)
	cols, rows := scr.Size()
	last := min(hi.Row, scr.ScrollbackLen()+rows-1)
	var buf [5]byte
	for row := max(lo.Row, ts.liveFrom, 0); row <= last; row++ {
		from, to := 0, cols-1
		if row == lo.Row {
			from = lo.Col
		}
		if row == hi.Row {
			to = min(hi.Col, cols-1)
		}
		for c := from; c <= to; c++ {
			cell := absCell(scr, c, row)
			r := uint32(cell.Rune)
			buf[0], buf[1], buf[2], buf[3], buf[4] = byte(r), byte(r>>8), byte(r>>16), byte(r>>24), byte(cell.Width)
			h.Write(buf[:])
		}
		buf[0] = '\n'
		h.Write(buf[:1])
	}
	return h.Sum64()
}
