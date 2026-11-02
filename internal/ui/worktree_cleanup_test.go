package ui

import (
	"path/filepath"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"github.com/patrick-gogo/kaze-terminal/internal/branchsession"
)

func TestWorktreeOpenIncludesScratchAndWorkspaceFolders(t *testing.T) {
	win, _ := sideWindow(t, true)
	root := filepath.Join(t.TempDir(), "repo-task")
	tab := openIn(t, win, filepath.Join(root, "src"))
	if !win.worktreeOpen(root) {
		t.Fatal("nested cwd not counted")
	}
	tab.Parent = 123
	if !win.worktreeOpen(root) {
		t.Fatal("scratch ignored")
	}
	tab.Parent = 0
	openMember(t, win, "server")
	win.mgr.Close(tab.ID)
	member := win.mgr.Tabs()[0]
	member.Dir = filepath.Join(root, "api")
	if !win.worktreeOpen(root) {
		t.Fatal("workspace launch folder ignored")
	}
}
func TestWorktreeCleanupOffersOnlyClosedCleanOwnedFolders(t *testing.T) {
	win, _ := sideWindow(t, true)
	root := t.TempDir()
	w := &win.side.launch.work
	w.closed = []string{root}
	st := branchsession.Status{Record: branchsession.Record{Folder: root, Repo: "repo", Branch: "task"}, Clean: true}
	win.applyWorktreeStatuses([]branchsession.Status{st})
	if len(w.confirm) != 1 {
		t.Fatal("no close offer")
	}
	win.keepWorktrees()
	win.applyWorktreeStatuses([]branchsession.Status{st})
	if len(w.confirm) != 0 {
		t.Fatal("Keep asked twice")
	}
	win.worktreeOpened(root)
	w.closed = []string{root}
	st.Clean = false
	win.applyWorktreeStatuses([]branchsession.Status{st})
	if len(w.confirm) != 0 {
		t.Fatal("dirty offer")
	}
	st.Clean = true
	openIn(t, win, root)
	win.applyWorktreeStatuses([]branchsession.Status{st})
	if len(w.confirm) != 0 {
		t.Fatal("open offer")
	}
}

func TestCleanupRefusalRemainsVisibleAfterLastTab(t *testing.T) {
	win, _ := sideWindow(t, true)
	tab := openIn(t, win, "")
	win.mgr.Close(tab.ID)
	w := &win.side.launch.work
	w.confirm = []branchsession.Record{{Folder: t.TempDir()}}
	w.results = make(chan worktreeResult, 1)
	w.results <- worktreeResult{removed: true, failures: []string{"repo-task: git refused: contains untracked files"}}
	win.drainWorktreeChecks()
	if win.shouldClose() || len(w.errors) != 1 {
		t.Fatal("last-tab cleanup refusal disappeared")
	}
	var r input.Router
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	if w.keep.Empty() {
		t.Fatal("no dismiss action")
	}
	r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	sideFrame(t, win, &r)
	if !win.shouldClose() {
		t.Fatal("could not close after reading refusal")
	}
}

func TestDeferredCloseCheckKeepsWindowUntilAnswered(t *testing.T) {
	win, _ := sideWindow(t, true)
	tab := openIn(t, win, "")
	win.mgr.Close(tab.ID)
	w := &win.side.launch.work
	w.checking = true
	w.due = time.Now().Add(time.Minute)
	at, ok := win.worktreeTick(time.Now())
	if !ok || !at.Equal(w.due) || win.shouldClose() {
		t.Fatal("deferred cleanup was lost")
	}
	win.keepWorktrees()
	if win.worktreeDialogShown() || !win.shouldClose() {
		t.Fatal("Keep did not cancel pending cleanup")
	}
	if _, ok = win.worktreeTick(time.Now().Add(2 * time.Minute)); ok {
		t.Fatal("idle cleanup timer survived Keep")
	}
}
