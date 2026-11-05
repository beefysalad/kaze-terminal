package ui

import (
	"image"
	"image/color"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/skills"
	"github.com/beefysalad/kaze-terminal/internal/theme"
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
	// Clash with the palette's default on this OS (Ctrl+Shift+K on Windows, Cmd+K on macOS).
	pal, ok := shortcutText(defaultBindings(runtime.GOOS)["palette"])
	if !ok {
		t.Fatal("palette default has no text form")
	}
	warns := win.applyKeys(map[string]string{"find": pal})
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
	win.setPaletteQuery("branch") // Kaze actions show once you type (palette redesign)
	win.refreshPalette()
	found := false
	for _, it := range win.palette.items {
		if it.Act == actNewBranchSession {
			found = true
			if want := splitShortcut(branch); !slices.Equal(it.Keys, want) {
				t.Fatalf("palette shows %q for a new branch session, want %q", it.Keys, want)
			}
		}
	}
	if !found {
		t.Fatal("no new branch session row in the palette")
	}
}

// shortcutsWindow is a window with one session, its config.toml (a temp file)
// holding cfg and its keymap loaded from it, and Settings open on Shortcuts,
// laid out once at 1000x900.
func shortcutsWindow(t *testing.T, cfg string) (*window, *gioinput.Router, *recStarter, string) {
	t.Helper()
	t.Cleanup(func() {
		km, _ := buildKeymap(runtime.GOOS, nil)
		input.SetReserved(runtime.GOOS, km.shortcuts())
	})
	win, st := sideWindow(t, true)
	if _, err := win.mgr.New(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	win.cfgPath = path
	win.applyKeys(nil)
	win.reloadKeys() // a broken file keeps the defaults
	win.openSettings()
	win.selectSection(slices.IndexFunc(win.sections(), func(s settingsSection) bool { return s.title == "Shortcuts" }))
	r := new(gioinput.Router)
	sizedFrame(t, win, r, 1000, 900)
	return win, r, st, path
}

// keysFrames runs the two frames a queued batch needs, at the page's size.
func keysFrames(t *testing.T, win *window, r *gioinput.Router) {
	t.Helper()
	sizedFrame(t, win, r, 1000, 900)
	sizedFrame(t, win, r, 1000, 900)
}

// keysControl is control id for action (its arg is the editableActions
// index), scrolled into the pane first.
func keysControl(t *testing.T, win *window, r *gioinput.Router, id, action string) control {
	t.Helper()
	arg := slices.Index(editableActions(runtime.GOOS), action)
	if arg < 0 {
		t.Fatalf("%s is not editable", action)
	}
	for range 20 {
		c := controlsOf(win, id)
		k := slices.IndexFunc(c, func(c control) bool { return c.arg == arg })
		if k < 0 {
			t.Fatalf("no %s control for %s in %+v", id, action, win.set.controls)
		}
		pane := win.set.pane
		switch rect := c[k].rect; {
		case rect.Max.Y > pane.Max.Y:
			win.set.scroll += rect.Max.Y - pane.Max.Y + 8
		case rect.Min.Y < pane.Min.Y:
			win.set.scroll -= pane.Min.Y - rect.Min.Y + 8
		default:
			return c[k]
		}
		sizedFrame(t, win, r, 1000, 900)
	}
	t.Fatalf("%s for %s never scrolled into view", id, action)
	return control{}
}

// clickRect presses and releases the primary button in the middle of rect.
func clickRect(t *testing.T, win *window, r *gioinput.Router, rect image.Rectangle) {
	t.Helper()
	p := at(centre(rect))
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p},
	)
	keysFrames(t, win, r)
}

// clickKeys clicks action's shortcut button.
func clickKeys(t *testing.T, win *window, r *gioinput.Router, action string) {
	t.Helper()
	clickRect(t, win, r, keysControl(t, win, r, "key_rec", action).rect)
}

// pressIn queues a press and a release of name with mods, then two frames.
func pressIn(t *testing.T, win *window, r *gioinput.Router, name key.Name, mods key.Modifiers) {
	t.Helper()
	r.Queue(
		key.Event{Name: name, Modifiers: mods, State: key.Press},
		key.Event{Name: name, Modifiers: mods, State: key.Release},
	)
	keysFrames(t, win, r)
}

// fileText is the file at path, "" when there is none.
func fileText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func pageText(win *window) string { return strings.Join(win.set.texts, "\n") }

// flatText is the page's texts joined by single spaces, so a wrapped line reads whole.
func flatText(win *window) string {
	return strings.Join(strings.Fields(strings.Join(win.set.texts, " ")), " ")
}

func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestClickStartsRecording(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	if win.set.keyRec != "find" {
		t.Fatalf("recording %q, want find", win.set.keyRec)
	}
	if !strings.Contains(pageText(win), recordingText) {
		t.Fatalf("no recording text in:\n%s", pageText(win))
	}
}

func TestEnterStartsRecording(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	want := keysControl(t, win, r, "key_rec", "find")
	focused := func() bool {
		f := win.set.focus
		return f >= 0 && f < len(win.set.controls) && win.set.controls[f].id == "key_rec" && win.set.controls[f].arg == want.arg
	}
	for i := 0; i < 60 && !focused(); i++ {
		pressIn(t, win, r, key.NameTab, 0)
	}
	if !focused() {
		t.Fatal("Tab never reached find's shortcut")
	}
	pressIn(t, win, r, key.NameReturn, 0)
	if win.set.keyRec != "find" {
		t.Fatalf("recording %q after Enter, want find", win.set.keyRec)
	}
	if got := fileText(t, path); got != "" {
		t.Fatalf("Enter was saved: %q", got)
	}
}

func TestEscCancels(t *testing.T) {
	for _, mods := range []key.Modifiers{0, key.ModShift} {
		win, r, _, path := shortcutsWindow(t, "")
		km := maps.Clone(win.km)
		clickKeys(t, win, r, "find")
		pressIn(t, win, r, key.NameEscape, mods)
		if win.set.keyRec != "" {
			t.Fatalf("%v+Esc: still recording %q", mods, win.set.keyRec)
		}
		if fileText(t, path) != "" || !maps.Equal(win.km, km) {
			t.Fatalf("%v+Esc changed the file or the keymap", mods)
		}
	}
}

func TestClickElsewhereCancels(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	clickRect(t, win, r, controlsOf(win, "cmd_filter")[0].rect)
	if win.set.keyRec != "" {
		t.Fatalf("a click on the search field left %q recording", win.set.keyRec)
	}
	clickKeys(t, win, r, "find")
	clickKeys(t, win, r, "palette")
	if win.set.keyRec != "palette" {
		t.Fatalf("recording %q after clicking palette's keys", win.set.keyRec)
	}
}

func TestLeavingTheSectionCancels(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	shortcuts := win.set.section
	clickKeys(t, win, r, "find")
	win.selectSection(0)
	if win.set.keyRec != "" {
		t.Fatal("still recording after another section was chosen")
	}
	win.selectSection(shortcuts)
	sizedFrame(t, win, r, 1000, 900)
	clickKeys(t, win, r, "find")
	win.hideSettings()
	if win.set.keyRec != "" {
		t.Fatal("still recording after Settings was hidden")
	}
	win.openSettings()
	keysFrames(t, win, r)
	clickKeys(t, win, r, "find")
	win.setFocused(false)
	if win.set.keyRec != "" {
		t.Fatal("still recording after the window lost focus")
	}
}

func TestIgnoredPressesKeepRecording(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	for _, p := range []struct {
		name key.Name
		mods key.Modifiers
	}{
		{key.NameCtrl, key.ModCtrl}, {key.NameShift, key.ModShift}, {"+", key.ModCtrl},
		{key.NameEnter, 0}, {"K", key.ModCtrl | key.ModSuper},
	} {
		pressIn(t, win, r, p.name, p.mods)
		if win.set.keyRec != "find" || fileText(t, path) != "" {
			t.Fatalf("%v+%q: recording %q, file %q", p.mods, p.name, win.set.keyRec, fileText(t, path))
		}
	}
	r.Queue(key.EditEvent{Text: "€"})
	keysFrames(t, win, r)
	if win.set.keyRec != "find" || fileText(t, path) != "" {
		t.Fatalf("typed text: recording %q, file %q", win.set.keyRec, fileText(t, path))
	}
}

func TestPlainPressSaves(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	ca := key.ModCtrl | key.ModAlt
	pressIn(t, win, r, "F", ca)
	if got := fileText(t, path); !strings.Contains(got, "find = \"ctrl+alt+f\"") {
		t.Fatalf("file %q", got)
	}
	if win.km[input.Shortcut{Name: "F", Mods: ca}] != actFind {
		t.Fatal("Ctrl+Alt+F does not find")
	}
	if _, ok := win.km[defaultBindings(runtime.GOOS)["find"]]; ok {
		t.Fatal("the old keys still act")
	}
	if win.set.keyRec != "" {
		t.Fatalf("still recording %q", win.set.keyRec)
	}
}

func TestSavedKeysActInTheSameBatch(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	rect := keysControl(t, win, r, "key_rec", "toggle_sidebar").rect
	p := at(centre(rect))
	ca := key.ModCtrl | key.ModAlt
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p},
		key.Event{Name: "B", Modifiers: ca, State: key.Press},
	)
	sizedFrame(t, win, r, 1000, 900)
	if got := fileText(t, path); !strings.Contains(got, "toggle_sidebar = \"ctrl+alt+b\"") {
		t.Fatalf("file %q", got)
	}
	if win.km[input.Shortcut{Name: "B", Mods: ca}] != actToggleSidebar {
		t.Fatal("Ctrl+Alt+B does not toggle the sidebar")
	}
	if _, ok := win.km[defaultBindings(runtime.GOOS)["toggle_sidebar"]]; ok {
		t.Fatal("the old keys still act")
	}
	if win.set.keyRec != "" {
		t.Fatalf("still recording %q", win.set.keyRec)
	}
}

func TestRecordingReachesNoActionOrShell(t *testing.T) {
	win, r, st, _ := shortcutsWindow(t, "")
	n := len(win.mgr.Tabs())
	clickKeys(t, win, r, "find")
	nt := defaultBindings(runtime.GOOS)["new_tab"]
	pressIn(t, win, r, nt.Name, nt.Mods)
	if got := len(win.mgr.Tabs()); got != n {
		t.Fatalf("%d sessions after the new session's keys, want %d", got, n)
	}
	time.Sleep(50 * time.Millisecond)
	if got := ptyOf(st, 0).sent(); got != "" {
		t.Fatalf("the shell got %q", got)
	}
}

func TestRecordingAfterTypingInSearch(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickRect(t, win, r, controlsOf(win, "cmd_filter")[0].rect)
	r.Queue(key.EditEvent{Text: "fi"})
	keysFrames(t, win, r)
	if win.set.cmdFilter != "fi" {
		t.Fatalf("search text %q", win.set.cmdFilter)
	}
	clickKeys(t, win, r, "find")
	pressIn(t, win, r, "F", key.ModCtrl|key.ModAlt)
	if got := fileText(t, path); !strings.Contains(got, "find = \"ctrl+alt+f\"") {
		t.Fatalf("file %q", got)
	}
	if win.set.cmdFilter != "fi" {
		t.Fatalf("search text %q after recording", win.set.cmdFilter)
	}
}

func TestEditingOff(t *testing.T) {
	for _, c := range []struct{ cfg, want string }{
		{"", "Shortcuts can't be saved: Kaze has no config folder on this computer."},
		{"[keys\n", "config.toml has an error, so shortcuts can't be changed here: "},
		{"keys.find = \"f5\"\n", "Your shortcuts are written outside a [keys] table in config.toml. Move them into a [keys] table to change them here."},
	} {
		win, r, _, _ := shortcutsWindow(t, c.cfg)
		if c.cfg == "" {
			win.cfgPath = ""
			win.checkKeys()
			keysFrames(t, win, r)
		}
		if !strings.HasPrefix(win.set.keysErr, c.want) {
			t.Fatalf("%q: hint %q, want %q", c.cfg, win.set.keysErr, c.want)
		}
		if !strings.Contains(flatText(win), flat(win.set.keysErr)) {
			t.Fatalf("%q: the page does not show %q:\n%s", c.cfg, win.set.keysErr, pageText(win))
		}
		clickKeys(t, win, r, "find")
		if win.set.keyRec != "" {
			t.Fatalf("%q: recording with editing off", c.cfg)
		}
		if !hasKeys(win, strings.Split(win.keyText("palette"), "+")...) {
			t.Fatalf("%q: the rows lost their keys", c.cfg)
		}
	}
}

// The search field's editor keeps its own copy of kaze's keys; a save must
// make it take the new ones.
func TestApplyKeysDropsFieldFilters(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickRect(t, win, r, controlsOf(win, "cmd_filter")[0].rect)
	f := win.set.fields["cmd_filter"]
	if f == nil || f.filters == nil {
		t.Fatal("the search field has no key filters yet")
	}
	win.applyKeys(map[string]string{"find": "ctrl+alt+f"})
	if f.filters != nil {
		t.Fatal("the field still filters the old keys")
	}
}

func TestRefusedTexts(t *testing.T) {
	k := input.Shortcut{Name: "K"}
	cs := input.Shortcut{Name: "C", Mods: key.ModCtrl | key.ModShift}
	cmd := input.Shortcut{Name: "C", Mods: key.ModCommand}
	for _, c := range []struct {
		goos   string
		old    input.Shortcut
		hasOld bool
		want   string
	}{
		{"windows", cs, true, "K needs Ctrl or Alt with it, or it would stop you typing it. Copy keeps Ctrl+Shift+C."},
		{"darwin", cmd, true, "K needs Ctrl, Option or Cmd with it, or it would stop you typing it. Copy keeps Cmd+C."},
		{"windows", input.Shortcut{}, false, "K needs Ctrl or Alt with it, or it would stop you typing it. Copy stays without a shortcut."},
	} {
		if got := refusedText(c.goos, "Copy", k, c.old, c.hasOld); got != c.want {
			t.Errorf("%s: %q, want %q", c.goos, got, c.want)
		}
	}
}

func TestNeedsModifier(t *testing.T) {
	for _, sc := range []input.Shortcut{
		{Name: "K"}, {Name: "K", Mods: key.ModShift}, {Name: key.NameReturn}, {Name: key.NameEscape},
		{Name: key.NameTab}, {Name: key.NameSpace}, {Name: key.NameUpArrow}, {Name: key.NameUpArrow, Mods: key.ModShift},
		{Name: "1"}, {Name: ","},
	} {
		if !needsModifier(sc) {
			t.Errorf("%v+%q does not need a modifier", sc.Mods, sc.Name)
		}
	}
	for _, sc := range []input.Shortcut{
		{Name: key.NameF5}, {Name: key.NameF5, Mods: key.ModShift}, {Name: "K", Mods: key.ModCtrl},
		{Name: "K", Mods: key.ModAlt}, {Name: "K", Mods: key.ModCommand},
	} {
		if needsModifier(sc) {
			t.Errorf("%v+%q needs a modifier", sc.Mods, sc.Name)
		}
	}
}

func TestWarnTexts(t *testing.T) {
	for sc, want := range map[input.Shortcut]string{
		{Name: "R", Mods: key.ModCtrl}: "Saved. Your shell and Claude Code use Ctrl+R (reverse search). Kaze will take it from them.",
		{Name: "Q", Mods: key.ModCtrl}: "Saved. Your shell and Claude Code use Ctrl+Q. Kaze will take it from them.",
	} {
		if got, ok := warnText(sc); !ok || got != want {
			t.Errorf("%q: %q %v, want %q", sc.Name, got, ok, want)
		}
	}
	for _, sc := range []input.Shortcut{
		{Name: "R", Mods: key.ModCtrl | key.ModShift}, {Name: "R", Mods: key.ModAlt}, {Name: "1", Mods: key.ModCtrl}, {Name: key.NameF5},
	} {
		if got, ok := warnText(sc); ok {
			t.Errorf("%v+%q warns %q", sc.Mods, sc.Name, got)
		}
	}
}

func TestClashText(t *testing.T) {
	got := clashText("windows", input.Shortcut{Name: "B", Mods: key.ModCtrl | key.ModShift}, "Toggle sidebar")
	if want := "Ctrl+Shift+B is used by Toggle sidebar. Use it here instead? Toggle sidebar will have no shortcut."; got != want {
		t.Fatalf("%q, want %q", got, want)
	}
}

// stripText is the text of the shown strip, as the page drew it (wrapped lines joined).
func stripHas(t *testing.T, win *window, want string) {
	t.Helper()
	if !strings.Contains(flatText(win), flat(want)) {
		t.Fatalf("the page does not show %q:\n%s", want, pageText(win))
	}
}

// pressKeys presses sc while a row records.
func pressKeys(t *testing.T, win *window, r *gioinput.Router, sc input.Shortcut) {
	t.Helper()
	pressIn(t, win, r, sc.Name, sc.Mods)
}

func defaultOf(action string) input.Shortcut { return defaultBindings(runtime.GOOS)[action] }

func TestRefusedOnThePage(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "copy")
	pressIn(t, win, r, "K", 0)
	want := refusedText(runtime.GOOS, "Copy", input.Shortcut{Name: "K"}, defaultOf("copy"), true)
	if runtime.GOOS == "windows" && want != "K needs Ctrl or Alt with it, or it would stop you typing it. Copy keeps Ctrl+Shift+C." {
		t.Fatalf("refused text %q", want)
	}
	stripHas(t, win, want)
	if fileText(t, path) != "" || win.set.keyMsg.kind != msgRefused {
		t.Fatalf("file %q, strip %+v", fileText(t, path), win.set.keyMsg)
	}
	clickRect(t, win, r, keysControl(t, win, r, "key_ok", "copy").rect)
	if win.set.keyMsg.kind != msgNone || len(controlsOf(win, "key_ok")) != 0 {
		t.Fatalf("OK left the strip: %+v", win.set.keyMsg)
	}
}

func TestFKeyAllowed(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	pressIn(t, win, r, key.NameF5, 0)
	if got := fileText(t, path); !strings.Contains(got, "find = \"f5\"") {
		t.Fatalf("file %q", got)
	}
}

func TestSamePressChangesNothing(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	pressKeys(t, win, r, defaultOf("find"))
	if got := fileText(t, path); got != "" || win.set.keyMsg.kind != msgNone || win.set.keyRec != "" {
		t.Fatalf("file %q, strip %+v, recording %q", got, win.set.keyMsg, win.set.keyRec)
	}
}

// clashOnFind records find and presses Toggle sidebar's keys.
func clashOnFind(t *testing.T) (*window, *gioinput.Router, string) {
	t.Helper()
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	pressKeys(t, win, r, defaultOf("toggle_sidebar"))
	want := clashText(runtime.GOOS, defaultOf("toggle_sidebar"), "Toggle sidebar")
	if runtime.GOOS == "windows" && want != "Ctrl+Shift+B is used by Toggle sidebar. Use it here instead? Toggle sidebar will have no shortcut." {
		t.Fatalf("clash text %q", want)
	}
	stripHas(t, win, want)
	if len(controlsOf(win, "key_cancel")) != 1 || len(controlsOf(win, "key_replace")) != 1 {
		t.Fatal("no Cancel and Replace")
	}
	return win, r, path
}

func TestClashCancel(t *testing.T) {
	win, r, path := clashOnFind(t)
	km := maps.Clone(win.km)
	clickRect(t, win, r, keysControl(t, win, r, "key_cancel", "find").rect)
	if fileText(t, path) != "" || !maps.Equal(win.km, km) || win.set.keyMsg.kind != msgNone {
		t.Fatalf("Cancel: file %q, strip %+v", fileText(t, path), win.set.keyMsg)
	}
}

func TestClashReplace(t *testing.T) {
	win, r, path := clashOnFind(t)
	clickRect(t, win, r, keysControl(t, win, r, "key_replace", "find").rect)
	text, _ := shortcutText(defaultOf("toggle_sidebar"))
	got := fileText(t, path)
	if !strings.Contains(got, "find = \""+text+"\"") || !strings.Contains(got, "toggle_sidebar = \"none\"") {
		t.Fatalf("file %q", got)
	}
	if win.km[defaultOf("toggle_sidebar")] != actFind {
		t.Fatal("the keys do not find")
	}
	if _, ok := win.binds["toggle_sidebar"]; ok {
		t.Fatal("Toggle sidebar still has keys")
	}
	if win.set.keyMsg.kind != msgNone {
		t.Fatalf("strip %+v after Replace", win.set.keyMsg)
	}
	i := slices.Index(win.set.texts, "Toggle sidebar")
	if i < 0 || !slices.Contains(win.set.texts[i:min(i+4, len(win.set.texts))], "None") {
		t.Fatalf("the Toggle sidebar row does not read None:\n%s", pageText(win))
	}
}

func TestClashWithSettingsRefused(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "palette")
	pressKeys(t, win, r, defaultOf("open_settings"))
	stripHas(t, win, "Settings needs a shortcut. Pick other keys.")
	if len(controlsOf(win, "key_replace")) != 0 || fileText(t, path) != "" || len(controlsOf(win, "key_ok")) != 1 {
		t.Fatalf("Replace shown or file %q", fileText(t, path))
	}
}

func TestWarningOnThePage(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	clickKeys(t, win, r, "palette")
	pressIn(t, win, r, "R", key.ModCtrl)
	if got := fileText(t, path); !strings.Contains(got, "palette = \"ctrl+r\"") {
		t.Fatalf("file %q", got)
	}
	stripHas(t, win, "Saved. Your shell and Claude Code use Ctrl+R (reverse search). Kaze will take it from them.")
	if len(controlsOf(win, "key_ok")) != 1 {
		t.Fatal("no OK")
	}
	clickKeys(t, win, r, "find")
	if win.set.keyMsg.kind != msgNone {
		t.Fatalf("recording another row left the strip %+v", win.set.keyMsg)
	}
}

func TestDefaultKeysRemoveTheLine(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\n")
	clickKeys(t, win, r, "find")
	pressKeys(t, win, r, defaultOf("find"))
	if got := fileText(t, path); got != "[keys]\n" {
		t.Fatalf("file %q", got)
	}
}

func TestSaveFailureKeepsKeymap(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	if err := os.WriteFile(path, []byte("[keys\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	km := maps.Clone(win.km)
	clickKeys(t, win, r, "find")
	pressIn(t, win, r, "F", key.ModCtrl|key.ModAlt)
	if !strings.HasPrefix(win.set.keyMsg.text, "Couldn't save: ") || win.set.keyMsg.kind != msgRefused {
		t.Fatalf("strip %+v", win.set.keyMsg)
	}
	if !maps.Equal(win.km, km) {
		t.Fatal("a failed save changed the keymap")
	}
}

func TestPageSaveKeepsAHandEdit(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "[keys]\n")
	if err := os.WriteFile(path, []byte("[keys]\npalette = \"ctrl+alt+p\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clickKeys(t, win, r, "find")
	pressIn(t, win, r, "F", key.ModCtrl|key.ModAlt)
	got := fileText(t, path)
	if !strings.Contains(got, "palette = \"ctrl+alt+p\"") || !strings.Contains(got, "find = \"ctrl+alt+f\"") {
		t.Fatalf("file %q", got)
	}
	if win.binds["palette"] != (input.Shortcut{Name: "P", Mods: key.ModCtrl | key.ModAlt}) {
		t.Fatalf("palette %v", win.binds["palette"])
	}
}

// A save the keymap could not take as asked (a hand edit put another action on
// the same keys meanwhile) says so in the row instead of dropping the warning.
func TestSaveWarningShownInTheRow(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "[keys]\n")
	if err := os.WriteFile(path, []byte("[keys]\npalette = \"ctrl+alt+f\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clickKeys(t, win, r, "find")
	pressIn(t, win, r, "F", key.ModCtrl|key.ModAlt)
	m := win.set.keyMsg
	if m.kind != msgWarn || m.action != "find" || !strings.Contains(m.text, "keys.find") {
		t.Fatalf("strip %+v", m)
	}
	stripHas(t, win, m.text)
}

func TestChangedRowsHaveTagAndReset(t *testing.T) {
	win, _, _, _ := shortcutsWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\ncopy = \"none\"\n")
	if n := len(controlsOf(win, "key_reset")); n != 2 {
		t.Fatalf("%d reset buttons, want 2", n)
	}
	if n := strings.Count(pageText(win), "\nchanged\n"); n != 2 {
		t.Fatalf("%d changed tags, want 2", n)
	}
	win, _, _, _ = shortcutsWindow(t, "")
	if n := len(controlsOf(win, "key_reset")); n != 0 || strings.Contains(pageText(win), "\nchanged\n") {
		t.Fatalf("%d reset buttons with nothing changed", n)
	}
}

func TestResetRemovesTheLine(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\n")
	clickRect(t, win, r, keysControl(t, win, r, "key_reset", "find").rect)
	if got := fileText(t, path); got != "[keys]\n" {
		t.Fatalf("file %q", got)
	}
	if win.binds["find"] != defaultOf("find") {
		t.Fatalf("find %v", win.binds["find"])
	}
}

func TestResetAsksOnClash(t *testing.T) {
	text, _ := shortcutText(defaultOf("find"))
	win, r, _, path := shortcutsWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\npalette = \""+text+"\"\n")
	clickRect(t, win, r, keysControl(t, win, r, "key_reset", "find").rect)
	want := clashText(runtime.GOOS, defaultOf("find"), "Open the palette")
	if runtime.GOOS == "windows" && want != "Ctrl+Shift+F is used by Open the palette. Use it here instead? Open the palette will have no shortcut." {
		t.Fatalf("clash text %q", want)
	}
	stripHas(t, win, want)
	clickRect(t, win, r, keysControl(t, win, r, "key_replace", "find").rect)
	if got := fileText(t, path); got != "[keys]\npalette = \"none\"\n" {
		t.Fatalf("file %q", got)
	}
	if win.km[defaultOf("find")] != actFind {
		t.Fatal("the default keys do not find")
	}
}

func TestResetCannotUnbindSettings(t *testing.T) {
	text, _ := shortcutText(defaultOf("find"))
	cfg := "[keys]\nfind = \"ctrl+alt+f\"\nopen_settings = \"" + text + "\"\n"
	win, r, _, path := shortcutsWindow(t, cfg)
	clickRect(t, win, r, keysControl(t, win, r, "key_reset", "find").rect)
	stripHas(t, win, "Settings needs a shortcut. Pick other keys.")
	if got := fileText(t, path); got != cfg {
		t.Fatalf("file %q", got)
	}
}

// resetAllLink clicks "Reset all to defaults".
func resetAllLink(t *testing.T, win *window, r *gioinput.Router) {
	t.Helper()
	c := controlsOf(win, "key_reset_all")
	if len(c) != 1 {
		t.Fatalf("%d Reset all links", len(c))
	}
	if !c[0].rect.In(win.set.pane) {
		win.set.scroll = win.set.scrollMax
		sizedFrame(t, win, r, 1000, 900)
		c = controlsOf(win, "key_reset_all")
	}
	clickRect(t, win, r, c[0].rect)
}

func TestResetAllAsks(t *testing.T) {
	cfg := "[keys]\nfind = \"ctrl+alt+f\"\n"
	win, r, _, path := shortcutsWindow(t, cfg)
	resetAllLink(t, win, r)
	if !slices.Contains(win.set.texts, "Reset every shortcut to its default?") {
		t.Fatalf("no question:\n%s", pageText(win))
	}
	clickRect(t, win, r, controlsOf(win, "keys_cancel")[0].rect)
	if got := fileText(t, path); got != cfg || win.dialogShown() {
		t.Fatalf("file %q, dialog %v", got, win.dialogShown())
	}
}

func TestResetAllRemovesOnlyKnownLines(t *testing.T) {
	cfg := "# mine\ntheme = \"midnight\"\n\n[keys]\nfind = \"ctrl+alt+f\"\npalette = \"ctrl+alt+p\"\ncopy = \"none\"\nzoom = \"ctrl+=\"\n\n[colors]\nbg = \"#000000\"\n"
	win, r, _, path := shortcutsWindow(t, cfg)
	resetAllLink(t, win, r)
	clickRect(t, win, r, controlsOf(win, "keys_reset_all")[0].rect)
	want := "# mine\ntheme = \"midnight\"\n\n[keys]\nzoom = \"ctrl+=\"\n\n[colors]\nbg = \"#000000\"\n"
	if got := fileText(t, path); got != want {
		t.Fatalf("file %q, want %q", got, want)
	}
	if !maps.Equal(win.binds, defaultBindings(runtime.GOOS)) {
		t.Fatalf("binds %v", win.binds)
	}
	if win.dialogShown() {
		t.Fatal("the question is still up")
	}
}

func TestShortcutsContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		k := c.shortcutInk()
		low, lowMark := 99.0, 99.0
		text := func(label string, fg, bg color.NRGBA) {
			got := contrast(fg, bg)
			low = min(low, got)
			if got < 4.5 {
				t.Errorf("%s: %s contrast %.2f, want at least 4.5", th.Key, label, got)
			}
		}
		mark := func(label string, fg color.NRGBA) {
			got := contrast(fg, k.card)
			lowMark = min(lowMark, got)
			if got < 3 {
				t.Errorf("%s: %s against the card %.2f, want at least 3", th.Key, label, got)
			}
		}
		text("label", k.label, k.card)
		text("action", k.action, k.card)
		text("none", k.none, k.card)
		text("none on a focused button", k.noneFocus, k.btnFocusBG)
		text("tag", k.tagText, k.tagBG)
		text("cap", k.capText, k.capFace)
		text("recording", k.recText, k.recBG)
		text("label on the recording row", k.recLabel, k.recRow)
		text("action on the recording row", k.recAction, k.recRow)
		if got := contrast(k.resetIcon, k.recRow); got < 3 {
			t.Errorf("%s: reset icon on the recording row %.2f, want at least 3", th.Key, got)
		}
		if k.recRow == k.card {
			t.Errorf("%s: the recording row has no tint", th.Key)
		}
		for kind, name := range map[msgKind]string{msgRefused: "refused", msgClash: "clash", msgWarn: "warning"} {
			text(name+" strip", k.strip[kind].text, k.strip[kind].bg)
			mark(name+" strip line", k.strip[kind].line)
		}
		text("button", k.btnText, k.btnBG)
		text("strong button", k.strongText, k.strongBG)
		mark("recording line", k.recLine)
		mark("recording dot", k.recDot)
		mark("reset icon", k.resetIcon)
		t.Logf("%s: lowest text contrast %.2f, lowest mark %.2f", th.Key, low, lowMark)
	}
}

func TestRowLayout(t *testing.T) {
	win, _, _, _ := shortcutsWindow(t, "[keys]\nfind = \"ctrl+alt+f\"\n")
	rows := win.set.keyRows
	if len(rows) != len(editableActions(runtime.GOOS))+len(mouseLines) {
		t.Fatalf("%d rows laid out", len(rows))
	}
	for _, g := range rows {
		if g.action == "" {
			continue
		}
		if g.name.Y <= g.label.Y || g.name.X != g.label.X {
			t.Errorf("%s: name at %v, label at %v", g.action, g.name, g.label)
		}
	}
	for _, g := range rows[1:] {
		if g.reset.Max.X != rows[0].reset.Max.X || g.keys.Max.X != rows[0].keys.Max.X {
			t.Errorf("%q: keys end %d, reset slot %d; first row %d, %d", g.action, g.keys.Max.X, g.reset.Max.X, rows[0].keys.Max.X, rows[0].reset.Max.X)
		}
	}
	if rows[0].keys.Max.X > rows[0].reset.Min.X {
		t.Fatal("the keys overlap the reset slot")
	}
}

func TestNoneDrawn(t *testing.T) {
	win, _, _, _ := shortcutsWindow(t, "[keys]\ncopy = \"none\"\n")
	tx := win.set.texts
	i := slices.Index(tx, "Copy")
	if i < 0 || i+2 >= len(tx) || tx[i+1] != "copy" || tx[i+2] != "None" {
		t.Fatalf("the Copy row reads %q", tx[i:min(i+5, len(tx))])
	}
	if i+3 < len(tx) && tx[i+3] != "changed" {
		t.Fatalf("after None comes %q, want the changed tag", tx[i+3])
	}
}

func TestNarrowStripWraps(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	pressKeys(t, win, r, defaultOf("toggle_sidebar"))
	if win.set.keyMsg.kind != msgClash {
		t.Fatalf("no clash: %+v", win.set.keyMsg)
	}
	win.set.scroll = 0
	win.drawSettings(pageContext(520, 900), image.Rect(0, 38, 520, 876)) // a 520 dp page: menu and column
	s := win.set.keyStrip
	if s.box.Empty() {
		t.Fatal("no strip laid out")
	}
	in := false
	for _, c := range win.set.cards {
		if s.box.In(c) {
			in = true
		}
	}
	if !in {
		t.Fatalf("strip %v is outside every card %v", s.box, win.set.cards)
	}
	for _, id := range []string{"key_cancel", "key_replace"} {
		c := controlsOf(win, id)
		if len(c) != 1 || c[0].rect.Min.Y < s.textBottom || !c[0].rect.In(s.box) {
			t.Fatalf("%s at %v, text ends at %d, strip %v", id, c, s.textBottom, s.box)
		}
	}
}

// The recording row is tinted across the card, as in the mockup's .row.rec;
// the other rows are not.
func TestRecordingRowTinted(t *testing.T) {
	win, r, _, _ := shortcutsWindow(t, "")
	clickKeys(t, win, r, "find")
	if win.set.keyRec != "find" {
		t.Fatalf("not recording find (keyRec %q)", win.set.keyRec)
	}
	var card image.Rectangle
	for _, c := range win.set.cards {
		for _, g := range win.set.keyRows {
			if g.action == "find" && g.keys.In(c) {
				card = c
			}
		}
	}
	for _, g := range win.set.keyRows {
		switch {
		case g.action == "find":
			if g.tint.Empty() || g.tint.Min.X != card.Min.X || g.tint.Max.X != card.Max.X || !g.keys.In(g.tint) {
				t.Errorf("find's recording row tint %v, want the card's width %v around its keys %v", g.tint, card, g.keys)
			}
		case !g.tint.Empty():
			t.Errorf("%q is tinted %v while find records", g.action, g.tint)
		}
	}
}

// Holding Enter on a shortcut button starts recording once; the held key's
// auto-repeats are not the new keys, so no refusal shows. Once Enter is let
// go, the next keys record as usual.
func TestHeldEnterDoesNotRecordItself(t *testing.T) {
	win, r, _, path := shortcutsWindow(t, "")
	want := keysControl(t, win, r, "key_rec", "find")
	focused := func() bool {
		f := win.set.focus
		return f >= 0 && f < len(win.set.controls) && win.set.controls[f].id == "key_rec" && win.set.controls[f].arg == want.arg
	}
	for i := 0; i < 60 && !focused(); i++ {
		pressIn(t, win, r, key.NameTab, 0)
	}
	if !focused() {
		t.Fatal("Tab never reached find's shortcut")
	}
	press := key.Event{Name: key.NameReturn, State: key.Press}
	r.Queue(press)
	keysFrames(t, win, r)
	r.Queue(press, press, press) // the key held down: Gio repeats the press
	keysFrames(t, win, r)
	if win.set.keyRec != "find" || win.set.keyMsg.kind != msgNone {
		t.Fatalf("after a held Enter: recording %q, message %q", win.set.keyRec, win.set.keyMsg.text)
	}
	r.Queue(key.Event{Name: key.NameReturn, State: key.Release})
	keysFrames(t, win, r)
	pressIn(t, win, r, "G", key.ModCtrl|key.ModAlt)
	if win.set.keyRec != "" || !strings.Contains(fileText(t, path), `find = "ctrl+alt+g"`) {
		t.Fatalf("Ctrl+Alt+G after the release: recording %q, file %q", win.set.keyRec, fileText(t, path))
	}
}
