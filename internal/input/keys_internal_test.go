package input

import (
	"testing"

	"gioui.org/io/key"
	"github.com/patrick-gogo/kaze-terminal/internal/term"
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

func TestSetReservedReplacesThePlatformTable(t *testing.T) {
	saved := reservedKeys["windows"]
	t.Cleanup(func() { reservedKeys["windows"] = saved })

	SetReserved("windows", []Shortcut{{Name: "T", Mods: key.ModCtrl | key.ModAlt}})
	ek(t, "windows", "T", key.ModCtrl|key.ModAlt, "<nil>")
	// The old default is free again, so it reaches the shell.
	ek(t, "windows", "T", key.ModCtrl|key.ModShift, "\x14")
	if !IsReserved("T", key.ModCtrl|key.ModAlt, "windows") {
		t.Fatal("IsReserved does not see the new table")
	}
}

// Shift+Enter aliases Alt/Option+Enter for programs with multiline prompts.
// Both Return and keypad Enter must stay distinct from ordinary submission.
func TestShiftEnterUsesMultilineSequence(t *testing.T) {
	for _, goos := range []string{"windows", "darwin"} {
		for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
			for _, tc := range []struct {
				mods key.Modifiers
				want string
			}{
				{0, "\r"},
				{key.ModShift, "\x1b\r"},
				{key.ModAlt, "\x1b\r"},
				{key.ModShift | key.ModAlt, "\x1b\r"},
				{key.ModCtrl, "\r"},
				{key.ModCtrl | key.ModShift, "\r"},
				{key.ModCommand, "\r"},
				{key.ModCommand | key.ModShift, "\r"},
			} {
				ek(t, goos, name, tc.mods, tc.want)
			}
		}
	}
}
