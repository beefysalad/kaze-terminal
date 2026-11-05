package ui

import (
	"errors"
	"runtime"
	"strings"

	"gioui.org/io/key"

	"github.com/beefysalad/kaze-terminal/internal/config"
	"github.com/beefysalad/kaze-terminal/internal/input"
)

// applyKeys sets win.km, win.binds, win.opt.Config.Keys and the reserved keys
// together from [keys] text, and drops cached key labels. Start-up and every
// save go through it, so a save gives what a restart would. Returns the warnings.
func (win *window) applyKeys(keys map[string]string) []string {
	km, warns := buildKeymap(runtime.GOOS, keys)
	binds, _ := resolveBindings(runtime.GOOS, keys)
	win.km, win.binds = km, binds
	win.opt.Config.Keys = keys
	input.SetReserved(runtime.GOOS, km.shortcuts())
	// The launcher caches its label; the title bar, palette and Settings read
	// keyText every time, and the alert caps are keyed by the shortcut itself.
	win.side.launch.work.shortcutReady = false
	// Settings' text fields keep filters for kaze's keys; they rebuild them.
	for _, f := range win.set.fields {
		f.filters = nil
	}
	if win.w != nil {
		win.w.Invalidate()
	}
	return warns
}

// reloadKeys re-Loads win.cfgPath and applies only its [keys] (spec 8.2). On a
// load error it returns the error and changes nothing.
func (win *window) reloadKeys() error {
	_, err := win.loadKeys()
	return err
}

// loadKeys is reloadKeys with the warnings applyKeys gave.
func (win *window) loadKeys() ([]string, error) {
	if win.cfgPath == "" {
		return nil, errors.New("kaze has no config file")
	}
	cfg, _, err := config.Load(win.cfgPath)
	if err != nil {
		return nil, err
	}
	return win.applyKeys(cfg.Keys), nil
}

// msgKind is what a message strip under a shortcut row says.
type msgKind int

const (
	msgNone msgKind = iota
	msgRefused
	msgClash
	msgWarn
)

// keyMessage is the strip under one shortcut row.
type keyMessage struct {
	action string // the row it sits under
	kind   msgKind
	text   string
	keys   input.Shortcut // clash: the keys asked about
	other  string         // clash: the action that has them
	reset  bool           // clash raised by a reset: Replace removes the line instead of setting it
}

const keysHint = "Click a shortcut to change it. Changes save at once."

// editableActions are the actions the Shortcuts page can change, in
// kazeCommands order: every action with a default on goos (spec 8.1).
func editableActions(goos string) []string {
	def := defaultBindings(goos)
	var out []string
	for _, c := range kazeCommands {
		if _, ok := def[c.Config]; ok {
			out = append(out, c.Config)
		}
	}
	return out
}

// checkKeys sets keysErr: why the page can't change shortcuts, "" when it can.
func (win *window) checkKeys() {
	s := &win.set
	switch err := config.KeysEditable(win.cfgPath); {
	case win.cfgPath == "":
		s.keysErr = "Shortcuts can't be saved: Kaze has no config folder on this computer."
	case errors.Is(err, config.ErrKeysForm):
		s.keysErr = "Your shortcuts are written outside a [keys] table in config.toml. Move them into a [keys] table to change them here."
	case err != nil:
		s.keysErr = "config.toml has an error, so shortcuts can't be changed here: " + strings.Join(strings.Fields(err.Error()), " ")
	default:
		s.keysErr = ""
	}
	if s.keysErr != "" {
		win.stopRecording()
	}
}

// startRecording makes action's row wait for the next key press.
func (win *window) startRecording(action string) {
	if win.set.keysErr != "" {
		return
	}
	win.set.keyRec, win.set.keyMsg = action, keyMessage{}
	win.w.Invalidate()
}

func (win *window) stopRecording() {
	if win.set.keyRec != "" {
		win.set.keyRec = ""
		win.w.Invalidate()
	}
}

// dropKeyEdit ends recording and clears the strip: the page left the
// section, Settings closed or the window lost focus (Ruling 6).
func (win *window) dropKeyEdit() {
	win.stopRecording()
	win.set.keyMsg = keyMessage{}
}

// modifierNames are keys that only modify another; alone they never end recording.
var modifierNames = map[key.Name]bool{
	key.NameCtrl: true, key.NameShift: true, key.NameAlt: true, key.NameSuper: true, key.NameCommand: true,
}

// recordKey takes a key event while a row records. Releases, modifiers alone,
// the Windows / Super key and keys [keys] can't name keep it recording; Esc
// cancels; anything else ends recording and is the row's new keys.
func (win *window) recordKey(e key.Event) {
	if e.State != key.Press || modifierNames[e.Name] || e.Modifiers.Contain(key.ModSuper) {
		return
	}
	if e.Name == key.NameEscape {
		win.stopRecording()
		return
	}
	sc := input.Shortcut{Name: e.Name, Mods: e.Modifiers}
	if _, ok := shortcutText(sc); !ok {
		return
	}
	action := win.set.keyRec
	win.stopRecording()
	win.keyPressed(action, sc)
}

// keyPressed decides what the keys pressed on action's row do.
func (win *window) keyPressed(action string, sc input.Shortcut) {
	win.saveFor(action, sc)
}

// saveFor saves sc as action's keys: the line goes when sc is the default
// (Ruling 1). A save error shows in the row.
func (win *window) saveFor(action string, sc input.Shortcut) {
	text := ""
	if sc != defaultBindings(runtime.GOOS)[action] {
		text, _ = shortcutText(sc)
	}
	if err := win.saveKeys(map[string]string{action: text}); err != nil {
		win.set.keyMsg = keyMessage{action: action, kind: msgRefused, text: "Couldn't save: " + err.Error()}
	}
}

// saveKeys writes edits to [keys] and rebuilds the keymap from the file. On a
// write error the live keymap stays as it was.
func (win *window) saveKeys(edits map[string]string) error {
	if win.cfgPath == "" {
		return errors.New("kaze has no config file")
	}
	err := config.SetKeys(win.cfgPath, edits)
	if err == nil {
		_, err = win.loadKeys()
	}
	win.checkKeys()
	win.w.Invalidate()
	return err
}
