package find

import (
	"strings"
	"testing"
)

// feedString feeds s rune by rune as one-cell positions on line 0 and returns the matches.
func feedString(m *matcher, s string) []Match {
	var out []Match
	col := 0
	for _, r := range s {
		p := Pos{Line: 0, Col: col}
		if mt, ok := m.feed(r, p, p); ok {
			out = append(out, mt)
		}
		col++
	}
	return out
}

func newMatcher(text string, matchCase bool) (*needle, *matcher) {
	nd := &needle{}
	nd.set(text, matchCase)
	return nd, &matcher{nd: nd}
}

func startCols(ms []Match) []int {
	var cols []int
	for _, m := range ms {
		cols = append(cols, m.Start.Col)
	}
	return cols
}

func TestNeedleFoldsCase(t *testing.T) {
	for _, hay := range []string{"warn", "Warn", "WARN"} {
		_, m := newMatcher("WARN", false)
		if got := feedString(m, hay); len(got) != 1 {
			t.Errorf("insensitive %q: %d matches, want 1", hay, len(got))
		}
	}
	for hay, want := range map[string]int{"warn": 0, "Warn": 0, "WARN": 1} {
		_, m := newMatcher("WARN", true)
		if got := feedString(m, hay); len(got) != want {
			t.Errorf("case-sensitive %q: %d matches, want %d", hay, len(got), want)
		}
	}
}

func TestNeedleUnicodeLower(t *testing.T) {
	cases := []struct {
		needle, hay string
		want        int
	}{
		{"ÄRGER", "ärger", 1},
		{"Σ", "σ", 1},
		{"ß", "SS", 0}, // simple lowercase only (Ruling 3)
	}
	for _, c := range cases {
		_, m := newMatcher(c.needle, false)
		if got := feedString(m, c.hay); len(got) != c.want {
			t.Errorf("%q in %q: %d matches, want %d", c.needle, c.hay, len(got), c.want)
		}
	}
}

func TestMatchesDoNotOverlap(t *testing.T) {
	_, m := newMatcher("aa", false)
	got := feedString(m, "aaaa")
	if cols := startCols(got); len(cols) != 2 || cols[0] != 0 || cols[1] != 2 {
		t.Fatalf("starts = %v, want [0 2]", cols)
	}
	if got[0].End.Col != 1 || got[1].End.Col != 3 {
		t.Fatalf("ends = %+v", got)
	}
}

func TestKMPRestarts(t *testing.T) {
	_, m := newMatcher("abab", false)
	if cols := startCols(feedString(m, "abaabab")); len(cols) != 1 || cols[0] != 3 {
		t.Fatalf("abab: starts = %v, want [3]", cols)
	}
	_, m = newMatcher("aab", false)
	if cols := startCols(feedString(m, "aaab")); len(cols) != 1 || cols[0] != 1 {
		t.Fatalf("aab: starts = %v, want [1]", cols)
	}
}

func TestResetBreaksAMatch(t *testing.T) {
	_, m := newMatcher("ab", false)
	n := len(feedString(m, "a"))
	m.reset()
	n += len(feedString(m, "b"))
	if n != 0 {
		t.Fatalf("%d matches across a reset, want 0", n)
	}
}

func TestNeedleCapped(t *testing.T) {
	text := strings.Repeat("x", 300)
	nd := &needle{}
	if !nd.set(text, false) {
		t.Fatal("first set reported no change")
	}
	if nd.n != MaxNeedle {
		t.Fatalf("n = %d, want %d", nd.n, MaxNeedle)
	}
	if nd.set(text, false) {
		t.Fatal("same text reported a change")
	}
	if !nd.set(text, true) {
		t.Fatal("case change reported no change")
	}
}

func TestEmptyNeedleMatchesNothing(t *testing.T) {
	nd, m := newMatcher("", false)
	if nd.n != 0 {
		t.Fatalf("n = %d", nd.n)
	}
	if got := feedString(m, "anything at all"); len(got) != 0 {
		t.Fatalf("%d matches", len(got))
	}
}

func TestNeedleSetAllocatesNothing(t *testing.T) {
	nd := &needle{}
	texts := [2]string{strings.Repeat("abcd", 10), strings.Repeat("wxyz", 10)}
	i := 0
	allocs := testing.AllocsPerRun(50, func() {
		nd.set(texts[i&1], false)
		i++
	})
	if allocs != 0 {
		t.Fatalf("set allocates %v per run", allocs)
	}
}
