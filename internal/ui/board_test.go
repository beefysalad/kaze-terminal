package ui

import (
	"image"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

var boardRect = image.Rect(260, 0, 1200, 760)

func TestAgentsBoardDefaultKeys(t *testing.T) {
	mac := defaultBindings("darwin")["agents_board"]
	win := defaultBindings("windows")["agents_board"]
	if mac != (input.Shortcut{Name: "A", Mods: key.ModCommand | key.ModShift}) || win != (input.Shortcut{Name: "A", Mods: key.ModCtrl | key.ModShift}) {
		t.Fatalf("mac %+v windows %+v", mac, win)
	}
	if actionByName["agents_board"] != actAgentsBoard {
		t.Fatal("agents_board not bound to the board action")
	}
}

func TestPaletteListsShowAgents(t *testing.T) {
	if !slices.ContainsFunc(kazeCommands, func(c kazeCommand) bool {
		return c.Config == "agents_board" && c.Label == "Show agents" && c.Act == actAgentsBoard
	}) {
		t.Fatal("no Show agents command")
	}
}

// boardWindow has tab 2 with a running, a waiting and a done subagent.
func boardWindow(t *testing.T) (*window, tabs.TabID) {
	t.Helper()
	win, _, _ := liveWindow(t, 2)
	id := win.mgr.Tabs()[1].ID
	for _, m := range []notify.Msg{
		{Kind: notify.KindAgentStart, AgentID: "run", AgentType: "Explore"},
		{Kind: notify.KindAgentStart, AgentID: "wait", AgentType: "Plan"},
		{Kind: notify.KindNeeds, AgentID: "wait"},
		{Kind: notify.KindAgentStart, AgentID: "done", AgentType: "general-purpose"},
		{Kind: notify.KindAgentStop, AgentID: "done", Report: "All good\nmore"},
	} {
		m.Tab, m.Session = int(id), "s"
		win.onNotify(m)
	}
	win.drainSubs(t0)
	return win, id
}
func TestBoardEscAndKeyClose(t *testing.T) {
	win, id := boardWindow(t)
	start := win.mgr.Active().ID
	win.runShortcut(pageContext(800, 600), actAgentsBoard)
	if !win.board.open || win.shownTab() != nil {
		t.Fatal("key did not open the board over the session")
	}
	win.runShortcut(pageContext(800, 600), actAgentsBoard)
	if win.board.open || win.mgr.Active().ID != start {
		t.Fatal("key did not close it back to the session")
	}
	win.openBoard(0)
	win.boardKey(key.Event{Name: key.NameEscape, State: key.Press})
	if win.board.open {
		t.Fatal("Esc did not close")
	}
	win.openBoard(id)
	win.drawBoard(pageContext(1200, 760), boardRect)
	r := win.board.back
	if r.Empty() {
		t.Fatal("no back button")
	}
	win.boardPress(pageContext(1200, 760), r.Min.Add(r.Size().Div(2)), 0)
	if win.board.open || win.mgr.Active().ID != start {
		t.Fatal("back did not return to the session it came from")
	}
}

func TestBoardExclusiveWithSettings(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	win.openSettings()
	if win.board.open {
		t.Fatal("settings left the board open")
	}
	win.openBoard(0)
	if win.set.shown {
		t.Fatal("the board left settings in front")
	}
	win.selectTab(win.mgr.Tabs()[0].ID)
	if win.board.open {
		t.Fatal("picking a session left the board open")
	}
}

func TestBoardHooksMissingHint(t *testing.T) {
	f := newClaudeFixture(t, "", false)
	win := claudeWindow(t, f, 800, 600)
	win.closeSettings()
	// Take the tool hook out.
	data := strings.Replace(readFile(t, f.settings), "notify tool", "notify nothing", 1)
	if err := os.WriteFile(f.settings, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	win.openBoard(0)
	win.drawBoard(pageContext(1200, 760), boardRect)
	if !slices.Contains(win.board.texts, "kaze needs three more Claude Code hooks to see subagents.") || win.board.setup.Empty() {
		t.Fatalf("no hint in %q", win.board.texts)
	}
	r := win.board.setup
	win.boardPress(pageContext(1200, 760), r.Min.Add(r.Size().Div(2)), 0)
	if !win.set.shown || win.sections()[win.set.section].title != "Agents" {
		t.Fatal("the hint's button did not open Settings, Agents")
	}
}

func TestStatusAgentsText(t *testing.T) {
	if agentsRunning(0) != "" || agentsRunning(1) != "1 agent running" || agentsRunning(4) != "4 agents running" {
		t.Fatal("status text")
	}
	win, _ := boardWindow(t)
	if win.statusModel(time.Now()).Agents != 2 {
		t.Fatalf("status count %d", win.statusModel(time.Now()).Agents)
	}
}

func TestCloseTabKeyClosesTheBoardNotTheSession(t *testing.T) {
	win, _ := boardWindow(t)
	n := len(win.mgr.Tabs())
	win.openBoard(0)
	win.runShortcut(pageContext(800, 600), actCloseTab)
	if win.board.open || len(win.mgr.Tabs()) != n {
		t.Fatalf("board open %v, tabs %d -> %d", win.board.open, n, len(win.mgr.Tabs()))
	}
}

func TestAgentLineUnderActiveRowClosesTheBoard(t *testing.T) {
	win, id := boardWindow(t)
	win.mgr.SetActive(id)
	win.openBoard(0)
	win.showAgent(id, subagents.Key{Session: "s", ID: "run"}, false)
	if win.board.open {
		t.Fatal("the panel opened behind the board")
	}
}

func TestNewTabClosesTheBoard(t *testing.T) {
	win, _ := boardWindow(t)
	win.openBoard(0)
	win.newTab()
	if win.board.open {
		t.Fatal("new tab opened behind the board")
	}
}

func TestClaudeLeavingEndsItsSubagents(t *testing.T) {
	win, id := boardWindow(t)
	tab := win.mgr.Lookup(id)
	deliverNow(win, promptMsg(int(id), "", ""), t0) // Claude is running in the tab
	promptMark(t, tab)                              // the shell prompt is back: Claude exited
	win.leaveAgents(win.mgr.Tabs())
	if win.subs.Active(0) != 0 {
		t.Fatalf("%d subagents still running after Claude left", win.subs.Active(0))
	}
	if _, ok := win.subTimer(t0); ok {
		t.Fatal("the minute timer outlived Claude")
	}
}

func TestBoardNamesSessionsLikeTheSidebar(t *testing.T) {
	win, id := boardWindow(t)
	deliverNow(win, promptMsg(int(id), "/ship-ticket", "TODO-142 login"), t0)
	title, _ := win.sessionName(id)
	if want := win.sideIn().label(win.mgr.Lookup(id), win.locOf(win.mgr.Lookup(id)), win.agentOf(id)); title != want || title != "TODO-142" {
		t.Fatalf("board says %q, sidebar says %q", title, want)
	}
}
func TestOpenBoardResetsViewAndSelectsWaiting(t *testing.T) {
	win, id := boardWindow(t)
	b := &win.board
	b.filter = boardDone
	b.ed.SetText("x")
	b.searching = true
	win.openBoard(id)
	if b.tab != id || b.filter != boardAll || b.ed.Text() != "" || b.searching {
		t.Fatalf("open did not reset: tab %v filter %v text %q searching %v", b.tab, b.filter, b.ed.Text(), b.searching)
	}
	if want := (subagents.Key{Session: "s", ID: "wait"}); b.sel != want || b.prev.agent != b.sel {
		t.Fatalf("sel %+v prev %+v, want %+v", b.sel, b.prev.agent, want)
	}
	win.closeBoard()
	win.openBoard(0)
	if b.tab != 0 {
		t.Fatalf("tab %v after openBoard(0)", b.tab)
	}
}

func TestOpenBoardWithoutWaitingSelectsNothing(t *testing.T) {
	win, _, _ := liveWindow(t, 2)
	startAgent(win, win.mgr.Tabs()[1].ID, "s", "run")
	win.drainSubs(t0)
	win.openBoard(0)
	if win.board.sel != (subagents.Key{}) {
		t.Fatalf("sel %+v with no waiting agent", win.board.sel)
	}
}

func TestClearBoardSession(t *testing.T) {
	win, id := boardWindow(t)
	win.openBoard(id)
	win.clearBoardSession()
	if !win.board.open || win.board.tab != 0 {
		t.Fatalf("open %v tab %v", win.board.open, win.board.tab)
	}
}
