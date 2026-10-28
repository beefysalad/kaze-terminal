package input

import (
	"bytes"

	"github.com/patrick-gogo/kaze-terminal/internal/term"
)

var (
	pasteStart = []byte("\x1b[200~")
	pasteEnd   = []byte("\x1b[201~")
)

// EncodePaste normalizes newlines to CR and, in bracketed-paste mode, wraps
// the text. Both bracket markers are stripped from the text so a paste cannot close
// the bracket early and smuggle in commands. One buffer, one pass.
func EncodePaste(s string, m term.Modes) []byte {
	out := make([]byte, 0, len(s)+len(pasteStart)+len(pasteEnd))
	if m.BracketedPaste {
		out = append(out, pasteStart...)
	}
	body := len(out)
	// Filter while copying: after each byte, drop a marker that just completed.
	// This also catches markers re-joined by an earlier strip, in O(n).
	push := func(c byte) {
		out = append(out, c)
		if n := len(out); n-body >= 6 && c == '~' {
			tail := out[n-6:]
			if bytes.Equal(tail, pasteEnd) || bytes.Equal(tail, pasteStart) {
				out = out[:n-6]
			}
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\r':
			push('\r')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case '\n':
			push('\r')
		default:
			push(c)
		}
	}
	if m.BracketedPaste {
		out = append(out, pasteEnd...)
	}
	return out
}
