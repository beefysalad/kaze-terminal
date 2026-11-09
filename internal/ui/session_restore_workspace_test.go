package ui

import (
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/sessionstate"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

const restoredWS = "ws_restored"

// restoredWorkspace restarts Kaze on a saved run of a workspace whose tabs are
// api (a command), notes (no commands) and agent (a resumed conversation).
func restoredWorkspace(t *testing.T) *runFixture {
	t.Helper()
	f, p := restoreFixture(t)
	f.save(workspaces.Workspace{ID: restoredWS, Name: "restored", Tabs: []workspaces.Tab{
		{ID: "t_api", Title: "api", Folder: f.dirs[0], Commands: []string{"npm run dev"}},
		{ID: "t_notes", Title: "notes", Folder: f.dirs[1]},
		{ID: "t_agent", Title: "agent", Folder: f.dirs[2], Commands: []string{"recipe-agent-line"}},
	}})
	entry := func(i int, title string) sessionstate.Entry {
		return sessionstate.Entry{Cwd: f.dirs[i], FixedTitle: title, Workspace: restoredWS, WorkspaceName: "restored", Run: 1, Member: i}
	}
	agent := entry(2, "agent")
	agent.Agent, agent.Session = "codex", "01234567-89ab-cdef-0123-456789abcdef"
	if err := sessionstate.Save(p, sessionstate.Snapshot{Version: 1, Entries: []sessionstate.Entry{entry(0, "api"), entry(1, "notes"), agent}}); err != nil {
		t.Fatal(err)
	}
	f.win.initRestore()
	t.Cleanup(f.win.closeRestore)
	f.win.openStartup(nil)
	for i := range 3 {
		f.prompt(i)
	}
	f.win.wsTick(f.now)
	f.win.launchTick(f.now)
	return f
}

func TestRestoredWorkspaceIsHonestAndStartsNothing(t *testing.T) {
	f := restoredWorkspace(t)
	f.wantStatus(0, "Not started")
	f.wantStatus(1, "No commands")
	f.wantStatus(2, "Resumed")
	if got := f.st.pty(0).sent(); got != "" {
		t.Fatalf("restart typed %q", got)
	}
	runs := f.win.sideRuns()
	if len(runs) != 1 {
		t.Fatalf("runs=%d", len(runs))
	}
	if text, _ := runHeaderState(runs[0], f.win.ui.th); text != "1 not started" {
		t.Fatalf("header=%q", text)
	}
	if got := f.win.wsBadge(restoredWS); got != "Not started" {
		t.Fatalf("badge=%q", got)
	}
}

func TestRunStartsRestoredMembersInTheirTabs(t *testing.T) {
	f := restoredWorkspace(t)
	tabsBefore := len(f.win.mgr.Tabs())
	f.win.runWorkspace(restoredWS)
	for _, r := range f.win.runs {
		if r.ctx.Err() != nil {
			t.Fatal("restored run has a cancelled context, so its port and Docker waits cannot work")
		}
	}
	f.win.wsTick(f.now)
	f.until("api commands sent", func() bool { return strings.Contains(f.st.pty(0).sent(), "npm run dev") })
	if got := len(f.win.mgr.Tabs()); got != tabsBefore {
		t.Fatalf("Run opened new tabs: %d to %d", tabsBefore, got)
	}
	if strings.Contains(f.st.pty(2).sent(), "recipe-agent-line") {
		t.Fatal("recipe typed over a resumed agent")
	}
	f.wantStatus(0, "Running")
	if text, _ := runHeaderState(f.win.sideRuns()[0], f.win.ui.th); text == "1 not started" {
		t.Fatal("header still says not started")
	}
}

func TestRunLeavesARestoredTabYouTypedInAlone(t *testing.T) {
	f := restoredWorkspace(t)
	f.win.takeOver(f.win.mgr.Tabs()[0])
	f.wantStatus(0, "In use")
	f.win.runWorkspace(restoredWS)
	f.win.wsTick(f.now)
	if got := f.st.pty(0).sent(); strings.Contains(got, "npm run dev") {
		t.Fatalf("typed over the owner's tab: %q", got)
	}
}

func TestGoOrRunStartsARestoredWorkspace(t *testing.T) {
	f := restoredWorkspace(t)
	f.win.goOrRun(restoredWS)
	f.win.wsTick(f.now)
	f.until("api commands sent", func() bool { return strings.Contains(f.st.pty(0).sent(), "npm run dev") })
}

func TestRestartShowsNoStartedCard(t *testing.T) {
	f := restoredWorkspace(t)
	for _, c := range f.win.alerts.toasts {
		if c.Run != 0 {
			t.Fatalf("restart claimed a run summary: %+v", c)
		}
	}
}
