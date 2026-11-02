package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"github.com/patrick-gogo/kaze-terminal/internal/branchsession"
)

func TestBranchLauncherToggleShortcutAndCancel(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/repo"))
	var r input.Router
	sideFrame(t, win, &r)
	if win.side.hits.launchToggle.Empty() || win.side.hits.launchButton.Empty() {
		t.Fatal("launcher not shown")
	}
	sideClick(win, &r, win.side.hits.launchToggle, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	if win.opt.Config.SidebarLauncher || !win.side.hits.launchButton.Empty() || win.side.hits.launchToggle.Empty() {
		t.Fatal("collapse lost restore affordance")
	}
	win.runShortcut(pageContext(1000, 600), actNewBranchSession)
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	if !win.side.launch.open || !win.opt.Config.SidebarLauncher || !r.Source().Focused(&win.side.launch.ed) {
		t.Fatal("shortcut did not open and focus picker")
	}
	pressKey(t, win, &r, key.NameEscape, 0)
	if win.side.launch.open {
		t.Fatal("Esc did not cancel")
	}
}
func TestBranchLaunchCommandWaitsForShell(t *testing.T) {
	for _, agent := range []string{"claude", "codex", "shell"} {
		t.Run(agent, func(t *testing.T) {
			win, st := sideWindow(t, true)
			openIn(t, win, "")
			n := len(win.mgr.Tabs())
			win.side.launch.agent = agent
			win.finishBranchSession(branchsession.Result{Record: branchsession.Record{Folder: t.TempDir(), Branch: "task"}})
			ts := win.mgr.Tabs()
			if len(ts) != n+1 {
				t.Fatal("no session")
			}
			tab := ts[len(ts)-1]
			pending := win.launchOf(tab)
			want := map[string]string{"claude": "claude", "codex": "codex --no-daemon", "shell": ""}[agent]
			if want == "" {
				if pending != nil {
					t.Fatal("shell queued agent")
				}
			} else if pending == nil || pending.run.Line != want {
				t.Fatalf("launch %+v", pending)
			}
			if len(st.ptys) != n+1 {
				t.Fatal("shell start")
			}
		})
	}
}
func TestNewBranchSessionBindings(t *testing.T) {
	for os, mods := range map[string]key.Modifiers{"darwin": key.ModCommand | key.ModShift, "windows": key.ModCtrl | key.ModShift} {
		b := defaultBindings(os)["new_branch_session"]
		if b.Name != "N" || b.Mods != mods {
			t.Fatalf("%s: %+v", os, b)
		}
	}
}

func TestBranchPickerWorksInNarrowWindow(t *testing.T) {
	win, _ := sideWindow(t, true)
	openIn(t, win, "")
	var r input.Router
	sizedFrame(t, win, &r, 500, 600)
	if !win.side.hidden {
		t.Fatal("fixture sidebar shown")
	}
	win.openBranchLauncher()
	sizedFrame(t, win, &r, 500, 600)
	sizedFrame(t, win, &r, 500, 600)
	if win.side.hidden || win.side.hits.launchBranch.Empty() || !r.Source().Focused(&win.side.launch.ed) {
		t.Fatal("shortcut could not open narrow picker")
	}
	pressKey(t, win, &r, key.NameEscape, 0)
	sizedFrame(t, win, &r, 500, 600)
	if !win.side.hidden {
		t.Fatal("sidebar did not return to automatic narrow hiding")
	}
}

func TestBranchWorkerSuccessDuplicateSubmitAndFailure(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.test", "commit", "--allow-empty", "-m", "initial"}} {
		c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%v: %s", e, b)
		}
	}
	dir, _ = filepath.EvalSymlinks(dir)
	// Worktree remains inside this test's temporary parent.
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	c := exec.CommandContext(ctx, "git", "clone", "--local", dir, repo)
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	repo, _ = filepath.EvalSymlinks(repo)
	win, st := sideWindow(t, true)
	win.opt.Shell = []string{"zsh"}
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	openIn(t, win, "")
	l := &win.side.launch
	l.open = true
	l.agent = "codex"
	l.repos = []branchRepo{{key: "repo", name: "repo", dir: repo}}
	l.ed.SetText("fix/task")
	win.submitBranch()
	win.submitBranch()
	if len(win.mgr.Tabs()) != 1 {
		t.Fatal("session started before git completed")
	}
	deadline := time.Now().Add(5 * time.Second)
	for l.busy {
		win.drainBranchWork()
		if time.Now().After(deadline) {
			t.Fatal("branch worker timed out")
		}
		time.Sleep(time.Millisecond)
	}
	if l.err != "" || len(win.mgr.Tabs()) != 2 {
		t.Fatalf("launch: %s / %d tabs", l.err, len(win.mgr.Tabs()))
	}
	tab := win.mgr.Active()
	if tab.Dir != repo+"-fix-task" {
		t.Fatalf("wrong directory %q", tab.Dir)
	}
	win.launchTick(time.Now())
	if got := st.ptys[1].sent(); got != "" {
		t.Fatalf("agent started before shell readiness: %q", got)
	}
	report(t, tab, tab.Dir)
	win.launchTick(time.Now())
	waitSent(t, st.ptys[1], "codex --no-daemon\r")
	l.open = true
	l.ed.SetText("invalid..branch")
	win.submitBranch()
	for l.busy {
		win.drainBranchWork()
		if time.Now().After(deadline) {
			t.Fatal("failure worker timed out")
		}
		time.Sleep(time.Millisecond)
	}
	if l.err == "" || len(win.mgr.Tabs()) != 2 {
		t.Fatal("Git failure opened a session or lost the error")
	}
	waitCleanup := func() {
		t.Helper()
		for l.work.busy {
			win.drainBranchWork()
			if time.Now().After(deadline) {
				t.Fatal("cleanup worker timed out")
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitCleanup()
	win.closeTab(tab.ID)
	waitCleanup()
	if !l.work.due.IsZero() {
		for repo := range l.work.last {
			l.work.last[repo] = time.Now().Add(-2 * time.Minute)
		}
		l.work.due = time.Now().Add(-time.Second)
		win.worktreeTick(time.Now())
		waitCleanup()
	}
	if len(l.work.confirm) != 1 || l.work.confirm[0].Folder != tab.Dir {
		t.Fatalf("clean close offer missing: %+v", l.work)
	}
	win.keepWorktrees()
}

func TestBranchRepoPopupWinsOverUnderlyingFields(t *testing.T) {
	win, _ := sideWindow(t, true)
	first := openIn(t, win, osPath("/code/first"))
	openIn(t, win, osPath("/code/second"))
	var r input.Router
	sideFrame(t, win, &r)
	win.mgr.SetActive(first.ID)
	win.openBranchLauncher()
	sideFrame(t, win, &r)
	sideClick(win, &r, win.side.hits.launchRepo, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	if len(win.side.hits.launchRepos) != 2 {
		t.Fatal("repo menu not open")
	}
	sideClick(win, &r, win.side.hits.launchRepos[1], pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	if win.side.launch.repo != 1 || win.side.launch.repoOpen {
		t.Fatal("repo menu click reached underlying field")
	}
}

func TestBranchRepoPopupKeepsEscapeAndKeyboardSelection(t *testing.T) {
	win, _ := sideWindow(t, true)
	first := openIn(t, win, osPath("/code/first"))
	openIn(t, win, osPath("/code/second"))
	win.mgr.SetActive(first.ID)
	var r input.Router
	sideFrame(t, win, &r)
	win.openBranchLauncher()
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	sideClick(win, &r, win.side.hits.launchRepo, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	if !r.Source().Focused(&win.side.launch.ed) {
		t.Fatal("repo popup lost keyboard target")
	}
	pressKey(t, win, &r, key.NameDownArrow, 0)
	pressKey(t, win, &r, key.NameReturn, 0)
	if win.side.launch.repo != 1 || win.side.launch.repoOpen || !win.side.launch.open {
		t.Fatal("repo keyboard selection submitted session")
	}
	sideClick(win, &r, win.side.hits.launchRepo, pointer.ButtonPrimary)
	sideFrame(t, win, &r)
	pressKey(t, win, &r, key.NameEscape, 0)
	if win.side.launch.open {
		t.Fatal("Escape did not cancel from repo popup")
	}
}
