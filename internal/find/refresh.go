package find

import "github.com/beefysalad/kaze-terminal/internal/screen"

// Output tells the search that output arrived since the last call; run it under the
// screen's lock. Matches whose first line left the history are dropped. A buffer
// switch or a resize rescans everything (Ruling 8). Otherwise history never changes
// in place, so only the rows that were live when the scan last read the screen, and
// the rows a match could reach back from them, are scanned again by the next Step.
func (s *Search) Output(scr *screen.Screen) {
	if s.nd.n == 0 {
		return
	}
	s.m.nd = &s.nd
	s.checkGeom(scr)
	s.dropEvicted(int(scr.Evicted()))
	// A match spans at most n rows (every row it crosses holds at least one of its
	// runes), so one starting above from ends above liveFrom, in unchanged history.
	from := s.liveFrom - (s.nd.n + 1)
	if s.next < from || (s.next == from && s.col == 0) {
		return // the scan has not read past from yet
	}
	dropped := false
	for s.count > 0 && s.back().Start.Line >= from {
		s.popBack()
		dropped = true
	}
	if dropped {
		s.gen++
	}
	// Resume where a left-to-right scan of the kept text would be: right after the
	// newest kept match when it runs into from (the matcher is empty after a match),
	// else at from with an empty matcher.
	s.next, s.col = from, 0
	if s.count > 0 {
		if end := s.back().End; end.Line >= from {
			s.next, s.col = end.Line, end.Col+1
		}
	}
	s.m.reset()
	s.settled = false
}

// Nearest is the match to make current for a view showing stable lines top..bottom:
// the first starting in view, else the last above it, else the first below; -1 with
// no matches (Ruling 11).
func (s *Search) Nearest(top, bottom int) int {
	ms := s.Matches()
	if len(ms) == 0 {
		return -1
	}
	i := s.search(Pos{top, 0})
	switch {
	case i < len(ms) && ms[i].Start.Line <= bottom:
		return i
	case i > 0:
		return i - 1
	default:
		return i
	}
}

// Locate is the match starting at p, else the first after p, else the last; -1 with none.
func (s *Search) Locate(p Pos) int {
	ms := s.Matches()
	if len(ms) == 0 {
		return -1
	}
	return min(s.search(p), len(ms)-1)
}

// search is the index of the first match starting at or after p.
func (s *Search) search(p Pos) int {
	ms := s.out
	lo, hi := 0, len(ms)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if st := ms[mid].Start; st.Line < p.Line || (st.Line == p.Line && st.Col < p.Col) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}
