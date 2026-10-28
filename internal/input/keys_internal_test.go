package input

import (
	"testing"

	"gioui.org/io/key"
	"github.com/patrick-gogo/kaze/internal/term"
)

func ek(t *testing.T, goos string, name key.Name, mods key.Modifiers, want string) {
	t.Helper()
	got := encodeKey(goos, name, mods, term.Modes{})
	if want == "<nil>" {
		if got != nil {
			t.Fatalf("%s %q: got %q want nil", goos, name, got)
		}
		return
	}
	if string(got) != want {
		t.Fatalf("%s %q mods %d: got %q want %q", goos, name, mods, got, want)
	}
}

func TestReservedKeysAreNilPerPlatform(t *testing.T) {
	cs := key.ModCtrl | key.ModShift
	for _, n := range []key.Name{"C", "V", "T", "W"} {
		ek(t, "windows", n, cs, "<nil>")
		ek(t, "darwin", n, key.ModCommand, "<nil>")
	}
	ek(t, "windows", key.NameTab, key.ModCtrl, "<nil>")
	ek(t, "windows", "C", key.ModCtrl, "\x03")
	ek(t, "darwin", "C", key.ModCtrl, "\x03")
}

func TestUnreservedCmdAndSuperCombosAreSwallowed(t *testing.T) {
	ek(t, "darwin", "K", key.ModCommand, "<nil>")
	ek(t, "windows", "K", key.ModSuper, "<nil>")
	ek(t, "darwin", "K", key.ModCommand|key.ModShift, "<nil>")
}

func TestCtrlBackspaceAndPunctuation(t *testing.T) {
	ek(t, "windows", key.NameDeleteBackward, key.ModCtrl, "\x08")
	ek(t, "windows", key.NameDeleteBackward, 0, "\x7f")
	ek(t, "windows", "/", key.ModCtrl, "\x1f")
	ek(t, "windows", "@", key.ModCtrl, "\x00")
	ek(t, "windows", "2", key.ModCtrl, "\x00")
	ek(t, "windows", "?", key.ModCtrl, "\x7f")
	ek(t, "windows", "?", key.ModCtrl|key.ModShift, "\x7f")
}

func TestAltShiftTab(t *testing.T) {
	ek(t, "windows", key.NameTab, key.ModAlt|key.ModShift, "\x1b\x1b[Z")
}
