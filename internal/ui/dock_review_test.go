package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

// Review C1: a click back in the terminal, or closing the focused panel, gives
// typing back to the terminal.
func TestDockTerminalClickTakesKeysBack(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	clickAt(t, win, r, centre(win.gridRect))
	typeKeys(t, win, r, "hi")
	sideFrame(t, win, r)
	if !strings.Contains(ptyOf(st, 0).sent(), "hi") {
		t.Fatalf("typing after a terminal click was lost: %q", ptyOf(st, 0).sent())
	}
}

func TestDockCloseReturnsKeys(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "done")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	clickAt(t, win, r, centre(win.dock.hits.tabs[0].close)) // the only tab's ×
	typeKeys(t, win, r, "ok")
	sideFrame(t, win, r)
	if win.dockOf(a.ID) != nil || !strings.Contains(ptyOf(st, 0).sent(), "ok") {
		t.Fatalf("typing after × was lost: %q", ptyOf(st, 0).sent())
	}
}

// Review I1: other keys pressed while the panel has the keyboard go to the
// terminal (typing and app shortcuts alike), and a tab switch never hands
// another session's panel the keyboard.
func TestDockFocusedPanelPassesTypingOn(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	typeKeys(t, win, r, "ls")
	sideFrame(t, win, r)
	// Session.Write queues input; a frame need not wait for the PTY writer.
	waitSent(t, ptyOf(st, 0), "ls")
	if !strings.Contains(ptyOf(st, 0).sent(), "ls") || win.dock.keys {
		t.Fatalf("typing into a focused panel was lost: %q keys %v", ptyOf(st, 0).sent(), win.dock.keys)
	}
}

func TestDockFocusedPanelKeepsShortcuts(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	win.km, _ = buildKeymap(runtime.GOOS, nil)
	win.binds, _ = resolveBindings(runtime.GOOS, nil)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	sc := shortcutFor(t, win, actPalette)
	r.Queue(sc, key.Event{Name: sc.Name, Modifiers: sc.Modifiers, State: key.Release})
	twoFrames(t, win, r)
	if !win.palette.open {
		t.Fatal("the palette shortcut did nothing while the panel had the keyboard")
	}
}

func TestDockKeysLeaveOnTabSwitch(t *testing.T) {
	win, st, r, a, b, _, _ := memberWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	win.openDock(b.ID, startAgent(win, b.ID, "t", "y"))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	typeKeys(t, win, r, "pwd")
	sideFrame(t, win, r)
	if !strings.Contains(ptyOf(st, 1).sent(), "pwd") {
		t.Fatalf("typing after a tab switch went to the panel: %q", ptyOf(st, 1).sent())
	}
}

// Review I2: a finished agent's transcript longer than one read is read to the end.
func TestDockReadsLongFinishedTranscript(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl")
	path := subagents.TranscriptPath(main, "x")
	os.MkdirAll(filepath.Dir(path), 0o755)
	var b strings.Builder
	for b.Len() < 600<<10 {
		b.WriteString(fixtureAssistant(strings.Repeat("y", 1000)))
	}
	b.WriteString(fixtureAssistant("the last note"))
	os.WriteFile(path, []byte(b.String()), 0o644)
	k := dockAgent(t, win, int(a.ID), main)
	stopAgent(win, int(a.ID), "")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	s, ag := win.dockOf(a.ID), win.subs.Get(k)
	now := ag.Ended.Add(time.Minute)
	for i := 0; i < 10; i++ {
		at, wake := win.dockTick(s, ag, now)
		if !wake {
			break
		}
		now = at
	}
	n := len(s.tr.Messages)
	if n == 0 || s.tr.Messages[n-1].Text != "the last note" {
		t.Fatal("the end of a long finished transcript was never read")
	}
}

// Review I3: clicking an agent whose panel is open in another session shows it.
func TestSidebarAgentFromOtherTabShowsPanel(t *testing.T) {
	win, _, r, a, b, _, _ := memberWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	win.selectTab(b.ID)
	twoFrames(t, win, r)
	sideClick(win, r, agentHit(t, win, k).rect, pointer.ButtonPrimary)
	twoFrames(t, win, r)
	if win.mgr.Active().ID != a.ID || win.dockOf(a.ID) == nil {
		t.Fatal("the click closed the panel it should have shown")
	}
}

// Review I4: a panel closing mid-drag does not leave the drag stuck.
func TestDockDragEndsWhenPanelCloses(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	twoFrames(t, win, r)
	p := centre(win.dock.hits.handle)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p)})
	sideFrame(t, win, r)
	win.closeDock(a.ID)
	sideFrame(t, win, r)
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at(p)})
	sideFrame(t, win, r)
	if win.dock.drag.on {
		t.Fatal("drag still on after its panel closed")
	}
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	twoFrames(t, win, r)
	_, rows := win.mgr.Size()
	if ptyOf(st, 0).lastSize()[1] >= rows {
		t.Fatal("a reopened panel never resized its terminal")
	}
}

// PR 121 review: a finished agent whose transcript never opens does not wake
// the window every poll forever.
func TestDockIdleWhenTranscriptMissing(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl") // its subagent transcript is never written
	k := dockAgent(t, win, int(a.ID), main)
	stopAgent(win, int(a.ID), "ok")
	twoFrames(t, win, r)
	win.openDock(a.ID, k)
	s, ag := win.dockOf(a.ID), win.subs.Get(k)
	if _, wake := win.dockTick(s, ag, ag.Ended.Add(time.Minute)); wake {
		t.Fatal("a finished agent with no transcript keeps polling")
	}
}

// PR 121 review: the panel's keyboard goes back to the terminal when its
// agent is dropped while the reader covers the panel.
func TestDockKeysReturnWhenAgentDroppedUnderReader(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	if !win.dock.keys {
		t.Fatal("click did not give the panel the keyboard")
	}
	stopAgent(win, int(a.ID), "ok")
	d := win.openReportDoc(a.ID, "Explore · report", "# Report\n")
	twoFrames(t, win, r)
	win.subs.ClearDone(int(a.ID))
	twoFrames(t, win, r)
	if win.dockOf(a.ID) != nil {
		t.Fatal("panel of a dropped agent still open")
	}
	win.closeDoc(d.id)
	twoFrames(t, win, r)
	before := ptyOf(st, 0).sent()
	typeKeys(t, win, r, "z")
	sideFrame(t, win, r)
	waitSent(t, ptyOf(st, 0), before+"z")
	// A panel opened later does not inherit the stale keyboard: Esc is the
	// terminal's, not a close.
	y := startAgent(win, a.ID, "s", "y")
	win.openDock(a.ID, y)
	twoFrames(t, win, r)
	if win.dock.keys || win.focusTarget() == &win.dock.body {
		t.Fatal("a new panel inherited the dropped panel's keyboard")
	}
	dockKey(t, win, r, key.NameEscape, 0)
	if win.dockOf(a.ID) == nil {
		t.Fatal("Esc closed a panel that never had the keyboard")
	}
	waitSent(t, ptyOf(st, 0), before+"z")
}

// PR 121 review: typing that follows the palette shortcut in the same frame
// reaches the palette, not the session, when the panel has the keyboard.
func TestDockPaletteOpeningAndTypingSameFrame(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	if !win.dock.keys {
		t.Fatal("click did not give the panel the keyboard")
	}
	before := ptyOf(st, 0).sent()
	sc := shortcutFor(t, win, actPalette)
	r.Queue(key.Event{Name: sc.Name, Modifiers: sc.Modifiers, State: key.Press}, key.Event{Name: sc.Name, Modifiers: sc.Modifiers, State: key.Release}, key.EditEvent{Range: key.Range{}, Text: "ship"})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	if got := ptyOf(st, 0).sent(); got != before {
		t.Fatalf("palette input leaked into terminal: %q", strings.TrimPrefix(got, before))
	}
	if got := win.palette.ed.Text(); got != "ship" {
		t.Fatalf("query %q, want ship", got)
	}
}

// PR 121 review: IME composition passes through the focused panel, so
// preedit text is not sent to the shell before it is committed.
func TestDockPassesComposition(t *testing.T) {
	win, st, r, a, _, _, _ := memberWindow(t)
	win.openDock(a.ID, dockAgent(t, win, int(a.ID), ""))
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	if !win.dock.keys {
		t.Fatal("click did not give the panel the keyboard")
	}
	before := ptyOf(st, 0).sent()
	r.Queue(key.CompositionEvent{Start: 0, End: 1}, key.EditEvent{Range: key.Range{}, Text: "に"})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	if got := ptyOf(st, 0).sent(); got != before {
		t.Fatalf("preedit reached the shell: %q", strings.TrimPrefix(got, before))
	}
}
