package input_test

import (
	"testing"

	"gioui.org/io/key"
	"github.com/patrick-gogo/kaze/internal/input"
	"github.com/patrick-gogo/kaze/internal/term"
)

func TestWheelInSGRMouseMode(t *testing.T) {
	m := term.Modes{MouseButton: true, MouseSGR: true}
	chk(t, input.EncodeWheel(1, 4, 2, m), "\x1b[<64;5;3M")
	chk(t, input.EncodeWheel(-1, 4, 2, m), "\x1b[<65;5;3M")
	chk(t, input.EncodeWheel(2, 0, 0, m), "\x1b[<64;1;1M\x1b[<64;1;1M")
	if input.EncodeWheel(1, 4, 2, term.Modes{}) != nil {
		t.Fatal("expected nil outside mouse mode")
	}
}

func TestMouseClickSGRPressAndRelease(t *testing.T) {
	m := term.Modes{MouseButton: true, MouseSGR: true}
	chk(t, input.EncodeMouse(0, true, 0, 9, 4, false, m), "\x1b[<0;10;5M")
	chk(t, input.EncodeMouse(0, false, 0, 9, 4, false, m), "\x1b[<0;10;5m")
	chk(t, input.EncodeMouse(2, true, key.ModCtrl|key.ModShift, 0, 0, false, m), "\x1b[<22;1;1M")
}

func TestMouseMotionOnlyReportedInMatchingMode(t *testing.T) {
	btn := term.Modes{MouseButton: true, MouseSGR: true}
	any := term.Modes{MouseAny: true, MouseSGR: true}
	if input.EncodeMouse(-1, true, 0, 1, 1, true, btn) != nil {
		t.Fatal("hover must not report under MouseButton")
	}
	chk(t, input.EncodeMouse(-1, true, 0, 1, 1, true, any), "\x1b[<35;2;2M")
	chk(t, input.EncodeMouse(0, true, 0, 1, 1, true, btn), "\x1b[<32;2;2M")
	if input.EncodeMouse(0, true, 0, 1, 1, false, term.Modes{}) != nil {
		t.Fatal("nil without mouse mode")
	}
}

func TestMouseLegacyEncodingWithoutSGR(t *testing.T) {
	chk(t, input.EncodeMouse(0, true, 0, 0, 0, false, term.Modes{MouseX10: true}), "\x1b[M !!")
	chk(t, input.EncodeMouse(0, false, 0, 0, 0, false, term.Modes{MouseButton: true}), "\x1b[M#!!")
	if input.EncodeMouse(0, false, 0, 0, 0, false, term.Modes{MouseX10: true}) != nil {
		t.Fatal("X10 reports presses only")
	}
	if input.EncodeMouse(0, true, 0, 300, 0, false, term.Modes{MouseButton: true}) != nil {
		t.Fatal("legacy cannot encode col > 222")
	}
}
