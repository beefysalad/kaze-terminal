package input

import (
	"strconv"

	"gioui.org/io/key"
	"github.com/patrick-gogo/kaze/internal/term"
)

func mouseOn(m term.Modes) bool { return m.MouseX10 || m.MouseButton || m.MouseAny }

// EncodeWheel reports wheel steps (positive lines = up) as button 64/65
// presses. nil means mouse reporting is off and the caller scrolls the view.
func EncodeWheel(lines int, col, row int, m term.Modes) []byte {
	if !mouseOn(m) || lines == 0 {
		return nil
	}
	btn, n := 64, lines
	if lines < 0 {
		btn, n = 65, -lines
	}
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, report(btn, true, col, row, m)...)
	}
	return out
}

// EncodeMouse reports a button event. button is 0 left, 1 middle, 2 right,
// -1 for none (hover). nil when the active mode doesn't report the event.
func EncodeMouse(button int, press bool, mods key.Modifiers, col, row int, motion bool, m term.Modes) []byte {
	if !mouseOn(m) {
		return nil
	}
	if motion {
		if !m.MouseAny && !(m.MouseButton && button >= 0) {
			return nil
		}
	} else if !press && !m.MouseButton && !m.MouseAny && !m.MouseSGR {
		return nil // X10 reports presses only
	}
	if !motion && button < 0 {
		return nil
	}
	cb := button
	if cb < 0 {
		cb = 3
	}
	if mods&key.ModShift != 0 {
		cb |= 4
	}
	if mods&key.ModAlt != 0 {
		cb |= 8
	}
	if mods&key.ModCtrl != 0 {
		cb |= 16
	}
	if motion {
		cb |= 32
	}
	return report(cb, press, col, row, m)
}

func report(cb int, press bool, col, row int, m term.Modes) []byte {
	if m.MouseSGR {
		end := byte('M')
		if !press {
			end = 'm'
		}
		return []byte("\x1b[<" + strconv.Itoa(cb) + ";" + strconv.Itoa(col+1) + ";" + strconv.Itoa(row+1) + string(end))
	}
	if !press {
		cb = cb&^3 | 3 // legacy release carries no button
	}
	if col+33 > 255 || row+33 > 255 || cb+32 > 255 {
		return nil
	}
	return []byte{0x1b, '[', 'M', byte(cb + 32), byte(col + 33), byte(row + 33)}
}
