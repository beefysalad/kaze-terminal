package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/find"
	"github.com/beefysalad/kaze-terminal/internal/render"
	"github.com/beefysalad/kaze-terminal/internal/screen"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// findWork is the search behind the bar: it runs only while the bar is open,
// once per change of the text or Aa, and on new output at most once a frame
// (coalesced by the session's OutputSeq). An unchanged frame returns before
// taking the session lock and allocates nothing (Review Focus 2).
type findWork struct {
	search find.Search
	marks  render.Marks

	tab    tabs.TabID // the pane the search ran on; another pane starts over
	member bool
	fresh  bool // the bar opened again: search from scratch

	seq        uint64 // the session's OutputSeq the search last saw
	resizeSeq  uint64 // the session's ResizeSeq: the pane may be sized apart from the shared grid
	cols, rows int    // the view's grid the search last saw
	set        bool   // text and matchCase were given to the search
	text       string
	matchCase  bool
	settled    bool
	gen        uint64 // the search's Gen the marks were built from

	cur    int      // the current match's index, -1 none
	curPos find.Pos // its start, so the same text stays current as output arrives
	pick   bool     // choose the current match nearest the view once the scan settles
	reveal bool     // scroll the current match into view once it is known

	queries, locks, marksGiven, outputs int // counters for tests
}

// findTick runs the search for one frame while the bar is open (Ruling 7, 8, 11).
func (win *window) findTick(gtx layout.Context) {
	f := &win.find
	if !f.open {
		return
	}
	v, ok := win.findView()
	if !ok {
		return // findCheck closes it
	}
	w := &f.work
	s := v.tab.Session
	seq := s.OutputSeq()
	rs := s.ResizeSeq()
	if w.set && !w.fresh && w.tab == v.tab.ID && w.member == f.member &&
		w.text == f.text && w.matchCase == f.matchCase && seq == w.seq &&
		v.cols == w.cols && v.rows == w.rows && rs == w.resizeSeq && w.settled && !w.pick && !w.reveal &&
		w.search.Gen() == w.gen {
		return // a still frame: no lock, no allocation
	}
	w.locks++
	s.Lock()
	scr := s.Screen()
	if w.fresh || w.tab != v.tab.ID || w.member != f.member {
		// Another pane, or opened again: nothing found before is known to hold.
		w.search.SetQuery("", false)
		w.tab, w.member, w.fresh, w.set = v.tab.ID, f.member, false, false
		w.cur, w.pick, w.reveal = -1, true, false
	}
	if !w.set || w.text != f.text || w.matchCase != f.matchCase {
		changed := w.set
		w.text, w.matchCase, w.set = f.text, f.matchCase, true
		w.queries++
		w.search.SetQuery(w.text, w.matchCase)
		w.cur, w.pick = -1, true
		w.reveal = w.reveal || changed // a typed change or Aa scrolls; opening does not
	}
	if seq != w.seq || v.cols != w.cols || v.rows != w.rows || rs != w.resizeSeq {
		w.outputs++
		w.search.Output(scr)
		w.seq, w.cols, w.rows, w.resizeSeq = seq, v.cols, v.rows, rs
	}
	w.settled = w.search.Step(scr, find.StepBudget)
	if g := w.search.Gen(); g != w.gen || (w.pick && w.settled) {
		w.gen = g
		w.pickCurrent(scr)
	}
	if w.reveal && w.settled {
		w.reveal = false
		w.revealCurrent(scr)
	}
	s.Unlock()
	if !w.settled {
		gtx.Execute(op.InvalidateCmd{}) // the next frame scans on; no timer
	}
}

// pickCurrent sets the current match after the matches changed: the one nearest
// the view after a new query, else the one at the same position (or the next
// after it, once its text is gone), and copies the spans into the marks.
func (w *findWork) pickCurrent(scr *screen.Screen) {
	ms := w.search.Matches()
	switch {
	case len(ms) == 0:
		w.cur = -1
		if w.settled {
			w.pick = false
		}
	case w.pick && w.settled:
		top, bottom := viewSpan(scr)
		w.cur, w.pick = w.search.Nearest(top, bottom), false
	case w.pick:
		w.cur = -1 // still scanning: chosen once the whole buffer is known
	default:
		w.cur = w.search.Locate(w.curPos)
	}
	if w.cur >= 0 {
		w.curPos = ms[w.cur].Start
	}
	if w.marks.Spans == nil {
		w.marks.Spans = make([]render.MatchSpan, 0, 2*find.MaxMatches)
	}
	w.marks.Spans = w.marks.Spans[:0]
	for _, sp := range w.search.Spans() {
		w.marks.Spans = append(w.marks.Spans, render.MatchSpan{Line: sp.Line, From: sp.From, To: sp.To, Match: int32(sp.Match)})
	}
	w.marks.Cur = int32(w.cur)
	w.marks.Gen++
}

// viewSpan is the stable lines of the view's first and last rows.
func viewSpan(scr *screen.Screen) (top, bottom int) {
	_, rows := scr.Size()
	top = int(scr.Evicted()) + scr.ScrollbackLen() - scr.ViewOffset()
	return top, top + rows - 1
}

// revealCurrent scrolls so the current match's first line is in the middle
// row, when it is not in view already. The alt screen has nothing to scroll.
func (w *findWork) revealCurrent(scr *screen.Screen) {
	ms := w.search.Matches()
	if w.cur < 0 || w.cur >= len(ms) || scr.OnAltScreen() {
		return
	}
	m := ms[w.cur]
	top, bottom := viewSpan(scr)
	if m.Start.Line >= top && m.End.Line <= bottom {
		return
	}
	_, rows := scr.Size()
	newTop := m.Start.Line - rows/2
	scr.SetViewOffset(int(scr.Evicted()) + scr.ScrollbackLen() - newTop) // clamped
}

// findStep makes the next (dir 1) or previous (dir -1) match current, wrapping
// around, and scrolls it into view.
func (win *window) findStep(dir int) {
	w := &win.find.work
	ms := w.search.Matches()
	n := len(ms)
	if n == 0 || w.pick {
		return
	}
	switch {
	case w.cur < 0 && dir > 0:
		w.cur = 0
	case w.cur < 0:
		w.cur = n - 1
	default:
		w.cur = (w.cur + dir + n) % n
	}
	w.curPos = ms[w.cur].Start
	w.marks.Cur = int32(w.cur)
	w.marks.Gen++
	w.reveal = true
	win.invalidate()
}

// findMarksFor is the highlights for the pane showing id (member: a docked
// member tab), nil unless the bar is open there with some text: the other pane
// and a closed bar draw nothing.
func (win *window) findMarksFor(id tabs.TabID, member bool) *render.Marks {
	f := &win.find
	w := &f.work
	if !f.open || f.tab != id || f.member != member || !w.set || w.tab != id || w.member != member || w.text == "" {
		return nil
	}
	w.marksGiven++
	return &w.marks
}

// findCount is the bar's count: the current match (1-based, 0 none), the
// matches kept, whether more were found than kept, and "No results".
func (win *window) findCount() (cur, total int, capped, none bool) {
	w := &win.find.work
	if !win.find.open || !w.set || w.text == "" {
		return 0, 0, false, false
	}
	total = len(w.search.Matches())
	return w.cur + 1, total, w.search.Capped(), w.settled && total == 0
}

// findTrack is the scroll marks' column beside grid: 6 dp in the grid's right
// margin, inside pane (Ruling 14).
func findTrack(gtx layout.Context, grid, pane image.Rectangle) image.Rectangle {
	mid := grid.Max.X + gtx.Dp(8)/2
	return image.Rect(mid-gtx.Dp(3), grid.Min.Y, mid+gtx.Dp(3), grid.Max.Y).Intersect(pane)
}
