package screen

import "testing"

func TestText(t *testing.T) {
	for _, tc := range []struct {
		name, printed string
		lines         int
		alt           bool
		want          string
	}{
		{"wrapped", "abcdef", 0, false, "abcdef"},
		{"wrapped spaces", "ab   cd", 0, false, "ab   cd"},
		{"trim", "ab  ", 0, false, "ab\n"},
		{"trim wrapped tail", "ab    ", 0, false, "ab"},
		{"wide", "界界xy", 0, false, "界界xy"},
		{"wide wrap padding", "abcd界", 0, false, "abcd界"},
		{"wide padding history boundary", "abcd界\r\nz", 1, false, "abcd界\nz"},
		{"spaces history boundary", "ab   cd\r\nz", 1, false, "ab   cd\nz"},
		{"visible", "one\r\ntwo\r\nthree", 0, false, "two\nthree"},
		{"history", "one\r\ntwo\r\nthree", 1, false, "one\ntwo\nthree"},
		{"negative", "one\r\ntwo\r\nthree", -1, false, "two\nthree"},
		{"alt", "one\r\ntwo\r\nthree", 20, true, "alt\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(5, 2)
			for _, r := range tc.printed {
				switch r {
				case '\r':
					s.CarriageReturn()
				case '\n':
					s.LineFeed()
				default:
					s.Print(r)
				}
			}
			if tc.alt {
				s.UseAltScreen(true)
				s.SetCursor(0, 0)
				for _, r := range "alt" {
					s.Print(r)
				}
			}
			if got := Text(s, tc.lines); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
