package ui

import (
	"testing"
	"time"

	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func startAgent(win *window, tab tabs.TabID, session, id string) subagents.Key {
	win.liveMgr.Store(win.mgr)
	win.onNotify(notify.Msg{Tab: int(tab), Kind: notify.KindAgentStart, Session: session, AgentID: id, AgentType: "Explore"})
	return subagents.Key{Session: session, ID: id}
}

// agentRowFor is the sidebar's agent row for session id after a frame.
func agentRowFor(t *testing.T, win *window, id tabs.TabID) sideRowHit {
	t.Helper()
	return rowHit(t, win, id)
}

// markAgent makes session id run an agent, so the Agents section lists it.
func markAgent(win *window, id tabs.TabID) { setAgent(win, id, agentState{claude: true}) }

// setAgent gives session id agent state a, stamped with the shell's current prompt
// count so it reads as live, not as an agent that has left.
func setAgent(win *window, id tabs.TabID, a agentState) {
	if win.agents == nil {
		win.agents = map[tabs.TabID]agentState{}
	}
	if t := win.mgr.Lookup(id); t != nil {
		a.shellAt = shellSeq(t)
	}
	win.agents[id] = a
}

func TestSidebarAgentRowOfOtherSessionKeepsItsPanel(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	markAgent(win, a.ID)
	markAgent(win, b.ID)
	x := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.showAgent(a.ID, x, false)
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	sideClick(win, r, agentRowFor(t, win, a.ID).rect, pointer.ButtonPrimary)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != a.ID {
		t.Fatalf("the agent row showed %d, want session a (%d)", win.mgr.Active().ID, a.ID)
	}
	if s := win.dockOf(a.ID); s == nil || s.agent != x || win.dockRect().Empty() {
		t.Fatal("session a forgot its panel after its agent row was clicked")
	}
}

func TestSidebarMemberAgentShowsMember(t *testing.T) {
	win, _, r, _, _, m1, _ := memberWindow(t)
	k := startAgent(win, m1.ID, "m", "x")
	twoFrames(t, win, r)
	win.showAgent(m1.ID, k, true)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != m1.ID || win.dockOwner() != m1.ID || win.dockOf(m1.ID) == nil || win.dockRect().Empty() {
		t.Fatalf("member not shown with its panel: active %d owner %d", win.mgr.Active().ID, win.dockOwner())
	}
}

func TestToastGoOpensWaitingAgent(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	first := startAgent(win, b.ID, "t", "w1")
	time.Sleep(2 * time.Millisecond)
	startAgent(win, b.ID, "t", "w2")
	startAgent(win, b.ID, "t", "run")
	for _, id := range []string{"w2", "w1"} {
		win.onNotify(notify.Msg{Tab: int(b.ID), Kind: notify.KindNeeds, Session: "t", AgentID: id})
	}
	twoFrames(t, win, r)
	win.toastGo(b.ID)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != b.ID || win.dockOf(b.ID) == nil || win.dockOf(b.ID).agent != first {
		t.Fatalf("Go did not open the first waiting agent: %+v", win.dockOf(b.ID))
	}
	win.toastGo(a.ID)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != a.ID || win.dockOf(a.ID) != nil {
		t.Fatal("Go with no waiting subagent should only switch")
	}
}

func TestDockForgetsGoneAgent(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	win.subs.CloseTab(int(a.ID))
	twoFrames(t, win, r)
	if win.dockOf(a.ID) != nil || !win.dockRect().Empty() {
		t.Fatal("panel kept for a forgotten agent")
	}
}

func TestShowAgentOpensAnAgentTab(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.showAgent(a.ID, k, true)
	twoFrames(t, win, r)
	d := win.deckOf(a.ID)
	if d == nil || len(d.entries) != 1 || d.entries[0].view == nil || d.entries[0].view.agent != k {
		t.Fatalf("deck %+v", d)
	}
}
