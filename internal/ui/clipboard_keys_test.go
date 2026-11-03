package ui

import (
	"runtime"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/input"
)

// On Windows, plain Ctrl+C and Ctrl+V work like Warp: Ctrl+V pastes, Ctrl+C
// copies a selection and otherwise still interrupts. macOS keeps Cmd+C / Cmd+V.
func TestWarpClipboardKeys(t *testing.T) {
	for _, c := range []struct {
		goos   string
		name   key.Name
		mods   key.Modifiers
		hasSel bool
		want   action
	}{
		{"windows", "V", key.ModCtrl, false, actPaste},
		{"windows", "V", key.ModCtrl, true, actPaste},
		{"windows", "C", key.ModCtrl, true, actCopy},
		{"windows", "C", key.ModCtrl, false, actNone},                // interrupts
		{"windows", "C", key.ModCtrl | key.ModShift, true, actNone},  // the keymap's copy
		{"windows", "V", key.ModCtrl | key.ModShift, false, actNone}, // the keymap's paste
		{"windows", "V", key.ModCtrl | key.ModAlt, false, actNone},   // AltGr
		{"darwin", "V", key.ModCtrl, false, actNone},
		{"darwin", "C", key.ModCtrl, true, actNone},
	} {
		if got := warpClipKey(c.goos, c.name, c.mods, c.hasSel); got != c.want {
			t.Errorf("%s %s mods=%v sel=%v: got %v, want %v", c.goos, c.name, c.mods, c.hasSel, got, c.want)
		}
	}
}

// Ctrl+C with a selection copies it and clears it, so the next Ctrl+C
// interrupts. Ctrl+V never sends a raw Ctrl+V to the shell.
func TestCtrlCCopiesThenInterrupts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows keys")
	}
	st := &recStarter{}
	win := testWindow(t, st)
	var clip []string
	win.toClipboard = func(s string) { clip = append(clip, s) }
	tab, p := shellTab(t, win, st, "hello-kaze", func() bool {
		a := win.mgr.Active()
		return a != nil && textShown(a, 9, 'e')()
	})
	r := &pointerRun{t: t, win: win, tab: tab}
	P := pointer.ButtonPrimary
	r.send(pointer.Press, P, 0, 0, 0)
	r.send(pointer.Drag, P, 0, 9, 0)
	r.send(pointer.Release, 0, 0, 9, 0)
	if got := r.selected(); got != "hello-kaze" {
		t.Fatalf("selected %q", got)
	}
	ctrl := func(name key.Name) {
		win.route(layout.Context{}, []event.Event{key.Event{Name: name, Modifiers: key.ModCtrl, State: key.Press}}, testCW, testCH)
	}

	ctrl("C")
	if len(clip) != 1 || clip[0] != "hello-kaze" {
		t.Fatalf("Ctrl+C copied %q, want [hello-kaze]", clip)
	}
	if win.sels[tab.ID].sel.Active {
		t.Fatal("Ctrl+C left the selection")
	}
	ctrl("V")
	time.Sleep(50 * time.Millisecond)
	if got := p.sent(); got != "" {
		t.Fatalf("sent %q to the shell, want nothing", got)
	}

	ctrl("C")
	waitSent(t, p, "\x03")
	if len(clip) != 1 {
		t.Fatalf("Ctrl+C with no selection copied %q", clip)
	}
}

// A user's own binding for Ctrl+V wins over the Warp paste.
func TestKeymapWinsOverWarpKeys(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows keys")
	}
	st := &recStarter{}
	win := testWindow(t, st)
	win.km[input.Shortcut{Name: "V", Mods: key.ModCtrl}] = actNewTab
	before := len(win.mgr.Tabs())
	if _, err := win.mgr.New(); err != nil {
		t.Fatal(err)
	}
	before++
	win.route(layout.Context{}, []event.Event{key.Event{Name: "V", Modifiers: key.ModCtrl, State: key.Press}}, testCW, testCH)
	if got := len(win.mgr.Tabs()); got != before+1 {
		t.Fatalf("tabs %d, want %d: the user's Ctrl+V binding did not run", got, before+1)
	}
}
