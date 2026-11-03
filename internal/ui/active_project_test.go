package ui

import (
	"image/color"
	"testing"

	"gioui.org/io/input"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

func TestActiveProjectFollowsSelectedSession(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/alpha/api"))
	b := openIn(t, win, osPath("/code/beta/ui"))
	plain := openIn(t, win, "")
	var r input.Router
	check := func(want string) {
		t.Helper()
		sideFrame(t, win, &r)
		if got := win.side.st.activeRepo; got != want {
			t.Fatalf("active project %q, want %q", got, want)
		}
	}
	win.mgr.SetActive(a.ID)
	check("key-alpha")
	builds := win.side.cache.builds
	win.mgr.SetActive(b.ID)
	check("key-beta")
	if win.side.cache.builds != builds {
		t.Fatal("selection rebuilt the sidebar model")
	}
	win.openSettings()
	check("key-beta")
	win.closeSettings()
	check("key-beta")
	win.side.folded = map[string]bool{"key-beta": true}
	check("key-beta")
	win.side.ed.SetText("alpha")
	check("key-beta")
	if groups := win.side.cache.model.Groups; len(groups) != 1 || groups[0].Key != "key-alpha" {
		t.Fatal("filter fixture did not hide the active group")
	}
	win.side.ed.SetText("")
	win.mgr.SetActive(plain.ID)
	check("")
	win.mgr.CloseAll()
	check("")
}

func TestActiveProjectPersistsForFilteredSiblingAndDocument(t *testing.T) {
	win, _ := sideWindow(t, true)
	a := openIn(t, win, osPath("/code/repo/main"))
	b := openIn(t, win, osPath("/code/repo/worktree"))
	win.names = map[tabs.TabID]string{a.ID: "First", b.ID: "Second"}
	var r input.Router
	sideFrame(t, win, &r)
	win.mgr.SetActive(a.ID)
	win.side.ed.SetText("Second")
	sideFrame(t, win, &r)
	if win.side.st.activeRepo != "key-repo" || len(win.side.cache.model.Groups) != 1 || len(win.side.cache.model.Groups[0].Rows) != 1 {
		t.Fatal("filtered sibling lost project context")
	}
	win.side.ed.SetText("")
	win.openDoc(a.ID, writeDoc(t, t.TempDir(), "notes.md", "# Notes"))
	sideFrame(t, win, &r)
	if win.side.st.activeRepo != "key-repo" {
		t.Fatal("document lost project context")
	}
}

func TestActiveProjectHeaderTextContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		bg, fg := c.activeProjectColors()
		if bg == c.th.Chrome || fg == c.th.ActiveFG {
			t.Errorf("%s active header lacks accent emphasis", th.Key)
		}
		for label, col := range map[string]color.NRGBA{"name": fg, "count": c.activeProjectCountColor(false), "waiting": c.activeProjectCountColor(true)} {
			if got := contrast(col, bg); got < 4.5 {
				t.Errorf("%s active project %s contrast %.2f", th.Key, label, got)
			}
		}
	}
}

func TestActiveProjectKeepsMainSessionContextInPane(t *testing.T) {
	win, _, r, a, _, member, _ := paneWindow(t)
	report(t, member, osPath("/code/beta/server"))
	p, _ := win.fl.open(a.ID, member.ID, paneMember)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(p.hits.body))
	sideFrame(t, win, r)
	if win.fl.keys != member.ID || win.side.st.activeRepo != "key-alpha" {
		t.Fatal("floating pane replaced the main session's project emphasis")
	}
}

func TestActiveProjectChangesWithSessionFolder(t *testing.T) {
	win, _ := sideWindow(t, true)
	tab := openIn(t, win, osPath("/code/alpha/app"))
	var r input.Router
	sideFrame(t, win, &r)
	before := win.side.cache.draws
	report(t, tab, osPath("/code/beta/app"))
	sideFrame(t, win, &r)
	if win.side.st.activeRepo != "key-beta" || win.side.cache.draws <= before {
		t.Fatal("changed session repository left a stale project highlight")
	}
}
