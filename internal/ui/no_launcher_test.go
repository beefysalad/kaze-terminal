package ui

import (
	"image"
	"testing"
)

// "New session on a branch" is gone (2026-11-06): the agents make branches and
// worktrees themselves, and the title bar's + already opens a session.
func TestNoBranchLauncherCommand(t *testing.T) {
	for _, c := range kazeCommands {
		if c.Config == "new_branch_session" {
			t.Fatalf("the %q command is still listed", c.Label)
		}
	}
	for _, goos := range []string{"windows", "darwin"} {
		if _, ok := defaultBindings(goos)["new_branch_session"]; ok {
			t.Fatalf("%s still binds new_branch_session", goos)
		}
		// A config written before the removal still loads without a warning.
		if _, warns := resolveBindings(goos, map[string]string{"new_branch_session": "ctrl+shift+n"}); len(warns) != 0 {
			t.Fatalf("%s: old new_branch_session key warns %q", goos, warns)
		}
	}
}

// With no launcher section, the session list reaches the sidebar's bottom.
func TestSidebarListReachesBottom(t *testing.T) {
	m := sideModel{Loose: sideRows(1, 3, "s")}
	gtx := pageContext(sidebarW, 300)
	h := newChrome().layoutSidebar(gtx, m, sideState{shown: 1}, image.Pt(sidebarW, 300))
	if h.list.Max.Y != 300 {
		t.Fatalf("list %v, want it to end at the bottom (300)", h.list)
	}
}
