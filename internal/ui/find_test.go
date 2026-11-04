package ui

import (
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

func TestFindDefaultShortcut(t *testing.T) {
	for _, c := range []struct {
		goos string
		sc   input.Shortcut
	}{
		{"windows", input.Shortcut{Name: "F", Mods: key.ModCtrl | key.ModShift}},
		{"darwin", input.Shortcut{Name: "F", Mods: key.ModCommand}},
	} {
		km, warns := buildKeymap(c.goos, nil)
		if len(warns) != 0 || km[c.sc] != actFind {
			t.Errorf("%s: %+v maps to %v (warns %v), want actFind", c.goos, c.sc, km[c.sc], warns)
		}
	}
}

func TestPlainCtrlFGoesToShell(t *testing.T) {
	km, _ := buildKeymap("windows", nil)
	out := routeKey("windows", km, "F", key.ModCtrl, term.Modes{})
	if out.act != actNone || string(out.data) != "\x06" {
		t.Fatalf("Ctrl+F: act %v data %q, want the shell's ^F", out.act, out.data)
	}
}

func TestFindShortcutOverride(t *testing.T) {
	cs := key.ModCtrl | key.ModShift
	km, warns := buildKeymap("windows", map[string]string{"find": "ctrl+shift+g"})
	if len(warns) != 0 || km[input.Shortcut{Name: "G", Mods: cs}] != actFind {
		t.Fatalf("override: warns %v, Ctrl+Shift+G = %v", warns, km[input.Shortcut{Name: "G", Mods: cs}])
	}
	if a, ok := km[input.Shortcut{Name: "F", Mods: cs}]; ok {
		t.Fatalf("Ctrl+Shift+F still bound to %v after the override", a)
	}
	km, warns = buildKeymap("windows", map[string]string{"find": "ctrl+shift+t"})
	if len(warns) != 1 || !strings.Contains(warns[0], "new_tab") {
		t.Fatalf("clash: warns %v, want one naming new_tab", warns)
	}
	if km[input.Shortcut{Name: "F", Mods: cs}] != actFind || km[input.Shortcut{Name: "T", Mods: cs}] != actNewTab {
		t.Fatal("a clashing override did not keep both defaults")
	}
}

func TestFindListedInShortcuts(t *testing.T) {
	win, _ := alertWindow(t, 1)
	text := commandsPage(t, win)
	if !strings.Contains(text, "Find in the terminal") || !hasKeys(win, "Ctrl", "Shift", "F") {
		t.Fatalf("Shortcuts page lacks find:\n%s", text)
	}
	for _, g := range shortcutGroups {
		if g.title == "Editing and mouse" && strings.Join(g.configs, ",") != "copy,paste,find" {
			t.Fatalf("Editing and mouse lists %v", g.configs)
		}
	}
}
