package ui

import (
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// spacesWindow has space alpha (a1, a2), space beta (b1) and one Shell tab (s1),
// a1 shown, after one frame.
func spacesWindow(t *testing.T) (*window, *input.Router, map[string]*tabs.Tab) {
	t.Helper()
	win, _ := sideWindow(t, true)
	ts := map[string]*tabs.Tab{
		"a1": openIn(t, win, osPath("/code/alpha/one")),
		"a2": openIn(t, win, osPath("/code/alpha/two")),
		"b1": openIn(t, win, osPath("/code/beta/one")),
		"s1": openIn(t, win, ""),
	}
	win.mgr.SetActive(ts["a1"].ID)
	r := new(input.Router)
	sideFrame(t, win, r)
	return win, r, ts
}

func spaceRect(t *testing.T, win *window, key string) sideKeyHit {
	t.Helper()
	for _, h := range win.side.hits.spaces {
		if h.key == key {
			return h
		}
	}
	t.Fatalf("no space line for %q in %+v", key, win.side.hits.spaces)
	return sideKeyHit{}
}

func TestSidebarDrawsOneCalmLinePerSpace(t *testing.T) {
	win, _, _ := spacesWindow(t)
	hs := win.side.hits.spaces
	if len(hs) != 3 {
		t.Fatalf("%d space lines, want 3 (alpha, beta, Shell): %+v", len(hs), hs)
	}
	for i, h := range hs {
		if h.rect.Dy() != sideSpaceH {
			t.Fatalf("space %d is %d px high, want %d", i, h.rect.Dy(), sideSpaceH)
		}
		if i > 0 && h.rect.Min.Y < hs[i-1].rect.Max.Y {
			t.Fatalf("space %d overlaps the one above: %v after %v", i, h.rect, hs[i-1].rect)
		}
	}
	if len(win.side.hits.rows) != 0 {
		t.Fatalf("%d session rows drawn for sessions that run no agent, want none: %+v", len(win.side.hits.rows), win.side.hits.rows)
	}
}

func TestClickingASpaceShowsItsLastTab(t *testing.T) {
	win, r, ts := spacesWindow(t)
	win.selectTab(ts["a2"].ID)
	sideFrame(t, win, r)
	sideClick(win, r, spaceRect(t, win, "key-beta").rect, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	if got := win.mgr.Active().ID; got != ts["b1"].ID {
		t.Fatalf("clicking space beta showed %d, want b1 (%d)", got, ts["b1"].ID)
	}
	sideClick(win, r, spaceRect(t, win, "key-alpha").rect, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	if got := win.mgr.Active().ID; got != ts["a2"].ID {
		t.Fatalf("clicking space alpha showed %d, want its last used tab a2 (%d)", got, ts["a2"].ID)
	}
}

func TestSelectedSpaceLineIsMarkedAndOthersAreNot(t *testing.T) {
	win, r, ts := spacesWindow(t)
	if win.side.st.space != "key-alpha" {
		t.Fatalf("selected space %q, want key-alpha", win.side.st.space)
	}
	win.selectTab(ts["s1"].ID)
	sideFrame(t, win, r)
	if win.side.st.space != shellSpaceKey {
		t.Fatalf("selected space %q after showing the Shell tab, want Shell", win.side.st.space)
	}
}

func TestAgentsSectionGroupsAndClickSelects(t *testing.T) {
	win, r, ts := spacesWindow(t)
	setAgent(win, ts["a1"].ID, agentState{claude: true})                // idle
	setAgent(win, ts["b1"].ID, agentState{claude: true, running: true}) // working
	setAgent(win, ts["s1"].ID, agentState{agent: "codex"})              // needs you
	win.alerts.marks = map[tabs.TabID]mark{ts["s1"].ID: markNeeds}
	sideFrame(t, win, r)
	rows := win.side.hits.rows
	if len(rows) != 3 {
		t.Fatalf("%d agent rows, want 3: %+v", len(rows), rows)
	}
	want := []tabs.TabID{ts["s1"].ID, ts["b1"].ID, ts["a1"].ID} // Needs you, Working, Idle
	for i, id := range want {
		if rows[i].id != id {
			t.Fatalf("agent row %d is tab %d, want %d (Needs you, Working, Idle order)", i, rows[i].id, id)
		}
		if i > 0 && rows[i].rect.Min.Y < rows[i-1].rect.Max.Y {
			t.Fatalf("agent rows overlap: %v after %v", rows[i].rect, rows[i-1].rect)
		}
	}
	if rows[0].rect.Dy() != sideAgentRowH {
		t.Fatalf("agent row is %d px high, want %d", rows[0].rect.Dy(), sideAgentRowH)
	}
	sideClick(win, r, rows[0].rect, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	if got := win.mgr.Active().ID; got != ts["s1"].ID {
		t.Fatalf("clicking the needs-you agent showed %d, want its tab %d", got, ts["s1"].ID)
	}
	if win.side.st.space != shellSpaceKey {
		t.Fatalf("the agent click left space %q selected, want its space (Shell)", win.side.st.space)
	}
}

func TestSearchIconOpensTheFilterAndPlusOpensATab(t *testing.T) {
	win, r, _ := spacesWindow(t)
	h := win.side.hits
	if !h.filter.Empty() {
		t.Fatalf("the filter field %v is drawn before the search icon is used", h.filter)
	}
	if h.search.Empty() || h.plus.Empty() || h.search.Max.X > h.plus.Min.X {
		t.Fatalf("header icons: search %v, plus %v, want search left of plus", h.search, h.plus)
	}
	sideClick(win, r, h.search, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	if !win.side.filtering || win.side.hits.filter.Empty() || !r.Source().Focused(&win.side.ed) {
		t.Fatalf("search icon: filtering %v, field %v, focused %v", win.side.filtering, win.side.hits.filter, r.Source().Focused(&win.side.ed))
	}
	before := len(win.mgr.Tabs())
	sideClick(win, r, win.side.hits.plus, pointer.ButtonPrimary)
	sideFrame(t, win, r)
	if got := len(win.mgr.Tabs()); got != before+1 {
		t.Fatalf("plus: %d tabs, want %d", got, before+1)
	}
}

func TestHoveringASpaceRedrawsOnceAndSettles(t *testing.T) {
	win, r, _ := spacesWindow(t)
	sideFrame(t, win, r)
	draws := win.side.cache.draws
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: sidePt(win, centre(spaceRect(t, win, "key-beta").rect))})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	if win.side.st.hoverSpace != "key-beta" {
		t.Fatalf("hover space %q, want key-beta", win.side.st.hoverSpace)
	}
	if got := win.side.cache.draws - draws; got != 1 {
		t.Fatalf("hovering a space rebuilt the sidebar ops %d times, want 1", got)
	}
}

func TestFilterNarrowsAgentRowsToo(t *testing.T) {
	win, r, ts := spacesWindow(t)
	markAgent(win, ts["a1"].ID)
	markAgent(win, ts["b1"].ID)
	sideFrame(t, win, r)
	if len(win.side.hits.rows) != 2 {
		t.Fatalf("%d agent rows before the filter, want 2", len(win.side.hits.rows))
	}
	focusFilter(t, win, r)
	typeFilter(t, win, r, "beta")
	if len(win.side.hits.rows) != 1 || win.side.hits.rows[0].id != ts["b1"].ID {
		t.Fatalf("agent rows after filtering for beta: %+v, want only b1", win.side.hits.rows)
	}
	if len(win.side.hits.spaces) != 1 || win.side.hits.spaces[0].key != "key-beta" {
		t.Fatalf("spaces after filtering for beta: %+v", win.side.hits.spaces)
	}
}
