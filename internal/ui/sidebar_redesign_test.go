package ui

import (
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestSidebarRedesignNamePriority(t *testing.T) {
	for _, tc := range []struct{ name, title, folder, repo, agent, arg, owned, want string }{
		{name: "owner", title: "✳ Agent summary", folder: "/code/repo", repo: "repo", agent: "claude", arg: "ticket", owned: "My work", want: "My work"},
		{name: "argument", title: "✳ Agent summary", folder: "/code/repo", repo: "repo", agent: "claude", arg: "ticket", want: "ticket"},
		{name: "summary", title: "⠋ ✳ Agent summary", folder: "/code/repo", repo: "repo", agent: "claude", want: "Agent summary"},
		{name: "agent fallback", folder: "/code/repo", repo: "repo", agent: "codex", want: "codex"},
		{name: "repo shell", title: "old summary", folder: "/code/repo", repo: "repo", want: "zsh"},
		{name: "subfolder", title: "old summary", folder: "/code/repo/api", repo: "repo", want: "api"},
		{name: "left agent", title: "old summary", folder: "/code/repo", repo: "repo", arg: "Kept", want: "Kept"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := sideIn([]*tabs.Tab{sideTab(1, tc.title)}, map[tabs.TabID]locEntry{1: {cwd: tc.folder, repo: "key", repoName: tc.repo}})
			in.DefTitle = "zsh"
			in.Agent = func(tabs.TabID) agentState { return agentState{agent: tc.agent, arg: tc.arg} }
			in.Name = func(tabs.TabID) string { return tc.owned }
			if got := buildSide(in).Groups[0].Rows[0].Title; got != tc.want {
				t.Fatalf("name %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSidebarRedesignStatusPriority(t *testing.T) {
	for _, tc := range []struct {
		r    sideRow
		want string
	}{
		{sideRow{Mark: markNeeds, Running: true, AgentCount: 3}, "needs you"},
		{sideRow{Mark: markDone, Running: true, AgentCount: 3}, "done"},
		{sideRow{Running: true, AgentCount: 3}, "3 agents"},
		{sideRow{AgentCount: 1}, "1 agent"},
		{sideRow{Running: true}, "running"},
	} {
		if got := tc.r.stateWord(); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.r, got, tc.want)
		}
	}
	if sideRowH > 48 {
		t.Fatalf("two-line row is %ddp tall", sideRowH)
	}
}

func TestSidebarRedesignWorktreeDetail(t *testing.T) {
	for _, tc := range []struct{ top, branch, want string }{
		{"/code/repo-auth", "feature/auth", "⑂ repo-auth · feature/auth"},
		{"/code/repo-feature-auth", "feature/auth", "auth"},
	} {
		in := sideIn([]*tabs.Tab{sideTab(1, "")}, map[tabs.TabID]locEntry{1: {cwd: tc.top + "/src", repo: "key", repoName: "repo", branch: tc.branch, worktree: tc.top}})
		in.DefTitle = "zsh"
		r := buildSide(in).Groups[0].Rows[0]
		if r.Title != "zsh" || r.Detail != tc.want {
			t.Fatalf("row %+v, want zsh / %q", r, tc.want)
		}
	}
}
