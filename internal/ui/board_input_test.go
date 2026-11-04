package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// boardNarrow is a board area too narrow for the preview.
var boardNarrow = image.Rect(260, 0, 1000, 600)

func drawNarrow(win *window) layout.Context {
	gtx := pageContext(1000, 600)
	gtx.Now = time.Now()
	win.drawBoard(gtx, boardNarrow)
	return gtx
}

func boardClickAt(win *window, gtx layout.Context, r image.Rectangle, at time.Duration) {
	win.boardPress(gtx, centre(r), at)
}

func keyPress(name key.Name) key.Event { return key.Event{Name: name, State: key.Press} }

func TestBoardClickSelects(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	boardClickAt(win, gtx, mustRow(t, win, "run").rect, 0)
	if win.board.sel.ID != "run" || !win.board.open || win.dockOf(id) != nil {
		t.Fatalf("sel %+v open %v dock %v", win.board.sel, win.board.open, win.dockOf(id))
	}
	if win.board.prev.agent != win.board.sel {
		t.Fatal("the preview does not follow the selection")
	}
}

func TestBoardDoubleClickOpensInSession(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	run, wait := mustRow(t, win, "run").rect, mustRow(t, win, "wait").rect
	boardClickAt(win, gtx, wait, 0)
	boardClickAt(win, gtx, run, 100*time.Millisecond)
	if !win.board.open {
		t.Fatal("presses on two rows opened one")
	}
	boardClickAt(win, gtx, run, 2*doubleClick)
	boardClickAt(win, gtx, run, 2*doubleClick+100*time.Millisecond)
	if win.board.open {
		t.Fatal("a double-click left the board open")
	}
	if s := win.dockOf(id); s == nil || s.agent.ID != "run" {
		t.Fatalf("panel %+v", s)
	}
}

func TestBoardOpenInSessionAndEnter(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	boardClickAt(win, gtx, win.board.openIn, 0)
	if win.board.open || win.dockOf(id) == nil || win.dockOf(id).agent.ID != "wait" {
		t.Fatal("Open in session did not open the agent")
	}
	win.closeDock(id)
	win.openBoard(0)
	drawWide(win)
	win.boardKey(keyPress(key.NameDownArrow))
	win.boardKey(keyPress(key.NameReturn))
	if win.board.open || win.dockOf(id) == nil || win.dockOf(id).agent.ID != "run" {
		t.Fatal("Enter did not open the selected agent")
	}
}

func TestBoardArrowsStayInVisibleRows(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	b := &win.board
	steps := []struct {
		k    key.Name
		want string
	}{{key.NameDownArrow, "run"}, {key.NameDownArrow, "run"}, {key.NameUpArrow, "wait"}, {key.NameUpArrow, "wait"}}
	for i, s := range steps {
		win.boardKey(keyPress(s.k))
		if b.sel.ID != s.want {
			t.Fatalf("step %d: sel %q, want %q", i, b.sel.ID, s.want)
		}
	}
	b.sel = subagents.Key{}
	win.boardMove(1)
	if b.sel.ID != "wait" {
		t.Fatalf("Down from nothing picked %q", b.sel.ID)
	}
	b.ed.SetText("zzz")
	win.boardMove(1)
	win.boardMove(-1)
	if b.sel.ID != "wait" {
		t.Fatalf("arrows on an empty list moved to %q", b.sel.ID)
	}
}

func TestBoardSlashFocusesSearch(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	win.boardKey(keyPress("/"))
	if !win.board.searching || win.focusTarget() != &win.board.ed {
		t.Fatalf("searching %v focus %v", win.board.searching, win.focusTarget())
	}
}

func TestBoardSlashTypesInSearch(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.openBoard(0)
	twoFrames(t, win, r)
	win.boardKey(keyPress("/"))
	twoFrames(t, win, r)
	r.Queue(key.EditEvent{Text: "/"})
	twoFrames(t, win, r)
	if win.board.ed.Text() != "/" || !win.board.searching {
		t.Fatalf("text %q searching %v", win.board.ed.Text(), win.board.searching)
	}
}

func TestBoardEscClearsSearchFirst(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	win.board.ed.SetText("plan")
	win.board.searching = true
	win.boardSearchEsc()
	if win.board.ed.Text() != "" || !win.board.open || win.board.searching {
		t.Fatalf("text %q open %v searching %v", win.board.ed.Text(), win.board.open, win.board.searching)
	}
	win.boardSearchEsc()
	if win.board.open {
		t.Fatal("Esc on an empty search kept the board open")
	}
	win.openBoard(0)
	win.board.ed.SetText("plan")
	win.boardKey(keyPress(key.NameEscape))
	if win.board.ed.Text() != "" || !win.board.open {
		t.Fatal("Esc with the field unfocused did not clear the text first")
	}
	win.boardKey(keyPress(key.NameEscape))
	if win.board.open {
		t.Fatal("second Esc did not close")
	}
}

func TestBoardSearchSubmitKeepsText(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.openBoard(0)
	win.boardKey(keyPress("/"))
	twoFrames(t, win, r)
	r.Queue(key.EditEvent{Text: "ex"})
	twoFrames(t, win, r)
	r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	twoFrames(t, win, r)
	if win.board.ed.Text() != "ex" || win.board.searching || !win.board.open {
		t.Fatalf("text %q searching %v open %v", win.board.ed.Text(), win.board.searching, win.board.open)
	}
}

func TestBoardNarrowClickOpensSession(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	gtx := drawNarrow(win)
	if win.board.wide || win.board.prev.tr != nil || !win.board.preview.Empty() {
		t.Fatalf("wide %v transcript %v", win.board.wide, win.board.prev.tr)
	}
	boardClickAt(win, gtx, mustRow(t, win, "run").rect, 0)
	if win.board.open {
		t.Fatal("a narrow click left the board open")
	}
	if s := win.dockOf(id); s == nil || s.agent.ID != "run" {
		t.Fatal("no panel on run")
	}
}

func TestBoardSelectionGone(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	win.board.showDone = map[tabs.TabID]bool{id: true}
	done := subagents.Key{Session: "s", ID: "done"}
	win.boardSelect(done)
	drawPreview(win)
	win.subs.Dismiss(done)
	drawPreview(win)
	if win.board.sel != (subagents.Key{}) || win.board.prev.agent != (subagents.Key{}) || win.board.prev.tr != nil {
		t.Fatalf("sel %+v prev %+v", win.board.sel, win.board.prev.agent)
	}
	if !boardHas(win, "Pick an agent") {
		t.Fatalf("texts %q", win.board.texts)
	}
}

func TestBoardTokenDropsWithSession(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(id)
	win.closeTab(id)
	drawWide(win)
	if win.board.tab != 0 || !win.board.token.Empty() {
		t.Fatalf("tab %v token %v", win.board.tab, win.board.token)
	}
}

func TestBoardHoverRedrawsOnlyOnChange(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	startAgent(win, a.ID, "s", "x")
	startAgent(win, a.ID, "s", "y")
	twoFrames(t, win, r)
	win.openBoard(0)
	twoFrames(t, win, r)
	x, y := mustRow(t, win, "x").rect, mustRow(t, win, "y").rect
	win.board.redraws = 0
	move := func(p image.Point) {
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at(p)})
		sideFrame(t, win, r)
	}
	move(centre(x))
	move(centre(x).Add(image.Pt(3, 0)))
	if win.board.redraws != 1 || win.board.hover.ID != "x" {
		t.Fatalf("redraws %d hover %+v", win.board.redraws, win.board.hover)
	}
	move(centre(y))
	if win.board.redraws != 2 || win.board.hover.ID != "y" {
		t.Fatalf("redraws %d hover %+v", win.board.redraws, win.board.hover)
	}
}

// A click on the list gives the keyboard back from the search field, so the arrows work.
func TestBoardPressElsewhereLeavesSearch(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	gtx := drawWide(win)
	win.boardKey(keyPress("/"))
	boardClickAt(win, gtx, mustRow(t, win, "run").rect, 0)
	if win.board.searching || win.focusTarget() != &win.board.tag || win.board.sel.ID != "run" {
		t.Fatalf("searching %v sel %+v", win.board.searching, win.board.sel)
	}
}

// A "+N more" chip clicked while the board's search has the keyboard gives the
// keyboard back to the board: the field still reports focus in that frame, but
// only a newly focused field counts as a click into it.
func TestBoardChipLeavesSearch(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	startAgents(win, a.ID, 12)
	twoFrames(t, win, r)
	win.openBoard(a.ID)
	twoFrames(t, win, r)
	win.boardKey(keyPress("/"))
	twoFrames(t, win, r)
	if !win.board.searching || !r.Source().Focused(&win.board.ed) {
		t.Fatalf("fixture: searching %v", win.board.searching)
	}
	chips := trayHits(win, a.ID, hitAgentChip)
	if len(chips) != 1 {
		t.Fatalf("chips %v", chips)
	}
	sideClickAt(win, r, centre(chips[0].rect), pointer.ButtonPrimary, 0)
	twoFrames(t, win, r)
	if win.board.searching || win.focusTarget() != &win.board.tag || r.Source().Focused(&win.board.ed) {
		t.Fatalf("after the chip: searching %v, field focused %v", win.board.searching, r.Source().Focused(&win.board.ed))
	}
	// A click straight into the field still gives it the keyboard.
	win.boardKey(keyPress("/"))
	twoFrames(t, win, r)
	if !win.board.searching || !r.Source().Focused(&win.board.ed) {
		t.Fatalf("/ after the chip: searching %v", win.board.searching)
	}
}

// Enter opens the previewed agent even when the search hides its row, as
// "Open in session" does.
func TestBoardEnterOpensHiddenPreview(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(0)
	drawWide(win)
	win.boardSelect(subagents.Key{Session: "s", ID: "run"})
	win.board.ed.SetText("zzz")
	drawWide(win)
	win.boardKey(keyPress(key.NameReturn))
	if win.board.open || win.dockOf(id) == nil || win.dockOf(id).agent.ID != "run" {
		t.Fatal("Enter did not open the previewed agent hidden by the search")
	}
}
