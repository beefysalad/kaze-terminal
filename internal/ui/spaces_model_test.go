package ui

import (
	"reflect"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func spaceIDs(m spaceModel) []string {
	var keys []string
	for _, s := range m.Spaces {
		keys = append(keys, s.Key)
	}
	return keys
}

func TestSpacesGroupByRepoWithShellLast(t *testing.T) {
	ts, locs := sideFixture() // tabs 1,4 in X; 2 repo-less; 3 in Y
	m := buildSpaces(buildSide(sideIn(ts, locs)), 1)
	if got := spaceIDs(m); !reflect.DeepEqual(got, []string{"X", "Y", shellSpaceKey}) {
		t.Fatalf("spaces %v", got)
	}
	x := m.Spaces[0]
	if x.Name != "x" || x.Badge != 'X' || x.Count != 2 || !reflect.DeepEqual(x.spaceTabs(), []tabs.TabID{1, 4}) {
		t.Fatalf("x space %+v", x)
	}
	sh := m.Spaces[2]
	if sh.Name != "Shell" || sh.Badge != 'S' || !reflect.DeepEqual(sh.spaceTabs(), []tabs.TabID{2}) {
		t.Fatalf("shell space %+v", sh)
	}
	if buildSpaces(buildSide(sideIn(ts, locs)), 1).Spaces[0].Color != x.Color {
		t.Fatal("colour not stable")
	}
}

func TestSpacesExcludeMembers(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Member = func(tb *tabs.Tab) (uint64, bool) { return 1, tb.ID == 4 }
	m := buildSpaces(buildSide(in), 1)
	if got := m.Spaces[0].spaceTabs(); !reflect.DeepEqual(got, []tabs.TabID{1}) {
		t.Fatalf("member leaked into space: %v", got)
	}
}

func TestSelectedSpaceFollowsActiveTab(t *testing.T) {
	ts, locs := sideFixture()
	side := buildSide(sideIn(ts, locs))
	for id, want := range map[tabs.TabID]string{1: "X", 4: "X", 3: "Y", 2: shellSpaceKey} {
		m := buildSpaces(side, id)
		s, ok := m.selectedSpace()
		if !ok || s.Key != want || !s.Selected {
			t.Fatalf("active %d: selected %+v ok=%v, want %s", id, s, ok, want)
		}
	}
	if _, ok := buildSpaces(side, 99).selectedSpace(); ok {
		t.Fatal("unknown active tab selected a space")
	}
}

func TestClosingLastTabRemovesSpace(t *testing.T) {
	ts, locs := sideFixture()
	rest := []*tabs.Tab{ts[0], ts[1], ts[3]} // tab 3 (only member of Y) closed
	m := buildSpaces(buildSide(sideIn(rest, locs)), 1)
	if got := spaceIDs(m); !reflect.DeepEqual(got, []string{"X", shellSpaceKey}) {
		t.Fatalf("spaces after close %v", got)
	}
}

func TestSpaceCountsWaiting(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Mark = func(id tabs.TabID) mark {
		if id == 4 {
			return markNeeds
		}
		return markNone
	}
	m := buildSpaces(buildSide(in), 1)
	if m.Spaces[0].Waiting != 1 || m.Spaces[1].Waiting != 0 {
		t.Fatalf("waiting %+v", m.Spaces)
	}
}

func TestAgentsGroupOrder(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	now := time.Now()
	in.Agent = func(id tabs.TabID) agentState {
		switch id {
		case 1:
			return agentState{claude: true, running: true, since: now}
		case 2:
			return agentState{agent: "codex"}
		case 3:
			return agentState{claude: true}
		}
		return agentState{} // tab 4 runs no agent
	}
	in.Mark = func(id tabs.TabID) mark {
		if id == 3 {
			return markNeeds
		}
		return markNone
	}
	m := buildAgents(buildSide(in))
	var got []tabs.TabID
	var states []agentGroup
	for _, r := range m.Rows {
		got = append(got, r.ID)
		states = append(states, r.State)
	}
	if !reflect.DeepEqual(got, []tabs.TabID{3, 1, 2}) {
		t.Fatalf("order %v", got)
	}
	if !reflect.DeepEqual(states, []agentGroup{agentNeeds, agentWorking, agentIdle}) {
		t.Fatalf("states %v", states)
	}
	if m.Rows[1].Name != "claude" || m.Rows[2].Name != "codex" || !m.Rows[1].Since.Equal(now) {
		t.Fatalf("rows %+v", m.Rows)
	}
}

func TestAgentsExcludeMembersAndEmpty(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Agent = func(tabs.TabID) agentState { return agentState{claude: true} }
	in.Member = func(tb *tabs.Tab) (uint64, bool) { return 1, tb.ID != 1 }
	m := buildAgents(buildSide(in))
	if len(m.Rows) != 1 || m.Rows[0].ID != 1 {
		t.Fatalf("rows %+v", m.Rows)
	}
	if len(buildAgents(sideModel{}).Rows) != 0 {
		t.Fatal("empty model has rows")
	}
}

func TestSideModelCarriesSpacesAndAgents(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Agent = func(id tabs.TabID) agentState {
		if id == 3 {
			return agentState{claude: true, running: true, command: "go test"}
		}
		return agentState{}
	}
	m := buildSide(in)
	if got := []string{m.Spaces[0].Key, m.Spaces[1].Key, m.Spaces[2].Key}; !reflect.DeepEqual(got, []string{"X", "Y", shellSpaceKey}) {
		t.Fatalf("model spaces %v", got)
	}
	if len(m.Agents) != 1 || m.Agents[0].ID != 3 || m.Agents[0].Space != "y" || !m.Agents[0].Running || m.Agents[0].Subtitle != "go test" {
		t.Fatalf("model agents %+v", m.Agents)
	}
}

func TestFilterNarrowsSpacesAndAgents(t *testing.T) {
	ts, locs := sideFixture()
	in := sideIn(ts, locs)
	in.Agent = func(id tabs.TabID) agentState {
		if id == 1 || id == 3 {
			return agentState{claude: true}
		}
		return agentState{}
	}
	in.Filter = "y" // the name of space Y; it also appears in no other space
	m := buildSide(in)
	if len(m.Spaces) != 1 || m.Spaces[0].Key != "Y" || m.Spaces[0].Count != 1 {
		t.Fatalf("spaces after filter %+v", m.Spaces)
	}
	if len(m.Agents) != 1 || m.Agents[0].ID != 3 {
		t.Fatalf("agents after filter %+v", m.Agents)
	}
	in.Filter = "shell"
	if m := buildSide(in); len(m.Spaces) != 1 || m.Spaces[0].Key != shellSpaceKey {
		t.Fatalf("filtering by the Shell space's name gave %+v", m.Spaces)
	}
}
