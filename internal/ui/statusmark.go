package ui

import (
	"strings"
	"unicode"
)

// isStatusRune reports whether r is a status-like symbol an agent puts before
// its title (Claude Code's ✳, a ◐ or braille spinner frame). ASCII never is.
func isStatusRune(r rune) bool {
	if r < 0x80 {
		return false
	}
	switch {
	case r >= 0x2300 && r <= 0x23ff, // misc technical (⏺)
		r >= 0x25a0 && r <= 0x25ff,            // geometric shapes (◐)
		r >= 0x2700 && r <= 0x27bf,            // dingbats (✳ ✻ ✽)
		r >= 0x2800 && r <= 0x28ff,            // braille spinners
		r == 0xfe0e, r == 0xfe0f, r == 0x200d: // variation selectors and ZWJ belong to the emoji before them
		return true
	}
	return unicode.In(r, unicode.So, unicode.Sm, unicode.Sk) // Sk: skin-tone modifiers
}

// dropStatusMark strips a leading run of status symbols and spaces. A title
// that is nothing but symbols comes back unchanged, never empty: a spinner
// would otherwise rename the row on every turn, and the UI fonts draw most of
// these runes as empty boxes.
func dropStatusMark(s string) string {
	t := strings.TrimLeftFunc(s, func(r rune) bool { return unicode.IsSpace(r) || isStatusRune(r) })
	if strings.TrimSpace(t) == "" {
		return s
	}
	return t
}

// onlyStatus reports whether s holds nothing but spaces and status symbols.
func onlyStatus(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) && !isStatusRune(r) }) < 0
}
