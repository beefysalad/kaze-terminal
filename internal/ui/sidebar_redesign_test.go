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

func TestSidebarLinesStayCalm(t *testing.T) {
	if sideSpaceH != 34 || sideAgentRowH > 32 {
		t.Fatalf("a space line is %ddp and an agent row %ddp tall, want 34 and at most 32", sideSpaceH, sideAgentRowH)
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
