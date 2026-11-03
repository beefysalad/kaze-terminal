package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"github.com/patrick-gogo/kaze-terminal/internal/config"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

func TestFinalSessionCloseKeepsWindowUsable(t *testing.T) {
	for _, sidebar := range []bool{false, true} {
		t.Run(map[bool]string{false: "shortcut", true: "sidebar"}[sidebar], func(t *testing.T) {
			win, _ := sideWindow(t, true)
			win.wantFocus = true
			tab := openIn(t, win, "")
			var r input.Router
			sideFrame(t, win, &r)
			sideFrame(t, win, &r)
			if sidebar {
				r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: sidePt(win, centre(rowHit(t, win, tab.ID).rect))})
				sideFrame(t, win, &r)
				for _, row := range win.side.hits.rows {
					if row.id == tab.ID {
						sideClick(win, &r, row.close, pointer.ButtonPrimary)
						sideFrame(t, win, &r)
						break
					}
				}
			} else {
				r.Queue(shortcutFor(t, win, actCloseTab))
				sideFrame(t, win, &r)
			}
			if !win.mgr.Empty() {
				t.Fatal("last session did not close")
			}
			if win.shouldClose() {
				t.Fatal("empty window requested shutdown")
			}
			sideFrame(t, win, &r)
			r.Queue(shortcutFor(t, win, actNewTab))
			sideFrame(t, win, &r)
			if win.mgr.Active() == nil {
				t.Fatal("new-session shortcut did not work without sessions")
			}
		})
	}
}

func TestEmptyWindowSettingsAndConfiguredShortcut(t *testing.T) {
	win, _ := sideWindow(t, true)
	win.km, _ = buildKeymap("windows", map[string]string{"new_tab": "ctrl+n"})
	win.binds, _ = resolveBindings("windows", map[string]string{"new_tab": "ctrl+n"})
	var r input.Router
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	r.Queue(shortcutFor(t, win, actOpenSettings))
	sideFrame(t, win, &r)
	if !win.set.shown || len(win.set.controls) == 0 {
		t.Fatal("Settings is not usable without sessions")
	}
	sideFrame(t, win, &r)
	r.Queue(shortcutFor(t, win, actCloseTab))
	sideFrame(t, win, &r)
	if win.set.shown || !win.mgr.Empty() {
		t.Fatalf("closing Settings: shown=%v empty=%v focusedSettings=%v focusedEmpty=%v filtering=%v", win.set.shown, win.mgr.Empty(), r.Source().Focused(&win.set.tag), r.Source().Focused(&win.empty.newSession), win.side.filtering)
	}
	sideFrame(t, win, &r)
	r.Queue(key.Event{Name: "N", Modifiers: key.ModCtrl, State: key.Press})
	sideFrame(t, win, &r)
	if win.mgr.Active() == nil {
		t.Fatal("configured new-session shortcut did not work")
	}
}

func TestFinalShellExitLeavesEmptyWindow(t *testing.T) {
	win, st := sideWindow(t, true)
	tab := openIn(t, win, "")
	var r input.Router
	sideFrame(t, win, &r)
	ptyOf(st, 0).Close()
	deadline := time.Now().Add(time.Second)
	for {
		exited, _ := tab.Session.Exited()
		if exited {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shell did not exit")
		}
		time.Sleep(time.Millisecond)
	}
	sideFrame(t, win, &r)
	if !win.mgr.Empty() || win.shouldClose() {
		t.Fatal("final shell exit did not leave a usable empty window")
	}
}

func TestEmptyWindowLaunchFailureCanRetry(t *testing.T) {
	win, st := sideWindow(t, true)
	st.fail = errors.New("shell unavailable")
	win.runShortcut(layout.Context{}, actNewTab)
	var r input.Router
	sideFrame(t, win, &r)
	if !strings.Contains(win.stat.m.Notice, "shell unavailable") {
		t.Fatal("launch error is not visible in empty window status bar")
	}
	st.fail = nil
	sideFrame(t, win, &r)
	r.Queue(shortcutFor(t, win, actNewTab))
	sideFrame(t, win, &r)
	if win.mgr.Empty() {
		t.Fatal("could not retry opening a session")
	}
}

func TestEmptyWindowDoesNotBlink(t *testing.T) {
	win, _ := sideWindow(t, true)
	win.opt.Config.CursorBlink = true
	if _, ok := win.blink(time.Now()); ok {
		t.Fatal("empty window schedules cursor blink frames")
	}
}

func TestEmptyWindowNewSessionButtonAndKeyboard(t *testing.T) {
	for _, keyboard := range []bool{false, true} {
		t.Run(map[bool]string{false: "click", true: "keyboard"}[keyboard], func(t *testing.T) {
			win, _ := sideWindow(t, true)
			var r input.Router
			sideFrame(t, win, &r)
			sideFrame(t, win, &r)
			if win.empty.button.Empty() {
				t.Fatal("no New session button")
			}
			if keyboard {
				r.Queue(key.Event{Name: key.NameReturn, State: key.Press}, key.Event{Name: key.NameReturn, State: key.Release})
				sideFrame(t, win, &r)
			} else {
				clickAt(t, win, &r, centre(win.empty.button))
			}
			if win.mgr.Active() == nil || win.shouldClose() {
				t.Fatal("empty-state action did not open a session")
			}
			sideFrame(t, win, &r)
			if !r.Source().Focused(win.termTag) {
				t.Fatal("new terminal did not receive keyboard focus")
			}
		})
	}
}

func TestEmptyWindowTitleBarActions(t *testing.T) {
	win, _ := sideWindow(t, true)
	var r input.Router
	sideFrame(t, win, &r)
	sideFrame(t, win, &r)
	clickAt(t, win, &r, centre(win.bar.Plus))
	if win.mgr.Active() == nil {
		t.Fatal("title-bar New session did not work")
	}
	win.closeTab(win.mgr.Active().ID)
	sideFrame(t, win, &r)
	var action system.Action
	win.performFn = func(a system.Action) { action = a }
	clickAt(t, win, &r, centre(win.bar.Buttons[2]))
	if action != system.ActionClose {
		t.Fatal("explicit window close did not request ActionClose")
	}
}

func TestEmptyWindowWorkspaceLaunch(t *testing.T) {
	f := newRunFixture(t)
	win := f.win
	win.opt.Config = config.Default()
	win.custom = true
	win.termTag, win.capTag, win.toastTag = new(int), new(int), new(int)
	win.binds, _ = resolveBindings("windows", nil)
	var r input.Router
	frame := func() { sizedFrame(t, win, &r, 1000, 900) }
	frame()
	frame()
	r.Queue(shortcutFor(t, win, actOpenSettings))
	frame()
	frame()
	win.selectSection(wsIndex(t))
	frame()
	buttons := controlsOf(win, "ws.run")
	if len(buttons) != 1 {
		t.Fatalf("%d workspace Run controls without sessions", len(buttons))
	}
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(centre(buttons[0].rect))})
	frame()
	if len(win.mgr.Tabs()) != len(samuraiTabs) || win.set.shown {
		t.Fatal("workspace did not launch from empty window Settings")
	}
}

func TestEmptyWindowPersistsClosedSessions(t *testing.T) {
	f, _ := restoreFixture(t)
	f.win.initRestore()
	t.Cleanup(f.win.closeRestore)
	if err := f.win.openStartup(nil); err != nil {
		t.Fatal(err)
	}
	tab := f.win.mgr.Active()
	if len(loadSaved(t, f).Entries) != 1 {
		t.Fatal("initial session was not saved")
	}
	f.win.closeTab(tab.ID)
	if got := loadSaved(t, f); len(got.Entries) != 0 {
		t.Fatal("closed final session remained in saved state")
	}
}

func TestEmptyWindowTextContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		if got := contrast(c.emptyMuted(), c.pageColors().bg); got < 4.5 {
			t.Errorf("%s empty-state detail contrast %.2f, want at least 4.5", th.Key, got)
		}
	}
}
