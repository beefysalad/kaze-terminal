package ui

import (
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
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
