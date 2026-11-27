package ui

import (
	"fmt"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"image"
	"testing"
)

func TestSidebarNestedOwnership(t *testing.T) {
	side := sideModel{Loose: []sideRow{
		{ID: 1, Agent: "claude", Docs: []sideDoc{{ID: 1, Name: "owned.md"}}, Agents: []sideAgent{{Type: "Explore", Task: "inspect"}}, TrayTotal: 1},
		{ID: 2, Docs: []sideDoc{{ID: 2, Name: "shell.md"}}},
	}}
	spaces := buildSpaces(side, 0).Spaces
	if len(spaces[0].Docs) != 1 || spaces[0].Docs[0].ID != 2 {
		t.Fatalf("space must contain only agentless docs: %+v", spaces[0].Docs)
	}
	rows := buildAgents(side).Rows
	gtx := layout.Context{}
	base := gtx.Dp(sideHeadH + sideAgentLabelH + sideAgentRowH + 6)
	if agentsSpan(gtx, rows) <= base {
		t.Fatal("agent height omits nested tray and document")
	}
}

func TestNestedDocReveal(t *testing.T) {
	m := sideModel{Spaces: []spaceEntry{{Key: "shell"}}, Agents: []agentRow{{ID: 1, Docs: []sideDoc{{ID: 9}}}}}
	gtx := pageContext(260, 120)
	h := sideHits{list: image.Rect(0, 0, 260, 84), scrollMax: 100}
	sc, ok := revealScroll(gtx, m, sideState{doc: 9, space: "shell"}, h)
	if !ok || sc == 0 {
		t.Fatalf("nested doc was not revealed: %d %v", sc, ok)
	}
}

func TestNestedSidebarHits(t *testing.T) {
	win := testWindow(t, &recStarter{})
	win.ui = newChrome()
	gtx := pageContext(260, 600)
	r := sideRow{ID: 1, Agent: "claude", Docs: []sideDoc{{ID: 9, Name: "plan.md"}}, Agents: []sideAgent{{Type: "Explore", Task: "inspect"}}, Hidden: subagents.TrayCounts{Running: 2}, TrayTotal: 3}
	m := sideModel{Loose: []sideRow{r, {ID: 2, Docs: []sideDoc{{ID: 10, Name: "shell.md"}}}}}
	m.Spaces = buildSpaces(m, 0).Spaces
	m.Agents = buildAgents(m).Rows
	for _, right := range []bool{false, true} {
		h := win.ui.layoutSidebar(gtx, m, sideState{right: right}, image.Pt(260, 600))
		if len(h.agents) != 2 || !h.agents[1].more || h.agents[1].tab != 1 {
			t.Fatalf("tray/board hits: %+v", h.agents)
		}
		if len(h.docs) != 2 || h.docs[1].rect.Min.Y < h.agents[1].rect.Max.Y || h.agents[0].rect.Min.Y < h.rows[0].rect.Max.Y {
			t.Fatalf("nested order: %+v", h)
		}
		full := agentsSpanState(gtx, m.Agents, sideState{})
		h = win.ui.layoutSidebar(gtx, m, sideState{right: right, scroll: 999}, image.Pt(260, 140))
		if h.scrollMax <= 0 {
			t.Fatal("missing scroll")
		}
		for _, d := range h.docs {
			if d.rect != d.rect.Intersect(h.list) {
				t.Fatal("doc hit outside list")
			}
		}
		folded := win.ui.layoutSidebar(gtx, m, sideState{right: right, trayFolded: map[tabs.TabID]bool{1: true}}, image.Pt(260, 600))
		if len(folded.agents) != 0 || agentsSpanState(gtx, m.Agents, sideState{trayFolded: map[tabs.TabID]bool{1: true}}) >= full {
			t.Fatal("fold did not remove tray height")
		}
	}
}

func TestNestedTrayClicksAndFilter(t *testing.T) {
	win, router, ts := spacesWindow(t)
	tab := ts["a1"].ID
	setAgent(win, tab, agentState{claude: true})
	k := startAgent(win, tab, "nested", "explore")
	sideFrame(t, win, router)
	if len(win.side.hits.agents) != 1 {
		t.Fatal("missing live tray")
	}
	win.sidePress(centre(win.side.hits.agents[0].rect), pointer.ButtonPrimary)
	if d := win.dockOf(tab); d == nil || d.agent != k {
		t.Fatal("tray did not open dock")
	}
	win.sidePress(centre(win.side.hits.agents[0].rect), pointer.ButtonPrimary)
	if win.dockOf(tab) != nil {
		t.Fatal("tray did not toggle dock")
	}
	for i := 0; i < 4; i++ {
		startAgent(win, tab, "nested", fmt.Sprint(i))
	}
	sideFrame(t, win, router)
	hits := win.side.hits.agents
	if len(hits) != trayCap+1 || !hits[trayCap].more {
		t.Fatalf("cap: %+v", hits)
	}
	win.sidePress(centre(hits[trayCap].rect), pointer.ButtonPrimary)
	if !win.board.open || win.board.tab != tab {
		t.Fatal("overflow did not open session board")
	}
	in := win.sideIn()
	in.Filter = "explore"
	if len(buildSide(in).Agents) != 1 {
		t.Fatal("tray filter lost owning agent")
	}
}
