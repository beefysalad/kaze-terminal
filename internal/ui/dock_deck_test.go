package ui

import (
	"fmt"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

// Tests for the panel's deck of tabs (issue #133, plan task D1).

func TestDeckOpensTabsAndSwitches(t *testing.T) {
	win, _, _, a, _, _, _ := memberWindow(t)
	k1 := subagents.Key{Session: "s", ID: "k1"}
	k2 := subagents.Key{Session: "s", ID: "k2"}
	win.openDock(a.ID, k1)
	win.openDock(a.ID, k2)
	d := win.deckOf(a.ID)
	if d == nil || len(d.entries) != 2 || d.on != 1 || win.dockOf(a.ID).agent != k2 {
		t.Fatalf("deck %+v", d)
	}
	win.openDock(a.ID, k1)
	if d.on != 0 || len(d.entries) != 2 || win.dockOf(a.ID).agent != k1 {
		t.Fatalf("re-open: on %d, %d entries", d.on, len(d.entries))
	}
}

func TestDeckToggleHidesKeepsTabs(t *testing.T) {
	win, _, _, a, _, _, _ := memberWindow(t)
	k1 := subagents.Key{Session: "s", ID: "k1"}
	k2 := subagents.Key{Session: "s", ID: "k2"}
	win.openDock(a.ID, k1)
	win.openDock(a.ID, k2)
	win.toggleDock(a.ID, k2)
	d := win.deckOf(a.ID)
	if win.dockOf(a.ID) != nil || d == nil || !d.hidden || len(d.entries) != 2 {
		t.Fatalf("toggle did not hide and keep: %+v", d)
	}
	win.openDock(a.ID, k1)
	if s := win.dockOf(a.ID); s == nil || s.agent != k1 || d.hidden {
		t.Fatalf("open did not show again: %+v", d)
	}
}

func TestDeckCapReplacesOldestNotShown(t *testing.T) {
	win, _, _, a, _, _, _ := memberWindow(t)
	ks := make([]subagents.Key, 7)
	for i := range ks {
		ks[i] = subagents.Key{Session: "s", ID: fmt.Sprintf("k%d", i+1)}
	}
	for _, k := range ks[:6] {
		win.openDock(a.ID, k)
	}
	win.openDock(a.ID, ks[0]) // the 1st shown again
	win.openDock(a.ID, ks[6])
	d := win.deckOf(a.ID)
	if len(d.entries) != dockMaxTabs {
		t.Fatalf("%d entries, want %d", len(d.entries), dockMaxTabs)
	}
	for _, e := range d.entries {
		if e.view.agent == ks[1] {
			t.Fatal("the 2nd tab should be the one replaced")
		}
	}
	if win.dockOf(a.ID).agent != ks[6] {
		t.Fatal("the new tab is not shown")
	}
}

func TestDeckCloseTabMovesLeft(t *testing.T) {
	win, _, _, a, _, _, _ := memberWindow(t)
	ks := []subagents.Key{{Session: "s", ID: "1"}, {Session: "s", ID: "2"}, {Session: "s", ID: "3"}}
	for _, k := range ks {
		win.openDock(a.ID, k)
	}
	win.openDock(a.ID, ks[1])
	win.closeDockTab(a.ID, 1)
	if s := win.dockOf(a.ID); s == nil || s.agent != ks[0] {
		t.Fatalf("closing the middle tab did not show the left one: %+v", s)
	}
	win.closeDockTab(a.ID, 0)
	if s := win.dockOf(a.ID); s == nil || s.agent != ks[2] {
		t.Fatalf("closing the first tab did not show its right neighbour: %+v", s)
	}
	win.closeDockTab(a.ID, 0)
	if win.deckOf(a.ID) != nil {
		t.Fatal("closing the last tab kept the deck")
	}
}

func TestDeckDropsGoneAgentOnly(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k1 := startAgent(win, a.ID, "s", "x")
	k2 := startAgent(win, a.ID, "s", "y")
	win.openDock(a.ID, k1)
	win.openDock(a.ID, k2)
	twoFrames(t, win, r)
	stopAgent(win, int(a.ID), "done") // k1 is "x"
	twoFrames(t, win, r)
	if !win.subs.Dismiss(k1) {
		t.Fatal("k1 not dismissed")
	}
	twoFrames(t, win, r)
	d := win.deckOf(a.ID)
	if d == nil || len(d.entries) != 1 || d.entries[0].view.agent != k2 || win.dockOf(a.ID).agent != k2 {
		t.Fatalf("deck after dismissing k1: %+v", d)
	}
}
