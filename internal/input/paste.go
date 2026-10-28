package input

import (
	"bytes"

	"github.com/patrick-gogo/kaze/internal/term"
)

var (
	pasteStart = []byte("\x1b[200~")
	pasteEnd   = []byte("\x1b[201~")
)

// EncodePaste normalizes newlines to CR and, in bracketed-paste mode, wraps
// the text. The end marker is stripped from the text so a paste cannot close
// the bracket early and smuggle in commands. One buffer, one pass.
func EncodePaste(s string, m term.Modes) []byte {
	out := make([]byte, 0, len(s)+len(pasteStart)+len(pasteEnd))
	if m.BracketedPaste {
		out = append(out, pasteStart...)
	}
	body := len(out)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\r':
			out = append(out, '\r')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case '\n':
			out = append(out, '\r')
		default:
			out = append(out, c)
		}
	}
	// Removing a marker can join its neighbours into a new one; repeat.
	for {
		j := bytes.Index(out[body:], pasteEnd)
		if j < 0 {
			break
		}
		j += body
		out = append(out[:j], out[j+len(pasteEnd):]...)
	}
	if m.BracketedPaste {
		out = append(out, pasteEnd...)
	}
	return out
}
