// Package find searches a terminal screen and its scrollback for a plain text needle.
//
// It reads cells in place (no copy of the buffer), in bounded steps, and keeps its
// matches in stable line numbers (screen.Evicted() plus the absolute row), so the
// history ring dropping lines off the top never moves a match. It imports only
// internal/screen and the standard library, and it allocates nothing once warm.
package find

import "unicode"

// MaxNeedle is how many runes of the query count; the rest is ignored.
const MaxNeedle = 256

// Pos is one cell in stable lines: Line = screen.Evicted() + absolute row (history first).
type Pos struct{ Line, Col int }

// Match is one occurrence. End is its last cell, inclusive (the right half of a wide char).
type Match struct{ Start, End Pos }

// fold lowercases r rune by rune (simple lowercase, no full case folding: Ruling 3).
func fold(r rune) rune {
	if r < 0x80 {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}
	return unicode.ToLower(r)
}

// needle is the query as runes plus its KMP failure table, in fixed arrays so
// setting one allocates nothing.
type needle struct {
	rs   [MaxNeedle]rune
	n    int
	fail [MaxNeedle]int
	fold bool // case-insensitive: both sides go through fold
}

// set takes the first MaxNeedle runes of text and reports whether the needle changed.
func (nd *needle) set(text string, matchCase bool) bool {
	var rs [MaxNeedle]rune
	n := 0
	for _, r := range text {
		if n == MaxNeedle {
			break
		}
		if !matchCase {
			r = fold(r)
		}
		rs[n] = r
		n++
	}
	if n == nd.n && nd.fold == !matchCase && sameRunes(rs[:n], nd.rs[:n]) {
		return false
	}
	nd.rs, nd.n, nd.fold = rs, n, !matchCase
	// KMP failure table: fail[i] is the length of the longest proper prefix of
	// rs[:i+1] that is also its suffix.
	if n > 0 {
		nd.fail[0] = 0
	}
	k := 0
	for i := 1; i < n; i++ {
		for k > 0 && nd.rs[i] != nd.rs[k] {
			k = nd.fail[k-1]
		}
		if nd.rs[i] == nd.rs[k] {
			k++
		}
		nd.fail[i] = k
	}
	return true
}

func sameRunes(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// matcher streams cells through the needle's KMP automaton. It remembers the
// start and end position of the last n cells in a ring, so a full match knows
// where it began and which cells (rows) it covered.
type matcher struct {
	nd   *needle
	k    int // runes of the needle matched so far
	w    int // next ring slot to write; after a match, the match's first cell
	at   [MaxNeedle]Pos
	ends [MaxNeedle]Pos
}

// reset forgets any partial match: a hard line end, or a new needle.
func (m *matcher) reset() { m.k, m.w = 0, 0 }

// feed advances over one cell holding r, spanning start..end. On a full match it
// returns it and starts over (matches never overlap, left to right: Ruling 4).
func (m *matcher) feed(r rune, start, end Pos) (Match, bool) {
	nd := m.nd
	n := nd.n
	if n == 0 {
		return Match{}, false
	}
	if nd.fold {
		r = fold(r)
	}
	m.at[m.w], m.ends[m.w] = start, end
	m.w++
	if m.w == n {
		m.w = 0
	}
	k := m.k
	for k > 0 && r != nd.rs[k] {
		k = nd.fail[k-1]
	}
	if r == nd.rs[k] {
		k++
	}
	if k < n {
		m.k = k
		return Match{}, false
	}
	m.k = 0
	return Match{Start: m.at[m.w], End: end}, true
}

// cell returns the j-th cell (0 = first) of the match feed just returned; valid
// until the next feed.
func (m *matcher) cell(j int) (start, end Pos) {
	i := m.w + j
	if i >= m.nd.n {
		i -= m.nd.n
	}
	return m.at[i], m.ends[i]
}
