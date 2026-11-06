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
	// The title bar, palette and Settings read keyText every time, and the alert
	// caps are keyed by the shortcut itself.
	// Every text field keeps filters for kaze's keys, built once from win.km:
	// drop them all so each rebuilds from the new keymap.
	for _, f := range win.set.fields {
		f.filters = nil
	}
	win.side.filters, win.side.renKeys = nil, nil
	win.find.edKeys, win.board.edKeys = nil, nil
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
	win.set.keyRec, win.set.keyMsg, win.set.keyRecHeld = action, keyMessage{}, ""
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
	win.set.keyMsg, win.set.keysConfirm = keyMessage{}, false
}

// modifierNames are keys that only modify another; alone they never end recording.
var modifierNames = map[key.Name]bool{
	key.NameCtrl: true, key.NameShift: true, key.NameAlt: true, key.NameSuper: true, key.NameCommand: true,
}

// recordKey takes a key event while a row records. Releases, modifiers alone,
// the Windows / Super key, keys [keys] can't name and the auto-repeats of a
// held key that started recording keep it recording; Esc cancels; anything
// else ends recording and is the row's new keys.
func (win *window) recordKey(e key.Event) {
	if h := win.set.keyRecHeld; h != "" && e.Name == h {
		if e.State == key.Release {
			win.set.keyRecHeld = ""
		}
		return
	}
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

const settingsNeedsKeys = "Settings needs a shortcut. Pick other keys."

// needsModifier is true for keys that would stop the person typing them: no
// Ctrl, Alt or Cmd, and not an F-key (spec 3.2 step 1).
func needsModifier(sc input.Shortcut) bool {
	switch sc.Name {
	case key.NameF1, key.NameF2, key.NameF3, key.NameF4, key.NameF5, key.NameF6,
		key.NameF7, key.NameF8, key.NameF9, key.NameF10, key.NameF11, key.NameF12:
		return false
	}
	return sc.Mods&(key.ModCtrl|key.ModAlt|key.ModCommand) == 0
}

// refusedText says why sc can't be label's keys and what it keeps (Ruling 4:
// macOS names Option and Cmd, as its caps do).
func refusedText(goos, label string, sc, old input.Shortcut, hasOld bool) string {
	need := "Ctrl or Alt"
	if goos == "darwin" {
		need = "Ctrl, Option or Cmd"
	}
	end := label + " stays without a shortcut."
	if hasOld {
		end = label + " keeps " + formatShortcut(goos, old) + "."
	}
	return formatShortcut(goos, sc) + " needs " + need + " with it, or it would stop you typing it. " + end
}

// clashText asks whether sc should move here from otherLabel.
func clashText(goos string, sc input.Shortcut, otherLabel string) string {
	return formatShortcut(goos, sc) + " is used by " + otherLabel + ". Use it here instead? " + otherLabel + " will have no shortcut."
}

// shellKeyUse is what the shell or Claude Code does with Ctrl+<letter>.
var shellKeyUse = map[key.Name]string{"R": "reverse search", "C": "stop a command", "D": "end input",
	"L": "clear the screen", "A": "go to line start", "E": "go to line end",
	"U": "delete the line", "W": "delete a word", "Z": "suspend"}

// warnText is the warning for plain Ctrl+<letter>, which the shell loses.
func warnText(sc input.Shortcut) (string, bool) {
	n := string(sc.Name)
	if sc.Mods != key.ModCtrl || len(n) != 1 || n[0] < 'A' || n[0] > 'Z' {
		return "", false
	}
	use := ""
	if u, ok := shellKeyUse[sc.Name]; ok {
		use = " (" + u + ")"
	}
	return "Saved. Your shell and Claude Code use Ctrl+" + n + use + ". Kaze will take it from them.", true
}

// actionLabel is name's label on the page.
func actionLabel(name string) string {
	for _, c := range kazeCommands {
		if c.Config == name {
			return c.Label
		}
	}
	return name
}

// keyPressed decides what the keys pressed on action's row do (spec 3.2):
// refused, nothing, a clash question, or saved (with a warning when the shell
// loses the keys).
func (win *window) keyPressed(action string, sc input.Shortcut) {
	s := &win.set
	old, hasOld := win.binds[action]
	if needsModifier(sc) {
		s.keyMsg = keyMessage{action: action, kind: msgRefused, keys: sc,
			text: refusedText(runtime.GOOS, actionLabel(action), sc, old, hasOld)}
		return
	}
	if hasOld && old == sc {
		return
	}
	if o, ok := win.boundTo(sc, action); ok {
		if o == "open_settings" {
			s.keyMsg = keyMessage{action: action, kind: msgRefused, text: settingsNeedsKeys} // Ruling 2
			return
		}
		s.keyMsg = keyMessage{action: action, kind: msgClash, keys: sc, other: o,
			text: clashText(runtime.GOOS, sc, actionLabel(o))}
		return
	}
	win.saveFor(action, sc, map[string]string{action: keysValue(action, sc)})
}

// boundTo is the action other than except that has sc.
func (win *window) boundTo(sc input.Shortcut, except string) (string, bool) {
	for _, o := range sortedNames(win.binds) {
		if o != except && win.binds[o] == sc {
			return o, true
		}
	}
	return "", false
}

// keysValue is action's [keys] value for sc: "" (no line) when sc is the
// default (Ruling 1).
func keysValue(action string, sc input.Shortcut) string {
	if sc == defaultBindings(runtime.GOOS)[action] {
		return ""
	}
	text, _ := shortcutText(sc)
	return text
}

// saveFor saves edits, which give action the keys sc, and puts what the person
// should know in action's row: a save error, a warning the keymap gave about
// an edited action, or the shell-keys warning. A plain save shows nothing
// (Ruling 11).
func (win *window) saveFor(action string, sc input.Shortcut, edits map[string]string) {
	s := &win.set
	s.keyMsg = keyMessage{}
	warns, err := win.saveKeys(edits)
	if err != nil {
		s.keyMsg = keyMessage{action: action, kind: msgRefused, text: "Couldn't save: " + err.Error()}
		return
	}
	for _, w := range warns {
		for name := range edits {
			if strings.HasPrefix(w, "keys."+name+":") {
				s.keyMsg = keyMessage{action: action, kind: msgWarn, text: "Saved, with a warning: " + w}
				return
			}
		}
	}
	if text, ok := warnText(sc); ok {
		s.keyMsg = keyMessage{action: action, kind: msgWarn, keys: sc, text: text}
	}
}

// saveKeys writes edits to [keys] and rebuilds the keymap from the file,
// returning its warnings. On a write error the live keymap stays as it was.
func (win *window) saveKeys(edits map[string]string) ([]string, error) {
	if win.cfgPath == "" {
		return nil, errors.New("kaze has no config file")
	}
	err := config.SetKeys(win.cfgPath, edits)
	var warns []string
	if err == nil {
		warns, err = win.loadKeys()
	}
	win.checkKeys()
	win.w.Invalidate()
	return warns, err
}

// keysStrip runs a strip button: OK and Cancel close it, Replace moves the keys.
func (win *window) keysStrip(id string) {
	s := &win.set
	m := s.keyMsg
	s.keyMsg = keyMessage{}
	if id != "key_replace" || m.kind != msgClash {
		return
	}
	value := keysValue(m.action, m.keys)
	if m.reset {
		value = ""
	}
	win.saveFor(m.action, m.keys, map[string]string{m.action: value, m.other: "none"})
}

// keyChanged reports whether action's keys are not its default, an unbound
// action included (spec 8.7).
func (win *window) keyChanged(action string) bool {
	sc, ok := win.binds[action]
	return !ok || sc != defaultBindings(runtime.GOOS)[action]
}

// resetKey puts action back on its default, asking first when another action
// has those keys now.
func (win *window) resetKey(action string) {
	s := &win.set
	win.stopRecording()
	s.keyMsg = keyMessage{}
	def := defaultBindings(runtime.GOOS)[action]
	if o, ok := win.boundTo(def, action); ok {
		if o == "open_settings" {
			s.keyMsg = keyMessage{action: action, kind: msgRefused, text: settingsNeedsKeys} // Ruling 2
			return
		}
		s.keyMsg = keyMessage{action: action, kind: msgClash, keys: def, other: o, reset: true,
			text: clashText(runtime.GOOS, def, actionLabel(o))}
		return
	}
	win.saveFor(action, def, map[string]string{action: ""})
}

// resetAllKeys removes the line of every action the page edits; lines for
// names Kaze doesn't know stay.
func (win *window) resetAllKeys() {
	s := &win.set
	s.keysConfirm = false
	win.stopRecording()
	s.keyMsg = keyMessage{}
	edits := map[string]string{}
	for _, name := range editableActions(runtime.GOOS) {
		edits[name] = ""
	}
	if _, err := win.saveKeys(edits); err != nil {
		win.note("Couldn't save: " + err.Error())
	}
}

// keysDialogBox is the Reset all question while it is up.
func keysDialogBox(win *window) *dialogBox {
	if !win.set.keysConfirm {
		return nil
	}
	return &dialogBox{
		title: "Reset every shortcut to its default?",
		buttons: []ctrl{
			{kind: ctlButton, id: "keys_cancel", text: "Cancel"},
			{kind: ctlButton, id: "keys_reset_all", text: "Reset all", strong: true},
		},
	}
}
