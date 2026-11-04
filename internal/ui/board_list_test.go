package ui

import (
	"image"
	"slices"
	"testing"

	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// boardWide is a board area with room for the preview beside the list.
var boardWide = image.Rect(260, 0, 1400, 860)

// drawWide draws the board in boardWide and returns its context.
func drawWide(win *window) layout.Context {
	gtx := pageContext(1400, 860)
	win.drawBoard(gtx, boardWide)
	return gtx
}

// boardRowOf is the drawn row hit for agent id; ok is false when none is drawn.
func boardRowOf(win *window, id string) (boardRowHit, bool) {
	i := slices.IndexFunc(win.board.rows, func(h boardRowHit) bool { return !h.row.session && h.row.key.ID == id })
	if i < 0 {
		return boardRowHit{}, false
	}
	return win.board.rows[i], true
}

func mustRow(t *testing.T, win *window, id string) boardRowHit {
	t.Helper()
	h, ok := boardRowOf(win, id)
	if !ok {
		t.Fatalf("no row for %q", id)
	}
	return h
}

func boardHas(win *window, s string) bool { return slices.Contains(win.board.texts, s) }

func TestBoardDrawsHeaderAndSegments(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	drawWide(win)
	back, _ := win.sessionName(win.boardBack())
	for _, s := range []string{"Agents", boardSummary(3, 1), "Back to " + back, "All", "Needs you", "Running", "Done"} {
		if !boardHas(win, s) {
			t.Fatalf("no %q in %q", s, win.board.texts)
		}
	}
	b := &win.board
	for i, s := range b.seg {
		if s.Empty() || i > 0 && s.Min.X < b.seg[i-1].Max.X {
			t.Fatalf("segments %v", b.seg)
		}
	}
	if b.search.Empty() || b.search.Min.X <= b.seg[3].Max.X {
		t.Fatalf("search %v after segments %v", b.search, b.seg)
	}
}

func TestBoardNeedsCardFirst(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	drawWide(win)
	b := &win.board
	if len(b.rows) == 0 || b.rows[0].row.key.ID != "wait" || len(b.cardHits) == 0 || b.rows[0].rect.Max.Y > b.cardHits[0].rect.Min.Y {
		t.Fatalf("rows %+v cards %+v", b.rows, b.cardHits)
	}
	if !boardHas(win, "NEEDS YOU") {
		t.Fatal("no NEEDS YOU head")
	}
}

func TestBoardCardHeaderAndMeter(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	title, _ := win.sessionName(id)
	if !boardHas(win, title) || !boardHas(win, "Go to session ›") {
		t.Fatalf("texts %q", win.board.texts)
	}
	c := win.board.cardHits[0]
	if c.meter.Dy() != gtx.Dp(3) || c.goTo.Empty() {
		t.Fatalf("card %+v", c)
	}
}

func TestBoardDoneFold(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	if !boardHas(win, "✓ 1 finished ›") {
		t.Fatalf("texts %q", win.board.texts)
	}
	if _, ok := boardRowOf(win, "done"); ok {
		t.Fatal("a folded row was drawn")
	}
	win.boardPress(gtx, centre(win.board.cardHits[0].fold), 0)
	gtx = drawWide(win)
	mustRow(t, win, "done")
	if !boardHas(win, "Hide finished") {
		t.Fatalf("texts %q", win.board.texts)
	}
	win.boardPress(gtx, centre(win.board.cardHits[0].fold), 0)
	drawWide(win)
	if _, ok := boardRowOf(win, "done"); ok || !boardHas(win, "✓ 1 finished ›") {
		t.Fatal("did not fold back")
	}
}

func TestBoardSegmentPress(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	win.boardPress(gtx, centre(win.board.seg[boardDone]), 0)
	if win.board.filter != boardDone {
		t.Fatalf("filter %v", win.board.filter)
	}
	drawWide(win)
	if len(win.board.rows) != 1 || win.board.rows[0].row.key.ID != "done" {
		t.Fatalf("rows %+v", win.board.rows)
	}
}

func TestBoardTokenClears(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(id)
	gtx := drawWide(win)
	title, _ := win.sessionName(id)
	if !boardHas(win, title+" ×") || win.board.token.Empty() {
		t.Fatalf("texts %q", win.board.texts)
	}
	win.boardPress(gtx, centre(win.board.token), 0)
	if win.board.tab != 0 || !win.board.open {
		t.Fatalf("tab %v open %v", win.board.tab, win.board.open)
	}
}

func TestBoardSearchPressFocuses(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	win.wantFocus = false
	win.boardPress(gtx, centre(win.board.search), 0)
	if !win.board.searching || !win.wantFocus {
		t.Fatal("a press on the search did not give it the keyboard")
	}
}

func TestBoardGoToSession(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	win.boardPress(gtx, centre(win.board.cardHits[0].goTo), 0)
	if win.board.open || win.mgr.Active().ID != id || win.dockOf(id) != nil {
		t.Fatalf("open %v active %v dock %v", win.board.open, win.mgr.Active().ID, win.dockOf(id))
	}
}

// A done row's × shows on hover or selection, removes it and keeps the board open.
func TestBoardDismissDoneCard(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	win.board.showDone = map[tabs.TabID]bool{id: true}
	win.board.hover = subagents.Key{Session: "s", ID: "done"}
	gtx := drawWide(win)
	if !mustRow(t, win, "run").close.Empty() {
		t.Fatal("a running row has a close button")
	}
	x := mustRow(t, win, "done").close
	if x.Empty() {
		t.Fatal("the hovered done row has no close button")
	}
	win.boardPress(gtx, centre(x), 0)
	if !win.board.open {
		t.Fatal("closing a row closed the board")
	}
	if win.subs.Get(subagents.Key{Session: "s", ID: "done"}) != nil {
		t.Fatal("row not removed")
	}
	if win.dockOf(id) != nil {
		t.Fatal("dismissing opened a panel")
	}
}

// Clear in a card's head empties its finished agents; running and waiting stay.
func TestBoardClearDone(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	c := win.board.cardHits[0].clear
	if c.Empty() {
		t.Fatal("no Clear button")
	}
	win.boardPress(gtx, centre(c), 0)
	v := win.boardView()
	if v.counts[boardDone] != 0 || v.counts[boardRunning] != 1 || v.counts[boardNeeds] != 1 {
		t.Fatalf("counts %v", v.counts)
	}
	drawWide(win)
	if !win.board.cardHits[0].clear.Empty() {
		t.Fatal("Clear shown with nothing done")
	}
}

func TestBoardEmptyStates(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	win.board.ed.SetText("zzz")
	drawWide(win)
	if !boardHas(win, "No agents match") || !boardHas(win, "Try another filter, or clear the search.") {
		t.Fatalf("texts %q", win.board.texts)
	}
	empty, _, _ := liveWindow(t, 1)
	empty.openBoard(0)
	drawWide(empty)
	if !boardHas(empty, "No subagents yet") {
		t.Fatalf("texts %q", empty.board.texts)
	}
}

func TestBoardCardOpensPane(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawNarrow(win) // narrow: a click opens the agent in its session
	win.boardPress(gtx, centre(mustRow(t, win, "run").rect), 0)
	if win.board.open || win.mgr.Active().ID != id {
		t.Fatal("board still open or wrong session")
	}
	if s := win.dockOf(id); s == nil || s.agent != (subagents.Key{Session: "s", ID: "run"}) {
		t.Fatal("no agent panel")
	}
}

func TestBoardCardOpensDock(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.openBoard(a.ID)
	twoFrames(t, win, r)
	h := mustRow(t, win, "x")
	clickAt(t, win, r, centre(h.rect))
	if s := win.dockOf(a.ID); s == nil || s.agent != k {
		t.Fatal("board row did not open the panel")
	}
}
