package term

import (
	"fmt"
	"time"
)

// Modes is the set of DEC private modes the host cares about.
type Modes struct {
	AppCursorKeys, BracketedPaste, CursorVisible bool
	MouseX10, MouseButton, MouseAny, MouseSGR    bool
	Synchronized, AltScreen                      bool
	FocusEvents                                  bool // DEC 1004: report focus in/out
}

func (a *Applier) Modes() Modes { return a.modes }

// SyncSince is when synchronized output last started; zero when it is off.
func (a *Applier) SyncSince() time.Time { return a.syncSince }

func (a *Applier) Title() string { return a.title }

// setPrivateMode handles CSI ? Pm h / l.
func (a *Applier) setPrivateMode(params []int, on bool) {
	for _, p := range params {
		switch p {
		case 1:
			a.modes.AppCursorKeys = on
		case 25:
			a.modes.CursorVisible = on
		case 47, 1047:
			a.s.UseAltScreen(on)
			a.modes.AltScreen = on
		case 1049:
			if on == a.modes.AltScreen {
				continue
			}
			if on {
				a.s.SaveAltCursor()
			}
			a.s.UseAltScreen(on)
			a.modes.AltScreen = on
			if !on {
				a.s.RestoreAltCursor()
			}
		case 9:
			a.modes.MouseX10 = on
		case 1000, 1002:
			a.modes.MouseButton = on
		case 1003:
			a.modes.MouseAny = on
		case 1006:
			a.modes.MouseSGR = on
		case 1004:
			a.modes.FocusEvents = on
		case 2004:
			a.modes.BracketedPaste = on
		case 2026:
			a.modes.Synchronized = on
			if !on {
				a.syncSince = time.Time{}
			} else if a.syncSince.IsZero() {
				a.syncSince = a.now()
			}
		}
	}
}

// queryPrivateMode answers DECRQM: 1 set, 2 reset, 0 unknown.
func (a *Applier) queryPrivateMode(p int) {
	state := 0
	var v bool
	switch p {
	case 1:
		v = a.modes.AppCursorKeys
	case 25:
		v = a.modes.CursorVisible
	case 47, 1047, 1049:
		v = a.modes.AltScreen
	case 1000, 1002:
		v = a.modes.MouseButton
	case 1003:
		v = a.modes.MouseAny
	case 1006:
		v = a.modes.MouseSGR
	case 1004:
		v = a.modes.FocusEvents
	case 2004:
		v = a.modes.BracketedPaste
	case 2026:
		v = a.modes.Synchronized
	default:
		a.replyf("\x1b[?%d;0$y", p)
		return
	}
	state = 2
	if v {
		state = 1
	}
	a.replyf("\x1b[?%d;%d$y", p, state)
}

func (a *Applier) replyf(format string, args ...any) {
	if a.reply != nil {
		fmt.Fprintf(a.reply, format, args...)
	}
}

// deviceStatus handles CSI 5 n and CSI 6 n.
func (a *Applier) deviceStatus(params []int) {
	switch param(params, 0, 0) {
	case 5:
		a.replyf("\x1b[0n")
	case 6:
		col, row := a.s.Cursor()
		a.replyf("\x1b[%d;%dR", row+1, col+1)
	}
}
