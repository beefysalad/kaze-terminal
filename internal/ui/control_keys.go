package ui

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
	"github.com/beefysalad/kaze-terminal/internal/input"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

// Keys keeps a sequence together with respect to other control requests.
func (h *controlHost) Keys(id string, keys []string) error {
	target, err := h.session(id)
	if err != nil {
		return err
	}
	return target.keys(keys)
}

func (h *controlHost) Interrupt(id string, ctrlC bool) error {
	// Select both the agent and session from the same immutable publication.
	snapshot := h.snapshot.Load()
	if snapshot == nil || snapshot.sessions[id] == nil {
		return fmt.Errorf("tab %s is closed or unknown", id)
	}
	name := "ctrl+c"
	if !ctrlC {
		for _, tab := range snapshot.tabs {
			if tab.ID == id && (tab.Agent == "claude" || tab.Agent == "codex") {
				name = "esc"
				break
			}
		}
	}
	return snapshot.sessions[id].keys([]string{name})
}

func (target *controlSession) keys(keys []string) error {
	target.send.Lock()
	defer target.send.Unlock()
	s := target.session
	s.Lock()
	modes := s.Applier().Modes()
	s.Unlock()
	return submitControlKeys(keys, modes, s.Submit)
}

// submitControlKeys uses the terminal's existing key encoder, one Submit per
// key. Accepted means queued, as for other Session input, not PTY completion.
func submitControlKeys(keys []string, modes term.Modes, submit func([]byte) term.SubmitResult) error {
	for _, name := range keys {
		var k key.Name
		var mods key.Modifiers
		switch strings.ToLower(name) {
		case "enter":
			k = key.NameReturn
		case "esc":
			k = key.NameEscape
		case "tab":
			k = key.NameTab
		case "up":
			k = key.NameUpArrow
		case "down":
			k = key.NameDownArrow
		case "left":
			k = key.NameLeftArrow
		case "right":
			k = key.NameRightArrow
		case "backspace":
			k = key.NameDeleteBackward
		case "ctrl+c":
			k, mods = "C", key.ModCtrl
		case "ctrl+d":
			k, mods = "D", key.ModCtrl
		case "ctrl+l":
			k, mods = "L", key.ModCtrl
		default:
			return fmt.Errorf("unknown key %q", name)
		}
		encoded := input.EncodeTerminalKey(k, mods, modes)
		if len(encoded) == 0 {
			return fmt.Errorf("key %q has no terminal encoding", name)
		}
		if result := submit(encoded); result != term.Accepted {
			return fmt.Errorf("key %q: %s", name, result)
		}
	}
	return nil
}
