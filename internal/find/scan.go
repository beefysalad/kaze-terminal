package find

import "github.com/beefysalad/kaze-terminal/internal/screen"

// MaxMatches is how many matches are kept: the newest, nearest the live screen (Ruling 9).
const MaxMatches = 1000

// StepBudget is how many cells one Step may read under the session lock (Ruling 7).
const StepBudget = 2_000_000

// Span is one match's cells on one row, From..To inclusive; Match indexes Matches().
type Span struct{ Line, From, To, Match int }

// blank is an erased cell with default colours: a hard-ended row's trailing run of
// these is not text (Ruling 5).
var blank = screen.Cell{Rune: ' ', Width: 1, FG: screen.DefaultColor, BG: screen.DefaultColor}

// Search finds a needle in a screen's history and grid in bounded, resumable steps.
// The zero value is ready. It must not be copied after first use. Every method that
// takes a screen must run under the lock that guards that screen.
type Search struct {
	nd needle
	m  matcher

	// Matches in scan order (sorted), a FIFO ring: past MaxMatches the oldest goes.
	ring        []Match
	nsp         []int32 // spans per match, parallel to ring
	head, count int
	// The matches' spans, a FIFO ring in the same order.
	sp            []Span
	spHead, spLen int
	capped        bool
	lost          int // the End line of the newest match the cap dropped

	// Linearised views, rebuilt only when gen moved.
	out    []Match
	outSp  []Span
	linGen uint64

	next, col int  // the scan resumes at stable line next, column col
	settled   bool // nothing left to scan
	geomOK    bool // alt, cols and rows below are the scan's
	alt       bool
	cols      int
	rows      int
	liveFrom  int // stable line of live row 0 when the scan last read the screen

	gen     uint64
	scanned int
}

// init makes the rings once.
func (s *Search) init() {
	if s.ring != nil {
		return
	}
	s.ring = make([]Match, MaxMatches)
	s.nsp = make([]int32, MaxMatches)
	s.sp = make([]Span, 2*MaxMatches)
	s.out = make([]Match, 0, MaxMatches)
	s.outSp = make([]Span, 0, 2*MaxMatches)
}

// SetQuery sets the needle and reports whether it changed; a change restarts the scan.
func (s *Search) SetQuery(text string, matchCase bool) bool {
	if !s.nd.set(text, matchCase) {
		return false
	}
	s.init()
	s.clear()
	s.restart()
	return true
}

// clear drops every match.
func (s *Search) clear() {
	s.head, s.count, s.spHead, s.spLen = 0, 0, 0, 0
	s.capped = false
	s.gen++
}

// restart scans everything again from the oldest line held.
func (s *Search) restart() {
	s.m.nd = &s.nd
	s.m.reset()
	s.next, s.col = 0, 0
	s.settled = false
	s.geomOK = false
}

// checkGeom restarts the scan when the screen switched buffers or changed size
// since the scan began (Ruling 8): nothing found before can be trusted.
func (s *Search) checkGeom(scr *screen.Screen) {
	cols, rows := scr.Size()
	alt := scr.OnAltScreen()
	if s.geomOK && (alt != s.alt || cols != s.cols || rows != s.rows) {
		if s.count > 0 {
			s.clear()
		}
		s.restart()
	}
	if !s.geomOK {
		s.alt, s.cols, s.rows, s.geomOK = alt, cols, rows, true
	}
}

// dropEvicted forgets matches that started on a line the ring has since dropped.
func (s *Search) dropEvicted(evicted int) {
	dropped := false
	for s.count > 0 && s.ring[s.head].Start.Line < evicted {
		s.popFront()
		dropped = true
	}
	if s.capped && s.lost < evicted {
		s.capped = false // everything the cap dropped has left the history anyway
	}
	if dropped {
		s.gen++
	}
}

// Step scans from where the last one stopped until about budget cells were read (a
// row is never split, so it may pass the budget by one row) and reports whether the
// search has settled. A settled search reads nothing.
func (s *Search) Step(scr *screen.Screen, budget int) bool {
	if s.nd.n == 0 {
		s.settled = true
		return true
	}
	s.m.nd = &s.nd
	s.checkGeom(scr)
	evicted := int(scr.Evicted())
	s.dropEvicted(evicted)
	if s.settled {
		return true
	}
	held := scr.ScrollbackLen()
	start := evicted
	if s.alt {
		start = evicted + held // only the grid on the alt screen (Ruling 6)
	}
	end := evicted + held + s.rows
	if s.next < start {
		s.next, s.col = start, 0
		s.m.reset()
	}
	gen := s.gen
	used := 0
	budget = max(budget, 1)
	for s.next < end && used < budget {
		used += s.scanRow(scr, s.next-evicted, held)
		s.next++
		s.col = 0
	}
	s.scanned += used
	s.liveFrom = evicted + held
	if s.gen != gen {
		s.gen = gen + 1 // one bump per step however many matches it found
	}
	if s.next >= end {
		s.settled = true
	}
	return s.settled
}

// scanRow feeds absolute row abs (history first) from s.col and returns the cells it read.
func (s *Search) scanRow(scr *screen.Screen, abs, held int) int {
	line := s.next
	if abs < held {
		cells := scr.ScrollbackLine(abs)
		width := scr.ScrollbackWidth(abs)
		n := len(cells)
		wrapped := n > 0 && n == width && cells[n-1].Attrs&screen.AttrWrapped != 0
		if wrapped && isPad(cells[n-1]) && s.nextLeadIsWide(scr, abs+1, held) {
			n--
		}
		for c := s.col; c < n; c++ {
			s.feedCell(cells[c], line, c)
		}
		if !wrapped {
			s.m.reset()
		}
		return max(len(cells), 1)
	}
	row := abs - held
	cols := s.cols
	wrapped := scr.Cell(cols-1, row).Attrs&screen.AttrWrapped != 0
	n := cols
	if !wrapped {
		for n > 0 && scr.Cell(n-1, row) == blank {
			n--
		}
	} else if isPad(scr.Cell(cols-1, row)) && s.nextLeadIsWide(scr, abs+1, held) {
		n--
	}
	for c := s.col; c < n; c++ {
		s.feedCell(scr.Cell(c, row), line, c)
	}
	if !wrapped {
		s.m.reset()
	}
	return cols
}

// isPad reports a blank left at the right edge when a wide char did not fit there.
func isPad(c screen.Cell) bool { return c.Rune == ' ' && c.Width == 1 }

// nextLeadIsWide reports whether absolute row abs starts with a wide char.
func (s *Search) nextLeadIsWide(scr *screen.Screen, abs, held int) bool {
	if abs < held {
		cells := scr.ScrollbackLine(abs)
		return len(cells) > 0 && cells[0].Width == 2
	}
	return abs-held < s.rows && scr.Cell(0, abs-held).Width == 2
}

// feedCell feeds one cell; a continuation half is skipped, a null reads as a space.
func (s *Search) feedCell(c screen.Cell, line, col int) {
	if c.Width == 0 {
		return
	}
	r := c.Rune
	if r == 0 {
		r = ' '
	}
	start := Pos{line, col}
	end := start
	if c.Width == 2 {
		end.Col++
	}
	if mt, ok := s.m.feed(r, start, end); ok {
		s.add(mt)
	}
}

// add appends a match and its spans, dropping the oldest past MaxMatches.
func (s *Search) add(mt Match) {
	if s.count == MaxMatches {
		s.capped = true
		s.lost = s.ring[s.head].End.Line
		s.popFront()
	}
	n := int32(0)
	for j := 0; j < s.nd.n; j++ {
		st, en := s.m.cell(j)
		if n > 0 {
			last := s.spAt(s.spLen - 1)
			if last.Line == st.Line {
				last.To = en.Col
				continue
			}
		}
		s.pushSpan(Span{Line: st.Line, From: st.Col, To: en.Col})
		n++
	}
	i := s.head + s.count
	if i >= MaxMatches {
		i -= MaxMatches
	}
	s.ring[i], s.nsp[i] = mt, n
	s.count++
	s.gen++
}

// popFront drops the oldest match and its spans.
func (s *Search) popFront() {
	n := int(s.nsp[s.head])
	s.spHead = (s.spHead + n) % len(s.sp)
	s.spLen -= n
	s.head++
	if s.head == MaxMatches {
		s.head = 0
	}
	s.count--
}

// popBack drops the newest match and its spans.
func (s *Search) popBack() {
	i := (s.head + s.count - 1) % MaxMatches
	s.spLen -= int(s.nsp[i])
	s.count--
}

// back is the newest match; count must be > 0.
func (s *Search) back() Match { return s.ring[(s.head+s.count-1)%MaxMatches] }

func (s *Search) spAt(i int) *Span { return &s.sp[(s.spHead+i)%len(s.sp)] }

// pushSpan appends a span, growing the span ring only when matches cover many rows.
func (s *Search) pushSpan(sp Span) {
	if s.spLen == len(s.sp) {
		grown := make([]Span, 2*len(s.sp))
		for i := 0; i < s.spLen; i++ {
			grown[i] = *s.spAt(i)
		}
		s.sp, s.spHead = grown, 0
	}
	*s.spAt(s.spLen) = sp
	s.spLen++
}

// lin rebuilds the linearised matches and spans when they changed.
func (s *Search) lin() {
	if s.linGen == s.gen {
		return
	}
	s.linGen = s.gen
	s.out = s.out[:0]
	if cap(s.outSp) < s.spLen {
		s.outSp = make([]Span, 0, len(s.sp))
	}
	s.outSp = s.outSp[:0]
	k := 0
	for i := 0; i < s.count; i++ {
		j := (s.head + i) % MaxMatches
		s.out = append(s.out, s.ring[j])
		for n := int(s.nsp[j]); n > 0; n-- {
			sp := *s.spAt(k)
			sp.Match = i
			s.outSp = append(s.outSp, sp)
			k++
		}
	}
}

// Matches is every match kept, sorted; the slice is reused, valid until the next Output or Step.
func (s *Search) Matches() []Match {
	s.lin()
	return s.out
}

// Spans is every match's cells per row, sorted by Line then From; a match across a
// soft wrap has one span per row. Reused like Matches.
func (s *Search) Spans() []Span {
	s.lin()
	return s.outSp
}

// Capped reports that more than MaxMatches were found and the oldest were dropped.
func (s *Search) Capped() bool { return s.capped }

// Gen changes whenever Matches changes.
func (s *Search) Gen() uint64 { return s.gen }

// Scanned counts the cells read since the zero value.
func (s *Search) Scanned() int { return s.scanned }
