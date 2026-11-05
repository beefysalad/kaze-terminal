package ui

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/skills"
)

// keysWindow is a sidebar window whose config.toml (in a temp dir) holds body.
// The reserved keys go back to the defaults when the test ends.
func keysWindow(t *testing.T, body string) *window {
	t.Helper()
	t.Cleanup(func() {
		km, _ := buildKeymap(runtime.GOOS, nil)
		input.SetReserved(runtime.GOOS, km.shortcuts())
	})
	win, _ := sideWindow(t, true)
	tmp := t.TempDir()
	win.cfgPath = filepath.Join(tmp, "config.toml")
	win.claudePath = filepath.Join(tmp, ".claude", "settings.json")
	win.skillsFn = func(skills.Sources) []skills.Skill { return nil }
	if err := os.WriteFile(win.cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return win
}

func TestReloadKeysReplacesKeymapAndBinds(t *testing.T) {
	win := keysWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\n")
	if err := win.reloadKeys(); err != nil {
		t.Fatal(err)
	}
	ca, cs := key.ModCtrl|key.ModAlt, key.ModCtrl|key.ModShift
	if win.km[input.Shortcut{Name: "F", Mods: ca}] != actFind {
		t.Fatal("Ctrl+Alt+F does not find")
	}
	if a, ok := win.km[input.Shortcut{Name: "F", Mods: cs}]; ok {
		t.Fatalf("Ctrl+Shift+F still runs %v", a)
	}
	if win.binds["find"] != (input.Shortcut{Name: "F", Mods: ca}) {
		t.Fatalf("binds find %v", win.binds["find"])
	}
	if win.opt.Config.Keys["find"] != "ctrl+alt+f" {
		t.Fatalf("config keys %v", win.opt.Config.Keys)
	}
	if !input.IsReserved("F", ca, runtime.GOOS) || input.IsReserved("F", cs, runtime.GOOS) {
		t.Fatal("the reserved keys were not rebuilt")
	}
}

func TestReloadKeysLeavesKeymapOnError(t *testing.T) {
	win := keysWindow(t, "[keys\n")
	km, binds := maps.Clone(win.km), maps.Clone(win.binds)
	if err := win.reloadKeys(); err == nil {
		t.Fatal("no error for a broken file")
	}
	if !maps.Equal(win.km, km) || !maps.Equal(win.binds, binds) {
		t.Fatal("a failed reload changed the keymap")
	}
}

func TestReloadKeysTouchesNothingElse(t *testing.T) {
	win := keysWindow(t, "theme = \"washi\"\nfont_size = 20\n[keys]\npalette = \"ctrl+alt+p\"\n")
	theme, size, chrome, pal := win.opt.Config.Theme, win.opt.Config.FontSize, win.ui.th, win.pal.BG
	if err := win.reloadKeys(); err != nil {
		t.Fatal(err)
	}
	if win.opt.Config.Theme != theme || win.opt.Config.FontSize != size || win.ui.th != chrome || win.pal.BG != pal {
		t.Fatalf("theme %q size %v, want %q %v; chrome or palette changed: %v", win.opt.Config.Theme, win.opt.Config.FontSize, theme, size, win.ui.th != chrome || win.pal.BG != pal)
	}
	if !maps.Equal(win.opt.Config.Keys, map[string]string{"palette": "ctrl+alt+p"}) {
		t.Fatalf("config keys %v", win.opt.Config.Keys)
	}
}

func TestApplyKeysReturnsWarnings(t *testing.T) {
	win := keysWindow(t, "")
	warns := win.applyKeys(map[string]string{"find": "ctrl+shift+k"})
	if len(warns) != 1 || win.binds["find"] != defaultBindings(runtime.GOOS)["find"] {
		t.Fatalf("warns %q find %v", warns, win.binds["find"])
	}
}

// After a save, everything that shows keys shows the new ones on the next frame.
func TestReloadRefreshesShownKeys(t *testing.T) {
	win := keysWindow(t, "[keys]\nnew_branch_session = \"ctrl+alt+b\"\npalette = \"ctrl+alt+p\"\nnext_waiting = \"none\"\n")
	openIn(t, win, osPath("/code/alpha/aaa"))
	r := new(gioinput.Router)
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	if win.side.launch.work.shortcut == "" || win.nextWaitingCaps() == nil {
		t.Fatal("the start-up keys are not shown")
	}
	if err := win.reloadKeys(); err != nil {
		t.Fatal(err)
	}
	sideFrame(t, win, r)
	branch := formatShortcut(runtime.GOOS, input.Shortcut{Name: "B", Mods: key.ModCtrl | key.ModAlt})
	pal := formatShortcut(runtime.GOOS, input.Shortcut{Name: "P", Mods: key.ModCtrl | key.ModAlt})
	if got := win.side.launch.work.shortcut; got != branch {
		t.Fatalf("launcher shows %q, want %q", got, branch)
	}
	if got := win.keyText("palette"); got != pal {
		t.Fatalf("title bar key %q, want %q", got, pal)
	}
	if caps := win.nextWaitingCaps(); caps != nil {
		t.Fatalf("next_waiting caps %q, want none", caps)
	}
	win.openPalette()
	waitSkills(t, win, r)
	found := false
	for _, it := range win.palette.items {
		if it.Act == actNewBranchSession {
			found = true
			if it.Detail != branch {
				t.Fatalf("palette shows %q for a new branch session, want %q", it.Detail, branch)
			}
		}
	}
	if !found {
		t.Fatal("no new branch session row in the palette")
	}
}
