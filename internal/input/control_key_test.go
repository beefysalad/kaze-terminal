package input

import (
	"runtime"
	"testing"

	"gioui.org/io/key"
	"github.com/beefysalad/kaze-terminal/internal/term"
)

func TestTerminalKeyBypassesLocalShortcuts(t *testing.T) {
	reservedMu.Lock()
	old := reservedKeys[runtime.GOOS]
	reservedKeys[runtime.GOOS] = append(append([]Shortcut(nil), old...), Shortcut{"C", key.ModCtrl}, Shortcut{key.NameUpArrow, 0})
	reservedMu.Unlock()
	defer func() { reservedMu.Lock(); reservedKeys[runtime.GOOS] = old; reservedMu.Unlock() }()
	if b := EncodeKey("C", key.ModCtrl, term.Modes{}); b != nil {
		t.Fatalf("reserved key sent %q", b)
	}

	if got := string(EncodeTerminalKey("C", key.ModCtrl, term.Modes{})); got != "\x03" {
		t.Fatalf("terminal Ctrl+C %q", got)
	}
	if got := string(EncodeTerminalKey(key.NameUpArrow, 0, term.Modes{AppCursorKeys: true})); got != "\x1bOA" {
		t.Fatalf("terminal Up %q", got)
	}
	if b := EncodeTerminalKey(key.NameCtrl, 0, term.Modes{}); b != nil {
		t.Fatalf("modifier sent %q", b)
	}
}
