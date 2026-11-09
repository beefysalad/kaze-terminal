package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/find"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// findFor opens the bar on the pane with the keyboard, types text and runs
// frames until the search settles (at most 20).
func findFor(t *testing.T, win *window, r *input.Router, text string) {
	t.Helper()
	findKey(t, win, r)
	setFind(t, win, r, text)
}

// setFind replaces the field's text and runs frames until the search settles.
func setFind(t *testing.T, win *window, r *input.Router, text string) {
	t.Helper()
	r.Queue(key.EditEvent{Range: key.Range{Start: 0, End: win.find.ed.Len()}, Text: text})
	settleFind(t, win, r)
}

func settleFind(t *testing.T, win *window, r *input.Router) {
	t.Helper()
	for range 20 {
		sideFrame(t, win, r)
		if w := &win.find.work; w.settled && w.text == win.find.text && !w.pick && !w.reveal {
			return
		}
	}
	t.Fatal("the search did not settle in 20 frames")
}

// lines prints each line to pty i, CR LF after each, and waits until tab
// shows the last one.
func lines(t *testing.T, st *recStarter, i int, tab *tabs.Tab, ls ...string) {
	t.Helper()
	before := tab.Session.OutputSeq()
	last := ls[len(ls)-1]
	last = last[max(len(last)-10, 0):] // a long line wraps: its end is on one row
	emitTo(t, st, i, tab, strings.Join(ls, "\r\n")+"\r\n", func() bool {
		return tab.Session.OutputSeq() != before && strings.Contains(screenText(tab), last)
	})
}

// screenText is the live grid's text, rows joined by newlines.
func screenText(tab *tabs.Tab) string {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	scr := tab.Session.Screen()
	cols, rows := scr.Size()
	var b strings.Builder
	for y := range rows {
		for x := range cols {
			if c := scr.Cell(x, y); c.Width != 0 {
				b.WriteRune(max(c.Rune, ' '))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func filler(n int, prefix string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s %d", prefix, i)
	}
	return out
}

// absText reads n cells' runes from stable position p of tab.
func absText(tab *tabs.Tab, p find.Pos, n int) string {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	scr := tab.Session.Screen()
	abs := p.Line - int(scr.Evicted())
	if abs < 0 {
		return ""
	}
	var b strings.Builder
	for c := p.Col; c < p.Col+n; c++ {
		if sb := scr.ScrollbackLen(); abs < sb {
			if l := scr.ScrollbackLine(abs); c < len(l) {
				b.WriteRune(max(l[c].Rune, ' '))
			}
		} else {
			b.WriteRune(max(scr.Cell(c, abs-sb).Rune, ' '))
		}
	}
	return b.String()
}

// viewLines is the stable lines tab's view shows, top and bottom.
func viewLines(tab *tabs.Tab) (int, int) {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	scr := tab.Session.Screen()
	_, rows := scr.Size()
	top := int(scr.Evicted()) + scr.ScrollbackLen() - scr.ViewOffset()
	return top, top + rows - 1
}

// curShown reports whether the current match's first line is in tab's view.
func curShown(win *window, tab *tabs.Tab) bool {
	w := &win.find.work
	top, bottom := viewLines(tab)
	return w.cur >= 0 && w.curPos.Line >= top && w.curPos.Line <= bottom
}

func TestFindCountsMatches(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	ls := filler(40, "ok")
	for _, i := range []int{3, 9, 17, 25, 33} {
		ls[i] = "warn " + ls[i]
	}
	lines(t, st, 0, a, ls...)
	findFor(t, win, r, "warn")
	if cur, total, capped, none := win.findCount(); cur < 1 || total != 5 || capped || none {
		t.Fatalf("count (%d, %d, %v, %v), want (n, 5, false, false)", cur, total, capped, none)
	}
	setFind(t, win, r, "zzz")
	if cur, total, _, none := win.findCount(); cur != 0 || total != 0 || !none {
		t.Fatalf("zzz: count (%d, %d, none %v), want no results", cur, total, none)
	}
}

func TestFindSearchesHistory(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "a needle here")
	lines(t, st, 0, a, filler(300, "line")...)
	findFor(t, win, r, "needle")
	if _, total, _, _ := win.findCount(); total != 1 {
		t.Fatalf("%d matches, want 1", total)
	}
	if historyOffset(a) == 0 || !curShown(win, a) {
		t.Fatalf("offset %d: the view does not show the match", historyOffset(a))
	}
}

func TestFindPicksMatchInView(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn top")
	lines(t, st, 0, a, filler(300, "line")...)
	lines(t, st, 0, a, "warn here")
	findFor(t, win, r, "warn")
	cur, total, _, _ := win.findCount()
	if total != 2 || cur != 2 || historyOffset(a) != 0 {
		t.Fatalf("cur %d of %d, offset %d; want the screen's match, no scroll", cur, total, historyOffset(a))
	}
	if got := absText(a, win.find.work.curPos, 9); got != "warn here" {
		t.Fatalf("current reads %q", got)
	}
}

func TestFindStepWrapsAndScrolls(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn A")
	lines(t, st, 0, a, filler(300, "line")...)
	lines(t, st, 0, a, "warn B")
	findFor(t, win, r, "warn")
	step := func(name key.Name, mods key.Modifiers, want string) {
		t.Helper()
		pressKey(t, win, r, name, mods)
		settleFind(t, win, r)
		if got := absText(a, win.find.work.curPos, 6); got != want || !curShown(win, a) {
			t.Fatalf("after %s: current %q shown %v, want %q in view", name, got, curShown(win, a), want)
		}
	}
	step(key.NameReturn, 0, "warn A") // past the last: the first
	if historyOffset(a) == 0 {
		t.Fatal("stepping to a history match did not scroll")
	}
	step(key.NameReturn, 0, "warn B")
	step(key.NameReturn, key.ModShift, "warn A")
	step(key.NameUpArrow, 0, "warn B") // before the first: the last
	step(key.NameDownArrow, 0, "warn A")
	step(key.NameEnter, 0, "warn B")
}

func TestFindOutputDoesNotScroll(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "warn old")
	lines(t, st, 0, a, filler(300, "line")...)
	findFor(t, win, r, "warn")
	off, sb, pos := historyOffset(a), scrollbackLen(a), win.find.work.curPos
	if off == 0 {
		t.Fatal("not scrolled back to the match")
	}
	lines(t, st, 0, a, filler(50, "warn new")...)
	settleFind(t, win, r)
	if got, grew := historyOffset(a), scrollbackLen(a)-sb; got != off+grew {
		t.Fatalf("offset %d, want %d (%d plus the %d lines that scrolled in)", got, off+grew, off, grew)
	}
	if _, total, _, _ := win.findCount(); total != 51 {
		t.Fatalf("%d matches, want 51", total)
	}
	if win.find.work.curPos != pos || absText(a, pos, 8) != "warn old" {
		t.Fatalf("current moved to %+v (%q)", win.find.work.curPos, absText(a, win.find.work.curPos, 8))
	}
}

func TestFindCurrentStaysWhileOutputStreams(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	win.mgr.SetScrollback(10)
	c, err := win.mgr.New()
	if err != nil {
		t.Fatal(err)
	}
	win.mgr.SetActive(c.ID)
	twoFrames(t, win, r)
	lines(t, st, 4, c, "warn 1", "warn 2", "warn 3")
	findFor(t, win, r, "warn")
	if got := absText(c, win.find.work.curPos, 6); got != "warn 1" {
		t.Fatalf("current %q, want warn 1 (the first in view)", got)
	}
	lines(t, st, 4, c, filler(20, "x")...)
	settleFind(t, win, r)
	if got := absText(c, win.find.work.curPos, 6); got != "warn 1" || historyOffset(c) != 0 {
		t.Fatalf("current %q offset %d, want warn 1 at the bottom", got, historyOffset(c))
	}
	lines(t, st, 4, c, filler(60, "y")...)
	lines(t, st, 4, c, "warn 9")
	settleFind(t, win, r)
	if got := absText(c, win.find.work.curPos, 6); got != "warn 9" || historyOffset(c) != 0 {
		t.Fatalf("after eviction: current %q offset %d, want warn 9 at the bottom", got, historyOffset(c))
	}
	if _, total, _, _ := win.findCount(); total != 1 {
		t.Fatalf("%d matches after eviction, want 1", total)
	}
}

func TestFindMatchesAcrossWrap(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	a.Session.Lock()
	cols, _ := a.Session.Screen().Size()
	a.Session.Unlock()
	line := strings.Repeat("a", cols-2) + "WRAP" + strings.Repeat("b", 300-cols-2)
	lines(t, st, 0, a, line)
	findFor(t, win, r, "wrap")
	if _, total, _, _ := win.findCount(); total != 1 || len(win.find.work.marks.Spans) != 2 {
		t.Fatalf("%d matches, %d spans; want 1 match over 2 rows", total, len(win.find.work.marks.Spans))
	}
}

func TestFindInMemberTab(t *testing.T) {
	win, st, r, a, _, m1 := dockOne(t)
	lines(t, st, 2, m1, "warn one")
	findFor(t, win, r, "warn")
	if !win.find.member {
		t.Fatal("not on the member")
	}
	lines(t, st, 0, a, "warn in the session")
	settleFind(t, win, r)
	if _, total, _, _ := win.findCount(); total != 1 {
		t.Fatalf("%d matches after the session's output, want 1", total)
	}
	lines(t, st, 2, m1, "warn two")
	settleFind(t, win, r)
	if _, total, _, _ := win.findCount(); total != 2 {
		t.Fatalf("%d matches after the member's output, want 2", total)
	}
	if win.findMarksFor(m1.ID, true) == nil || win.findMarksFor(a.ID, false) != nil {
		t.Fatal("marks go to the wrong pane")
	}
}

func TestFindEscRestoresViewEndToEnd(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, "a needle here")
	lines(t, st, 0, a, filler(300, "line")...)
	findFor(t, win, r, "needle")
	if historyOffset(a) == 0 {
		t.Fatal("the view did not scroll to the match")
	}
	pressKey(t, win, r, key.NameEscape, 0)
	if win.find.open || historyOffset(a) != 0 {
		t.Fatalf("open %v offset %d after Esc, want closed at the bottom", win.find.open, historyOffset(a))
	}
}

func TestFindClosedScansNothing(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	for i := range 100 {
		lines(t, st, 0, a, fmt.Sprintf("warn %d", i))
		sideFrame(t, win, r)
	}
	if n := win.find.work.search.Scanned(); n != 0 || win.find.work.marksGiven != 0 || win.find.work.locks != 0 {
		t.Fatalf("closed bar: scanned %d, marks given %d, locks %d", n, win.find.work.marksGiven, win.find.work.locks)
	}
}

func TestFindStillFrameAllocatesNothing(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, filler(50, "warn")...)
	findFor(t, win, r, "warn")
	sideFrame(t, win, r)
	gtx := pageContext(1000, 600)
	locks := win.find.work.locks
	if n := testing.AllocsPerRun(20, func() { win.findTick(gtx) }); n != 0 {
		t.Fatalf("findTick on a still frame allocates %v", n)
	}
	if win.find.work.locks != locks {
		t.Fatalf("findTick on a still frame locked the session %d times", win.find.work.locks-locks)
	}
}

func TestFindTypingCoalesces(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, filler(50, "warn")...)
	findKey(t, win, r)
	sideFrame(t, win, r)
	q := win.find.work.queries
	for i, s := range []string{"w", "a", "r", "n"} {
		r.Queue(key.EditEvent{Range: key.Range{Start: i, End: i}, Text: s})
	}
	settleFind(t, win, r)
	if got := win.find.work.queries - q; got != 1 || win.find.text != "warn" {
		t.Fatalf("%d queries for one frame of typing (text %q), want 1", got, win.find.text)
	}
}

func TestFindLargeHistoryFinishesOverFrames(t *testing.T) {
	win, st, r, _, _, _, _ := memberWindow(t)
	win.mgr.SetScrollback(60_000)
	c, err := win.mgr.New()
	if err != nil {
		t.Fatal(err)
	}
	win.mgr.SetActive(c.ID)
	twoFrames(t, win, r)
	c.Session.Lock()
	cols, _ := c.Session.Screen().Size()
	c.Session.Unlock()
	var b strings.Builder
	pad := strings.Repeat("x", cols-12)
	for i := range 60_000 {
		if i%100 == 0 {
			fmt.Fprintf(&b, "%s warn %05d\r\n", pad[:cols-12], i)
		} else {
			fmt.Fprintf(&b, "%s line %05d\r\n", pad[:cols-12], i)
		}
	}
	// Wait for the final line so the search cannot race the fixture's remaining
	// output. Parsing 60,000 lines under -race is setup, not a latency check.
	emitToWithin(t, st, 4, c, b.String(), func() bool {
		return strings.Contains(screenText(c), "line 59999")
	}, 15*time.Second)
	findKey(t, win, r)
	r.Queue(key.EditEvent{Text: "warn"})
	w := &win.find.work
	frames := 0
	for frames < 20 {
		before := w.search.Scanned()
		sideFrame(t, win, r)
		frames++
		if grew := w.search.Scanned() - before; grew > find.StepBudget+cols {
			t.Fatalf("one frame scanned %d cells, over the budget", grew)
		}
		if w.settled && w.text == "warn" {
			break
		}
	}
	if !w.settled || frames < 2 {
		t.Fatalf("settled %v after %d frames, want over several frames", w.settled, frames)
	}
	if _, total, _, _ := win.findCount(); total != 600 {
		t.Fatalf("%d matches, want 600", total)
	}
}

func TestSelectionStillWorksWithFindOpen(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	var clip []string
	win.toClipboard = func(s string) { clip = append(clip, s) }
	emitTo(t, st, 0, a, "hello", func() bool { return textShown(a, 4, 'o')() })
	findFor(t, win, r, "hello")
	cw, ch := win.rend.CellSize()
	from := win.gridRect.Min.Add(image.Pt(cw/4, ch/2))
	dragFrom(t, win, r, from, image.Pt(4*cw+cw/2, 0))
	if win.find.keys || !win.find.open {
		t.Fatalf("keys %v open %v after the drag", win.find.keys, win.find.open)
	}
	pressKey(t, win, r, "C", key.ModCtrl|key.ModShift)
	if fmt.Sprint(clip) != "[hello]" {
		t.Fatalf("copied %q, want [hello]", clip)
	}
}

func TestFindPanelResizeSearchesAgain(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	lines(t, st, 0, a, filler(50, "warn")...)
	findFor(t, win, r, "warn")
	w := &win.find.work
	before := w.outputs
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	for range 6 {
		sideFrame(t, win, r)
	}
	if w.outputs == before {
		t.Fatal("opening the agent panel resized the session but the search did not run again")
	}
}
