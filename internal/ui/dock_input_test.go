package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

func dockKey(t *testing.T, win *window, r *input.Router, name key.Name, mods key.Modifiers) {
	t.Helper()
	r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press}, key.Event{Name: name, Modifiers: mods, State: key.Release})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

func TestDockTabsAndAutoReport(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	win.openDock(a.ID, k)
	floatFrames(t, win, r)
	if win.dockOf(a.ID).tab != dockActivity {
		t.Fatal("running agent not on Activity")
	}
	stopAgent(win, int(a.ID), "done")
	floatFrames(t, win, r)
	if win.dockOf(a.ID).tab != dockReport {
		t.Fatal("finishing did not show the report")
	}
	// A tab the user picked stays.
	win.closeDock(a.ID)
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStart, Session: "s", AgentID: "y", AgentType: "Plan"})
	y := subagents.Key{Session: "s", ID: "y"}
	win.openDock(a.ID, y)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.repTab))
	clickAt(t, win, r, centre(win.dock.hits.actTab))
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStop, Session: "s", AgentID: "y", Report: "r"})
	floatFrames(t, win, r)
	if s := win.dockOf(a.ID); s.tab != dockActivity || !s.picked {
		t.Fatalf("picked Activity was overridden: %+v", s.tab)
	}
	clickAt(t, win, r, centre(win.dock.hits.finish))
	if win.dockOf(a.ID).tab != dockReport {
		t.Fatal("read the report did not switch")
	}
}

func TestDockHandleDragSavesOnce(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	floatFrames(t, win, r)
	before := win.dockRect().Dy()
	p := centre(win.dock.hits.handle)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p)})
	sideFrame(t, win, r)
	for i := 1; i <= 4; i++ {
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p.Add(image.Pt(0, -20*i)))})
		sideFrame(t, win, r)
	}
	if b, _ := os.ReadFile(win.cfgPath); strings.Contains(string(b), "agent_panel_height") {
		t.Fatal("saved during the drag")
	}
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at(p.Add(image.Pt(0, -80)))})
	floatFrames(t, win, r)
	if got := win.dockRect().Dy(); got != before+80 {
		t.Fatalf("height %d, want %d", got, before+80)
	}
	b, _ := os.ReadFile(win.cfgPath)
	if !strings.Contains(string(b), "agent_panel_height") {
		t.Fatalf("not saved: %q", b)
	}
}

func TestDockEscStaysWithTerminal(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	floatFrames(t, win, r)
	dockKey(t, win, r, key.NameEscape, 0)
	if !strings.Contains(ptyOf(st, 0).sent(), "\x1b") {
		t.Fatalf("Esc did not reach the terminal: %q", ptyOf(st, 0).sent())
	}
	if win.dockOf(a.ID) == nil {
		t.Fatal("Esc in the terminal closed the panel")
	}
}

func TestDockEscClosesWhenFocused(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	if !win.dock.keys {
		t.Fatal("click did not give the panel the keyboard")
	}
	sent := ptyOf(st, 0).sent()
	dockKey(t, win, r, key.NameEscape, 0)
	if win.dockOf(a.ID) != nil {
		t.Fatal("Esc did not hide the focused panel")
	}
	if d := win.deckOf(a.ID); d == nil || !d.hidden || len(d.entries) != 1 {
		t.Fatalf("Esc lost the tabs: %+v", d)
	}
	if ptyOf(st, 0).sent() != sent {
		t.Fatal("Esc also reached the terminal")
	}
	if win.dock.keys {
		t.Fatal("keyboard not back with the terminal")
	}
}

func TestDockArrowsStepAgents(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	win.liveMgr.Store(win.mgr)
	for _, id := range []string{"a1", "a2", "a3"} {
		win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindAgentStart, Session: "s", AgentID: id, AgentType: "Explore"})
	}
	floatFrames(t, win, r)
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "a1"})
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	dockKey(t, win, r, key.NameDownArrow, 0)
	if got := win.dockOf(a.ID).agent.ID; got != "a2" {
		t.Fatalf("down went to %q", got)
	}
	dockKey(t, win, r, key.NameUpArrow, 0)
	dockKey(t, win, r, key.NameUpArrow, 0)
	if got := win.dockOf(a.ID).agent.ID; got != "a1" {
		t.Fatalf("up past the first went to %q", got)
	}
}

func TestDockLinkClickOpensDoc(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	dir := t.TempDir()
	report(t, a, dir)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"), 0o644)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "See notes.md now.")
	floatFrames(t, win, r)
	win.openDock(a.ID, k)
	floatFrames(t, win, r)
	if len(win.dock.hits.links) == 0 {
		t.Fatal("no link drawn")
	}
	p := centre(win.dock.hits.links[0].r)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Modifiers: key.ModShortcut, Position: at(p)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Modifiers: key.ModShortcut, Position: at(p)})
	floatFrames(t, win, r)
	if d := win.shownDoc(); d == nil || filepath.Base(d.path) != "notes.md" {
		t.Fatalf("doc not opened: %+v", d)
	}
}
