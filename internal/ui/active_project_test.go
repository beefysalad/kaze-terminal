package ui

import (
	"image/color"
	"testing"

	"gioui.org/font"
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

func TestSidebarTextContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		ink := c.sideInk()
		ui := th.UI
		for _, p := range []struct {
			name   string
			fg, bg color.NRGBA
		}{
			{"muted", ink.muted, ui.Chrome}, {"needs", ink.needs, ui.Chrome}, {"agent type", ink.kind, ui.Chrome},
			{"task", ui.Folder, ui.Chrome}, {"chip lead", ink.chipLead, ink.chipBG}, {"chip muted", ink.muted, ink.chipBG},
			{"chip needs", ink.needs, ink.chipBG}, {"tooltip", ink.tipFG, ink.tipBG},
			// The fold control ("N agents ⌄") on a plain and on a selected row; amber when one waits.
			{"fold", ui.Accent, ui.Chrome}, {"fold on active", ui.Accent, ui.Active},
			{"fold needs", ink.needs, ui.Chrome}, {"fold needs on active", ink.needs, ui.Active},
			// The open agent's line, highlighted with th.Active.
			{"open task", ui.Folder, ui.Active}, {"open muted", ink.muted, ui.Active},
			{"open needs", ink.needs, ui.Active}, {"open agent type", ink.kind, ui.Active},
		} {
			if got := contrast(p.fg, p.bg); got < 4.5 {
				t.Errorf("%s %s contrast %.2f, want 4.5", th.Key, p.name, got)
			}
		}
	}
}

func TestActiveProjectNameStyle(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		if col, w := c.groupNameStyle(true); col != th.UI.ActiveFG || w != font.Bold {
			t.Errorf("%s active name %v %v, want ActiveFG bold", th.Key, col, w)
		}
		if _, w := c.groupNameStyle(false); w != font.Normal {
			t.Errorf("%s other names weight %v", th.Key, w)
		}
	}
	c := newChrome()
	if shapedNotdef(c, c.fit("日本 alpha"), font.Font{Typeface: chromeFace, Weight: font.Bold}) {
		t.Fatal("a bold project name draws a missing-glyph box")
	}
}

func TestActiveProjectKeepsMainSessionContextInDockedMember(t *testing.T) {
	win, _, r, a, _, member, _ := memberWindow(t)
	report(t, member, osPath("/code/beta/server"))
	win.openDockMember(a.ID, member.ID)
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	sideFrame(t, win, r)
	if !win.dockMemberKeys() || win.side.st.activeRepo != "key-alpha" {
		t.Fatal("a docked member replaced the main session's project emphasis")
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
