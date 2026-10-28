package main

import "testing"

func TestStripRemovesQueriesModesAndEveryOSC(t *testing.T) {
	const st = "\x1b\x5c" // ESC backslash, the string terminator
	in := "a\x1b]52;c;aGVsbG8=\x07b\x1b]0;title" + st + "c\x1b]7501;?\x07d\x1bPq#0" + st +
		"e\x1b[?1004h\x1b[>0q\x1b[?u\x1b[1mf\x1b[0m"
	want := "abcde\x1b[1mf\x1b[0m"
	if got := string(strip.ReplaceAll([]byte(in), nil)); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
