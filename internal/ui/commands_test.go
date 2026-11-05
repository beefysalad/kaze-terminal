package ui

import (
	"image"
	"path/filepath"
	"runtime"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/skills"
)

func TestResolveBindingsKeepsOverrides(t *testing.T) {
	b, warns := resolveBindings("windows", map[string]string{"palette": "ctrl+shift+p"})
	if len(warns) != 0 || b["palette"] != (input.Shortcut{Name: "P", Mods: key.ModCtrl | key.ModShift}) {
		t.Fatalf("binds %+v, warns %v", b["palette"], warns)
	}
	b, warns = resolveBindings("windows", map[string]string{"palette": "ctrl+nope+p"})
	if len(warns) != 1 || b["palette"] != defaultBindings("windows")["palette"] {
		t.Fatalf("bad text: binds %+v, warns %v", b["palette"], warns)
	}
}

func TestFormatShortcut(t *testing.T) {
	cs := key.ModCtrl | key.ModShift
	cases := []struct {
		goos string
		sc   input.Shortcut
		want string
	}{
		{"windows", input.Shortcut{Name: "K", Mods: cs}, "Ctrl+Shift+K"},
		{"darwin", input.Shortcut{Name: "K", Mods: key.ModCommand}, "Cmd+K"},
		{"windows", input.Shortcut{Name: key.NameTab, Mods: key.ModCtrl}, "Ctrl+Tab"},
		{"darwin", input.Shortcut{Name: "`", Mods: key.ModCtrl}, "Ctrl+`"},
		{"windows", input.Shortcut{Name: "T", Mods: cs}, "Ctrl+Shift+T"},
		{"darwin", input.Shortcut{Name: ",", Mods: key.ModCommand}, "Cmd+,"},
		{"windows", input.Shortcut{Name: "X", Mods: key.ModSuper}, "Win+X"},
		{"darwin", input.Shortcut{Name: "A", Mods: key.ModAlt}, "Option+A"},
		{"windows", input.Shortcut{Name: "A", Mods: key.ModAlt}, "Alt+A"},
		{"darwin", input.Shortcut{Name: key.NameTab, Mods: cs}, "Ctrl+Shift+Tab"},
		{"windows", input.Shortcut{Name: key.NameReturn, Mods: key.ModCtrl}, "Ctrl+Enter"},
		{"windows", input.Shortcut{Name: key.NameEscape}, "Esc"},
		{"windows", input.Shortcut{Name: key.NameUpArrow, Mods: key.ModShift}, "Shift+Up"},
		{"windows", input.Shortcut{Name: key.NamePageUp}, "PageUp"},
		{"windows", input.Shortcut{Name: key.NameF5}, "F5"},
		{"darwin", input.Shortcut{Name: "K", Mods: key.ModCommand | key.ModShift | key.ModCtrl | key.ModAlt}, "Ctrl+Option+Shift+Cmd+K"},
	}
	for _, c := range cases {
		if got := formatShortcut(c.goos, c.sc); got != c.want {
			t.Errorf("%s %+v: got %q, want %q", c.goos, c.sc, got, c.want)
		}
	}
}

func TestEveryKnownActionIsListed(t *testing.T) {
	seen := map[string]int{}
	for _, c := range kazeCommands {
		seen[c.Config]++
		if c.Act != actionByName[c.Config] || c.Label == "" {
			t.Errorf("%+v does not match actionByName", c)
		}
	}
	for name := range config.KnownActions {
		if seen[name] != 1 {
			t.Errorf("%s listed %d times", name, seen[name])
		}
	}
	if len(kazeCommands) != len(config.KnownActions) {
		t.Errorf("%d commands for %d actions", len(kazeCommands), len(config.KnownActions))
	}
	if kazeCommands[0].Config != "new_tab" || kazeCommands[len(kazeCommands)-1].Config != "palette" {
		t.Errorf("order: first %s, last %s", kazeCommands[0].Config, kazeCommands[len(kazeCommands)-1].Config)
	}
}

func TestKeyTextUnbound(t *testing.T) {
	win, _ := alertWindow(t, 1)
	win.binds, _ = resolveBindings(runtime.GOOS, nil)
	if got, want := win.keyText("palette"), formatShortcut(runtime.GOOS, win.binds["palette"]); got != want || got == "" {
		t.Fatalf("bound: %q, want %q", got, want)
	}
	win.binds = map[string]input.Shortcut{}
	if got := win.keyText("palette"); got != "" {
		t.Fatalf("unbound: %q, want empty", got)
	}
}

func TestTitleWithoutPaletteKey(t *testing.T) {
	for _, w := range []int{320, 480, 600, 1200} {
		in := titleIn{Size: image.Pt(w, 42), SideW: 248, Crumb: fullCrumb, Key: "", Dims: titleTestDims, Measure: sevenPx}
		l := layoutTitle(in)
		if l.SearchKey || (l.SearchIconOnly && !l.Search.Empty() && l.Search.Dx() != titleTestDims.Btn) {
			t.Fatalf("%dpx: no palette key, but the search keeps room for one: %+v", w, l)
		}
		if searchKeyShown(l, "") {
			t.Fatalf("%dpx: a key cap is drawn for no key", w)
		}
	}
}

// Every place that shows an action's keys copes with no bindings at all.
func TestEveryReaderWithNoBindings(t *testing.T) {
	win, _ := sideWindow(t, true)
	tmp := t.TempDir()
	win.cfgPath = filepath.Join(tmp, "kaze", "config.toml")
	win.claudePath = filepath.Join(tmp, ".claude", "settings.json")
	win.skillsFn = func(skills.Sources) []skills.Skill { return nil }
	r := new(gioinput.Router)

	// The empty window, first with its shortcut line, then without.
	sideFrame(t, win, r)
	sideFrame(t, win, r)
	withKey := win.empty.button.Min.Y
	win.initBranchShortcut()
	if win.empty.shortcut == "" || win.side.launch.work.shortcut == "" {
		t.Fatalf("default keys not shown: empty %q launcher %q", win.empty.shortcut, win.side.launch.work.shortcut)
	}
	win.binds, win.km = map[string]input.Shortcut{}, keymap{}
	win.side.launch.work.shortcutReady = false
	sideFrame(t, win, r)
	if win.empty.shortcut != "" {
		t.Fatalf("empty window shortcut %q", win.empty.shortcut)
	}
	if got := win.empty.button.Min.Y; got <= withKey {
		t.Fatalf("the block still leaves room for a shortcut line: button at %d, %d with one", got, withKey)
	}
	if win.side.launch.work.shortcut != "" {
		t.Fatalf("launcher shortcut %q", win.side.launch.work.shortcut)
	}

	// A session: the title bar, the alert card, the palette.
	tabA := openIn(t, win, osPath("/code/alpha/aaa"))
	openIn(t, win, osPath("/code/alpha/bbb"))
	win.mgr.SetActive(tabA.ID)
	sideFrame(t, win, r)
	if key := win.keyText("palette"); key != "" {
		t.Fatalf("palette key %q", key)
	}
	b := win.mgr.Tabs()[1].ID
	win.alerts.apply(notify.Msg{Tab: int(b), Kind: notify.KindNeeds}, tabA.ID, true, only(3), t0)
	win.drawToasts(barContext(1000), toastArea(1000, 600, 24))
	sideFrame(t, win, r)
	if caps := win.nextWaitingCaps(); caps != nil {
		t.Fatalf("next_waiting caps %q", caps)
	}
	win.openPalette()
	waitSkills(t, win, r)
	for _, a := range win.palInput().Actions {
		if len(a.Keys) != 0 {
			t.Fatalf("palette action %s has keys %q", a.Config, a.Keys)
		}
	}
	for _, it := range win.palette.items {
		if it.Kind == palKaze && it.Workspace == "" && len(it.Keys) != 0 {
			t.Fatalf("palette row %s shows %q", it.Name, it.Keys)
		}
	}
	win.closePalette()
	sideFrame(t, win, r)

	// Settings > Shortcuts: "None" once per editable action.
	commandsPage(t, win)
	n := 0
	for _, s := range win.set.texts {
		if s == "None" {
			n++
		}
	}
	if want := len(defaultBindings(runtime.GOOS)); n != want {
		t.Fatalf("%d None rows, want %d: %q", n, want, win.set.texts)
	}
}
