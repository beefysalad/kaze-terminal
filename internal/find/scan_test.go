package find

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFindsInHistoryAndScreen(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("l%02d", i))
	}
	scr := screenWith(5, 3, 100, lines...)
	var s Search
	s.SetQuery("l1", false)
	settle(t, &s, scr)
	ms := s.Matches()
	if len(ms) != 10 {
		t.Fatalf("%d matches, want 10", len(ms))
	}
	for i, m := range ms {
		if i > 0 && m.Start.Line <= ms[i-1].Start.Line {
			t.Fatalf("matches out of order: %+v", ms)
		}
		if got := lineText(scr, m.Start.Line); got != fmt.Sprintf("l%02d", 10+i) {
			t.Fatalf("match %d on %q", i, got)
		}
	}
	if ms[0].Start.Line >= scr.ScrollbackLen() {
		t.Fatalf("first match on line %d, want history (< %d)", ms[0].Start.Line, scr.ScrollbackLen())
	}
	if s.Capped() {
		t.Fatal("capped")
	}
}

func TestMatchSpansSoftWrap(t *testing.T) {
	for _, pushed := range []bool{false, true} {
		scr := screenWith(10, 3, 100, "0123456789abc")
		if pushed { // the wrapped pair now sits in history
			printTo(scr, "\n\n\n\n")
		}
		var s Search
		s.SetQuery("89ab", false)
		settle(t, &s, scr)
		want := []Match{{Start: Pos{0, 8}, End: Pos{1, 1}}}
		if got := s.Matches(); !reflect.DeepEqual(got, want) {
			t.Fatalf("pushed=%v: matches %+v, want %+v", pushed, got, want)
		}
		wantSp := []Span{{Line: 0, From: 8, To: 9}, {Line: 1, From: 0, To: 1}}
		if got := s.Spans(); !reflect.DeepEqual(got, wantSp) {
			t.Fatalf("pushed=%v: spans %+v, want %+v", pushed, got, wantSp)
		}
	}
}

func TestMatchSpansManyRows(t *testing.T) {
	scr := screenWith(3, 4, 100, "xabcdefgy", "abc")
	var s Search
	s.SetQuery("abcdefg", false)
	settle(t, &s, scr)
	want := []Span{{Line: 0, From: 1, To: 2}, {Line: 1, From: 0, To: 2}, {Line: 2, From: 0, To: 1}}
	if got := s.Spans(); !reflect.DeepEqual(got, want) {
		t.Fatalf("spans %+v, want %+v", got, want)
	}
	s.SetQuery("abc", false)
	settle(t, &s, scr)
	want = []Span{{Line: 0, From: 1, To: 2}, {Line: 1, From: 0, To: 0}, {Line: 3, From: 0, To: 2, Match: 1}}
	if got := s.Spans(); !reflect.DeepEqual(got, want) {
		t.Fatalf("abc spans %+v, want %+v", got, want)
	}
}

func TestHardLineEndDoesNotJoin(t *testing.T) {
	scr := screenWith(10, 3, 100, "0123456789", "abc")
	var s Search
	s.SetQuery("9a", false)
	settle(t, &s, scr)
	if n := len(s.Matches()); n != 0 {
		t.Fatalf("%d matches across a hard line end", n)
	}
}

func TestWideCharacters(t *testing.T) {
	scr := screenWith(20, 3, 100, "日本語abc")
	cases := []struct {
		needle string
		want   Match
	}{
		{"本語", Match{Pos{0, 2}, Pos{0, 5}}},
		{"語a", Match{Pos{0, 4}, Pos{0, 6}}},
	}
	for _, c := range cases {
		var s Search
		s.SetQuery(c.needle, false)
		settle(t, &s, scr)
		if got := s.Matches(); len(got) != 1 || got[0] != c.want {
			t.Fatalf("%q: %+v, want %+v", c.needle, got, c.want)
		}
	}
	// A wide char that did not fit leaves a pad blank at the edge; it is not text.
	scr = screenWith(9, 3, 100, "12345678日本")
	var s Search
	s.SetQuery("8日", false)
	settle(t, &s, scr)
	want := Match{Pos{0, 7}, Pos{1, 1}}
	if got := s.Matches(); len(got) != 1 || got[0] != want {
		t.Fatalf("8日 across the wrap: %+v, want %+v", got, want)
	}
	wantSp := []Span{{Line: 0, From: 7, To: 7}, {Line: 1, From: 0, To: 1}}
	if got := s.Spans(); !reflect.DeepEqual(got, wantSp) {
		t.Fatalf("spans %+v, want %+v", got, wantSp)
	}
}

func TestTrailingBlanksAreNotText(t *testing.T) {
	scr := screenWith(10, 3, 100, "ab", "")
	var s Search
	s.SetQuery("b ", false)
	settle(t, &s, scr)
	if n := len(s.Matches()); n != 0 {
		t.Fatalf("%d matches on trailing blanks", n)
	}
}

func TestAltScreenSearchesTheGridOnly(t *testing.T) {
	scr := screenWith(10, 3, 100, "secret", "", "", "", "")
	if scr.ScrollbackLen() == 0 {
		t.Fatal("setup: nothing in history")
	}
	scr.UseAltScreen(true)
	printTo(scr, "shown")
	for needle, want := range map[string]int{"secret": 0, "shown": 1} {
		var s Search
		s.SetQuery(needle, false)
		settle(t, &s, scr)
		if n := len(s.Matches()); n != want {
			t.Fatalf("%q: %d matches, want %d", needle, n, want)
		}
	}
}

func TestStableLinesAfterEviction(t *testing.T) {
	var lines []string
	for i := 0; i < 14; i++ {
		lines = append(lines, fmt.Sprintf("a%02d", i))
	}
	scr := screenWith(10, 2, 5, lines...)
	if scr.Evicted() != 7 {
		t.Fatalf("setup: evicted %d, want 7", scr.Evicted())
	}
	var s Search
	s.SetQuery("a", false)
	settle(t, &s, scr)
	line := -1
	for _, m := range s.Matches() {
		if m.Start.Line < int(scr.Evicted()) {
			t.Fatalf("match on evicted line %d", m.Start.Line)
		}
		if lineText(scr, m.Start.Line) == "a10" {
			line = m.Start.Line
		}
	}
	if line < 0 {
		t.Fatal("no match on a10")
	}
	printTo(scr, "\nb\nc")
	var s2 Search
	s2.SetQuery("a", false)
	settle(t, &s2, scr)
	for _, m := range s2.Matches() {
		if m.Start.Line == line {
			if got := lineText(scr, line); got != "a10" {
				t.Fatalf("line %d now holds %q", line, got)
			}
			return
		}
	}
	t.Fatalf("a10's line %d lost its match", line)
}

func TestCapKeepsNewest(t *testing.T) {
	lines := make([]string, 1500)
	for i := range lines {
		lines[i] = "x"
	}
	scr := screenWith(5, 3, 10_000, lines...)
	var s Search
	s.SetQuery("x", false)
	settle(t, &s, scr)
	ms := s.Matches()
	if len(ms) != MaxMatches {
		t.Fatalf("%d matches, want %d", len(ms), MaxMatches)
	}
	if !s.Capped() {
		t.Fatal("not capped")
	}
	if ms[0].Start.Line != 500 || ms[len(ms)-1].Start.Line != 1499 {
		t.Fatalf("kept lines %d..%d, want 500..1499", ms[0].Start.Line, ms[len(ms)-1].Start.Line)
	}
	if sp := s.Spans(); len(sp) != MaxMatches || sp[0].Line != 500 || sp[0].Match != 0 || sp[999].Match != 999 {
		t.Fatalf("spans %d, first %+v", len(sp), sp[0])
	}
}

func longLines(n, cols int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = strings.Repeat("ab", cols/2)
	}
	return lines
}

func TestStepRespectsBudget(t *testing.T) {
	scr := screenWith(100, 10, 10_000, longLines(2000, 100)...)
	var s Search
	s.SetQuery("ba", false)
	before := s.Scanned()
	if s.Step(scr, 10_000) {
		t.Fatal("settled in one small step")
	}
	if got := s.Scanned() - before; got > 10_000+100 {
		t.Fatalf("one step read %d cells, budget 10000", got)
	}
	steps := 1
	for !s.Step(scr, 10_000) {
		steps++
		if steps > 64 {
			t.Fatal("did not settle in 64 steps")
		}
	}
	var one Search
	one.SetQuery("ba", false)
	if !one.Step(scr, 1<<30) {
		t.Fatal("one big step did not settle")
	}
	if !reflect.DeepEqual(s.Matches(), one.Matches()) || s.Capped() != one.Capped() {
		t.Fatalf("stepped and one-shot searches differ: %d vs %d", len(s.Matches()), len(one.Matches()))
	}
}

func TestSettledStepReadsNothing(t *testing.T) {
	scr := screenWith(10, 3, 100, "abc", "abd")
	var s Search
	s.SetQuery("ab", false)
	settle(t, &s, scr)
	n := s.Scanned()
	if !s.Step(scr, StepBudget) {
		t.Fatal("settled search stepped again")
	}
	if s.Scanned() != n {
		t.Fatalf("settled step read %d cells", s.Scanned()-n)
	}
}

func TestQueryChangeRestarts(t *testing.T) {
	scr := screenWith(10, 3, 100, "a1", "b2", "a3")
	var s Search
	s.SetQuery("a", false)
	settle(t, &s, scr)
	g := s.Gen()
	if !s.SetQuery("b", false) {
		t.Fatal("SetQuery reported no change")
	}
	if s.SetQuery("b", false) {
		t.Fatal("same query reported a change")
	}
	settle(t, &s, scr)
	if s.Gen() == g {
		t.Fatal("Gen did not bump")
	}
	ms := s.Matches()
	if len(ms) != 1 || matchText(scr, ms[0]) != "b" {
		t.Fatalf("matches %+v", ms)
	}
}

func TestEmptyQueryFindsNothing(t *testing.T) {
	scr := screenWith(10, 3, 100, "abc")
	var s Search
	s.SetQuery("", false)
	if !s.Step(scr, StepBudget) {
		t.Fatal("empty query not settled")
	}
	if len(s.Matches()) != 0 || s.Scanned() != 0 {
		t.Fatalf("empty query: %d matches, %d cells", len(s.Matches()), s.Scanned())
	}
}

func TestStepAllocatesNothingAfterWarmup(t *testing.T) {
	scr := screenWith(80, 24, 10_000, longLines(3000, 80)...)
	var s Search
	s.SetQuery("ab", false)
	settle(t, &s, scr)
	s.Matches()
	s.Spans()
	words := [2]string{"ba", "ab"}
	i := 0
	allocs := testing.AllocsPerRun(10, func() {
		s.SetQuery(words[i&1], false)
		i++
		for n := 0; n < 64 && !s.Step(scr, StepBudget); n++ {
		}
		s.Matches()
		s.Spans()
	})
	if allocs != 0 {
		t.Fatalf("a warm search allocates %v per run", allocs)
	}
}
