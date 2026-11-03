package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStripRemovesQueriesModesAndEveryOSC(t *testing.T) {
	const st = "\x1b\x5c" // ESC backslash, the string terminator
	in := "a\x1b]52;c;aGVsbG8=\x07b\x1b]0;title" + st + "c\x1b]7501;?\x07d\x1bPq#0" + st +
		"e\x1b[?1004h\x1b[>0q\x1b[?u\x1b[1mf\x1b[0m"
	want := "abcde\x1b[1mf\x1b[0m"
	if got := string(strip.ReplaceAll([]byte(in), nil)); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// emojiRunes is the emoji in s: anything outside the BMP's text blocks the table uses.
func emojiRunes(s string) []rune {
	var out []rune
	for _, r := range s {
		if r >= 0x2300 && r != '│' {
			out = append(out, r)
		}
	}
	return out
}

func TestEmojiLine(t *testing.T) {
	l := string(emojiLine(nil, 7))
	if !strings.Contains(l, "|") || !strings.HasSuffix(l, "\r\n") || !utf8.ValidString(l) {
		t.Fatalf("emojiLine(7) = %q, want a table row ending in CRLF", l)
	}
	if n := len(emojiRunes(l)); n != 1 {
		t.Fatalf("emojiLine(7) = %q holds %d emoji, want 1", l, n)
	}
	seen := map[rune]bool{}
	for i := range 100 {
		for _, r := range emojiRunes(string(emojiLine(nil, i))) {
			seen[r] = true
		}
	}
	if len(seen) < 15 {
		t.Fatalf("100 lines cover %d distinct emoji, want at least 15", len(seen))
	}
}
