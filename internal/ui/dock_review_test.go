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

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

// Review C1: a click back in the terminal, or closing the focused panel, gives
// typing back to the terminal.
func TestDockTerminalClickTakesKeysBack(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	clickAt(t, win, r, centre(win.gridRect))
	typeKeys(t, win, r, "hi")
	sideFrame(t, win, r)
	if !strings.Contains(ptyOf(st, 0).sent(), "hi") {
		t.Fatalf("typing after a terminal click was lost: %q", ptyOf(st, 0).sent())
	}
}

func TestDockCloseReturnsKeys(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "done")
	floatFrames(t, win, r)
	win.openDock(a.ID, k)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	clickAt(t, win, r, centre(win.dock.hits.close))
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
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	typeKeys(t, win, r, "ls")
	sideFrame(t, win, r)
	if !strings.Contains(ptyOf(st, 0).sent(), "ls") || win.dock.keys {
		t.Fatalf("typing into a focused panel was lost: %q keys %v", ptyOf(st, 0).sent(), win.dock.keys)
	}
}

func TestDockFocusedPanelKeepsShortcuts(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	win.km, _ = buildKeymap(runtime.GOOS, nil)
	win.binds, _ = resolveBindings(runtime.GOOS, nil)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	sc := shortcutFor(t, win, actPalette)
	r.Queue(sc, key.Event{Name: sc.Name, Modifiers: sc.Modifiers, State: key.Release})
	floatFrames(t, win, r)
	if !win.palette.open {
		t.Fatal("the palette shortcut did nothing while the panel had the keyboard")
	}
}

func TestDockKeysLeaveOnTabSwitch(t *testing.T) {
	win, st, r, a, b, _, _ := paneWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	win.openDock(b.ID, startAgent(win, b.ID, "t", "y"))
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.body))
	win.selectTab(b.ID)
	floatFrames(t, win, r)
	typeKeys(t, win, r, "pwd")
	sideFrame(t, win, r)
	if !strings.Contains(ptyOf(st, 1).sent(), "pwd") {
		t.Fatalf("typing after a tab switch went to the panel: %q", ptyOf(st, 1).sent())
	}
}

// Review I2: a finished agent's transcript longer than one read is read to the end.
func TestDockReadsLongFinishedTranscript(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
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
	floatFrames(t, win, r)
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
	win, _, r, a, b, _, _ := paneWindow(t)
	k := startAgent(win, a.ID, "s", "x")
	floatFrames(t, win, r)
	win.openDock(a.ID, k)
	win.selectTab(b.ID)
	floatFrames(t, win, r)
	sideClick(win, r, agentHit(t, win, k).rect, pointer.ButtonPrimary)
	floatFrames(t, win, r)
	if win.mgr.Active().ID != a.ID || win.dockOf(a.ID) == nil {
		t.Fatal("the click closed the panel it should have shown")
	}
}

// Review I4: a panel closing mid-drag does not leave the drag stuck.
func TestDockDragEndsWhenPanelCloses(t *testing.T) {
	win, st, r, a, _, _, _ := paneWindow(t)
	win.openDock(a.ID, startAgent(win, a.ID, "s", "x"))
	floatFrames(t, win, r)
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
	floatFrames(t, win, r)
	_, rows := win.mgr.Size()
	if ptyOf(st, 0).lastSize()[1] >= rows {
		t.Fatal("a reopened panel never resized its terminal")
	}
}
