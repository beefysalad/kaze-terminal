package ui

import (
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestDropStatusMark(t *testing.T) {
	cases := [][2]string{
		{"◐ Epic", "Epic"},
		{"⠂ Epic", "Epic"},
		{"✳ Epic", "Epic"},
		{"✳ ◐ Epic", "Epic"},
		{"◓◑ Epic V3-3347 tickets", "Epic V3-3347 tickets"},
		{"Epic", "Epic"},
		{"✳ ◐", "✳ ◐"},
		{"", ""},
	}
	for _, c := range cases {
		if got := dropStatusMark(c[0]); got != c[1] {
			t.Errorf("dropStatusMark(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestDropStatusMarkKeepsASCII(t *testing.T) {
	for _, s := range []string{"~/code/kaze", "+x", "<draft>"} {
		if got := dropStatusMark(s); got != s {
			t.Errorf("dropStatusMark(%q) = %q, want it unchanged", s, got)
		}
	}
}

func TestRuledStatusOnlyTitleFallsBack(t *testing.T) {
	for _, c := range []struct{ title, want string }{{"✳ ◐", "api"}, {"◐ Epic", "Epic"}, {"⠂", "api"}} {
		ts := []*tabs.Tab{sideTab(1, c.title)}
		in := sideIn(ts, map[tabs.TabID]locEntry{1: {cwd: "/code/repo/api"}})
		in.Agent = func(tabs.TabID) agentState { return agentState{claude: true} }
		m := buildSide(in)
		if len(m.Loose) != 1 || m.Loose[0].Title != c.want {
			t.Errorf("title %q: rows %+v, want title %q", c.title, m.Loose, c.want)
		}
	}
}

func TestResourceNameDropsSpinner(t *testing.T) {
	win, _ := sideWindow(t, true)
	for _, title := range []string{"◐ Epic", "⌘ Epic", "∑ Epic"} {
		if got := win.tabResourceName(&tabs.Tab{Title: title}); got != "Epic" {
			t.Errorf("tabResourceName(%q) = %q, want Epic", title, got)
		}
	}
}

// TestStatusMarkTakesVariationSelectors: FE0F/FE0E, ZWJ and skin-tone modifiers
// belong to the emoji before them, so they never end the leading status run.
func TestStatusMarkTakesVariationSelectors(t *testing.T) {
	for _, s := range []string{"⚡\uFE0F", "⚡\uFE0E", "👍\U0001F3FD", "✳\u200D⚡\uFE0F "} {
		if !onlyStatus(s) {
			t.Errorf("onlyStatus(%q) = false, want true", s)
		}
	}
	for _, c := range [][2]string{{"⚡\uFE0F Deploy", "Deploy"}, {"👍\U0001F3FD Ship", "Ship"}, {"✳\u200D⚡ Run", "Run"}} {
		if got := dropStatusMark(c[0]); got != c[1] {
			t.Errorf("dropStatusMark(%q) = %q, want %q", c[0], got, c[1])
		}
	}
	ts := []*tabs.Tab{sideTab(1, "⚡\uFE0F")}
	in := sideIn(ts, map[tabs.TabID]locEntry{1: {cwd: "/code/repo/api"}})
	in.Agent = func(tabs.TabID) agentState { return agentState{claude: true} }
	if m := buildSide(in); len(m.Loose) != 1 || m.Loose[0].Title != "api" {
		t.Errorf("title ⚡️: rows %+v, want the folder api", m.Loose)
	}
}
