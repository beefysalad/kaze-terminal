package find

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/screen"
)

func warnLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d warn", i)
	}
	return lines
}

func fresh(t *testing.T, scr *screen.Screen, text string) *Search {
	t.Helper()
	s := &Search{}
	s.SetQuery(text, false)
	settle(t, s, scr)
	return s
}

func TestOutputRescansOnlyLiveRows(t *testing.T) {
	scr := screenWith(80, 24, 10_000, warnLines(10_030)...)
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	before := s.Scanned()
	printTo(scr, "\nnew warn")
	s.Output(scr)
	settle(t, &s, scr)
	cols, rows := scr.Size()
	back := s.nd.n + 1
	if got, limit := s.Scanned()-before, (rows+back+1)*cols; got > limit {
		t.Fatalf("output rescan read %d cells, want <= %d", got, limit)
	}
	if want := fresh(t, scr, "warn"); !reflect.DeepEqual(s.Matches(), want.Matches()) {
		t.Fatal("incremental matches differ from a fresh search")
	}
}

func TestOutputFindsNewMatches(t *testing.T) {
	scr := screenWith(20, 5, 100, warnLines(30)...)
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	n, g := len(s.Matches()), s.Gen()
	printTo(scr, "\nwarn again")
	s.Output(scr)
	settle(t, &s, scr)
	if got := len(s.Matches()); got != n+1 {
		t.Fatalf("%d matches after output, want %d", got, n+1)
	}
	if s.Gen() == g {
		t.Fatal("Gen did not bump")
	}
}

func TestOutputDropsOverwrittenMatch(t *testing.T) {
	scr := screenWith(20, 5, 100, "a", "b", "warn here")
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	if len(s.Matches()) != 1 {
		t.Fatalf("setup: %d matches", len(s.Matches()))
	}
	scr.EraseInLine(2)
	scr.CarriageReturn()
	printTo(scr, "ok")
	s.Output(scr)
	settle(t, &s, scr)
	if n := len(s.Matches()); n != 0 {
		t.Fatalf("%d matches after the line was overwritten", n)
	}
}

func TestEvictionDropsOnlyOldMatches(t *testing.T) {
	lines := make([]string, 105)
	for i := range lines {
		lines[i] = fmt.Sprintf("l%d", i)
	}
	for _, i := range []int{10, 50, 90} {
		lines[i] = fmt.Sprintf("warn %d", i)
	}
	scr := screenWith(20, 5, 100, lines...)
	if scr.Evicted() != 0 {
		t.Fatalf("setup: evicted %d", scr.Evicted())
	}
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	if got := len(s.Matches()); got != 3 {
		t.Fatalf("setup: %d matches", got)
	}
	printTo(scr, strings.Repeat("\nmore", 30))
	s.Output(scr)
	settle(t, &s, scr)
	ms := s.Matches()
	if len(ms) != 2 || ms[0].Start.Line != 50 || ms[1].Start.Line != 90 {
		t.Fatalf("matches %+v, want lines 50 and 90", ms)
	}
	for _, m := range ms {
		if got, want := lineText(scr, m.Start.Line), fmt.Sprintf("warn %d", m.Start.Line); got != want {
			t.Fatalf("line %d holds %q, want %q", m.Start.Line, got, want)
		}
	}
}

func TestClearScrollbackDropsHistoryMatches(t *testing.T) {
	scr := screenWith(20, 3, 100, "warn 0", "x", "x", "x", "warn 4", "x")
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	if len(s.Matches()) != 2 {
		t.Fatalf("setup: %d matches", len(s.Matches()))
	}
	scr.EraseInDisplay(3)
	s.Output(scr)
	settle(t, &s, scr)
	ms := s.Matches()
	if len(ms) != 1 || lineText(scr, ms[0].Start.Line) != "warn 4" {
		t.Fatalf("matches %+v after ED 3", ms)
	}
}

func TestResizeAndAltRestart(t *testing.T) {
	scr := screenWith(80, 24, 10_000, warnLines(1000)...)
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	before := s.Scanned()
	scr.Resize(70, 24)
	s.Output(scr)
	settle(t, &s, scr)
	full := fresh(t, scr, "warn")
	if got := s.Scanned() - before; got < full.Scanned() {
		t.Fatalf("resize rescan read %d cells, a full pass reads %d", got, full.Scanned())
	}
	if !reflect.DeepEqual(s.Matches(), full.Matches()) {
		t.Fatal("matches after resize differ from a fresh search")
	}

	scr.UseAltScreen(true)
	printTo(scr, "warn on alt")
	before = s.Scanned()
	s.Output(scr)
	settle(t, &s, scr)
	full = fresh(t, scr, "warn")
	if got := s.Scanned() - before; got < full.Scanned() {
		t.Fatalf("alt rescan read %d cells, a full pass reads %d", got, full.Scanned())
	}
	if len(s.Matches()) != 1 {
		t.Fatalf("%d matches on the alt screen, want 1", len(s.Matches()))
	}
	scr.UseAltScreen(false)
	s.Output(scr)
	settle(t, &s, scr)
	if got, want := len(s.Matches()), len(fresh(t, scr, "warn").Matches()); got != want || got < 1000 {
		t.Fatalf("%d matches back on the main screen, want %d", got, want)
	}
}

// TestIncrementalEqualsFresh streams random output (wraps, wide chars, erases, a
// small evicting ring) with Output and small Steps in between; at the end the
// incremental search must equal a fresh one.
func TestIncrementalEqualsFresh(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	words := []string{"warn", "wa", "rn", "日本", "wa\nrn", "x", " ", "ab日", "warn日本warn"}
	for _, needle := range []string{"warn", "日本", "rnw", "a b"} {
		scr := screen.NewWithScrollback(9, 4, 50)
		var s Search
		s.SetQuery(needle, false)
		for i := 0; i < 400; i++ {
			switch k := rng.Intn(20); {
			case k == 0:
				printTo(scr, "\n")
			case k == 1:
				scr.EraseInLine(2)
			case k == 2:
				scr.SetCursor(rng.Intn(9), rng.Intn(4))
			default:
				printTo(scr, words[rng.Intn(len(words))])
			}
			if rng.Intn(3) == 0 {
				s.Output(scr)
				s.Step(scr, 1+rng.Intn(40))
			}
		}
		s.Output(scr)
		settle(t, &s, scr)
		want := fresh(t, scr, needle)
		if !reflect.DeepEqual(s.Matches(), want.Matches()) {
			t.Fatalf("%q: incremental %+v\nfresh %+v", needle, s.Matches(), want.Matches())
		}
		if !reflect.DeepEqual(s.Spans(), want.Spans()) {
			t.Fatalf("%q: spans differ", needle)
		}
	}
}

// A match from the last history row wrapped into live row 0: changing the live
// part must drop it, and making one there must find it.
func TestOutputRechecksMatchesIntoTheLiveRows(t *testing.T) {
	scr := screenWith(5, 3, 100, "1", "2", "xxxwarn", "tail", "end")
	if scr.ScrollbackLen() != 3 || scr.Cell(0, 0).Rune != 'r' {
		t.Fatalf("setup: history %d, live row 0 starts %q", scr.ScrollbackLen(), scr.Cell(0, 0).Rune)
	}
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	if len(s.Matches()) != 1 {
		t.Fatalf("setup: %d matches", len(s.Matches()))
	}
	scr.SetCursor(0, 0)
	printTo(scr, "XX")
	s.Output(scr)
	settle(t, &s, scr)
	if n := len(s.Matches()); n != 0 {
		t.Fatalf("%d matches after the live half changed", n)
	}
	scr.SetCursor(0, 0)
	printTo(scr, "rn")
	s.Output(scr)
	settle(t, &s, scr)
	if n := len(s.Matches()); n != 1 {
		t.Fatalf("%d matches after the live half came back", n)
	}
}

// A kept match that runs past where the rescan starts: the rescan must not find a
// match overlapping it (matches never overlap, left to right).
func TestOutputResumesAfterKeptMatch(t *testing.T) {
	scr := screenWith(2, 3, 100, "x"+strings.Repeat("a", 40))
	var s Search
	s.SetQuery("aaaa", false)
	settle(t, &s, scr)
	printTo(scr, "\nb")
	s.Output(scr)
	settle(t, &s, scr)
	want := fresh(t, scr, "aaaa")
	if !reflect.DeepEqual(s.Matches(), want.Matches()) {
		t.Fatalf("incremental %+v\nfresh %+v", s.Matches(), want.Matches())
	}
}

func TestOutputMidScanStillSettles(t *testing.T) {
	scr := screenWith(100, 10, 10_000, longLines(5000, 100)...)
	var s Search
	s.SetQuery("ba", false)
	settled := false
	for i := 0; i < 200 && !settled; i++ {
		if i < 100 { // output keeps arriving for a while, every step
			printTo(scr, "\nwarn ba")
			s.Output(scr)
		}
		settled = s.Step(scr, 50_000)
	}
	if !settled {
		t.Fatal("search never settled under streaming output")
	}
	if want := fresh(t, scr, "ba"); !reflect.DeepEqual(s.Matches(), want.Matches()) {
		t.Fatal("matches differ from a fresh search")
	}
}

// pointsScreen holds "x" at the start of lines 2, 5 and 9 of 12 (nothing evicted).
func pointsSearch(t *testing.T) *Search {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = "."
	}
	for _, i := range []int{2, 5, 9} {
		lines[i] = "x"
	}
	return fresh(t, screenWith(10, 5, 100, lines...), "x")
}

func TestNearest(t *testing.T) {
	s := pointsSearch(t)
	cases := []struct{ top, bottom, want int }{
		{4, 6, 1},   // in view
		{2, 9, 0},   // several in view: the first
		{6, 8, 1},   // none in view: the one just above
		{0, 1, 0},   // only below: the first below
		{10, 11, 2}, // only above
	}
	for _, c := range cases {
		if got := s.Nearest(c.top, c.bottom); got != c.want {
			t.Errorf("Nearest(%d, %d) = %d, want %d", c.top, c.bottom, got, c.want)
		}
	}
	var empty Search
	if got := empty.Nearest(0, 10); got != -1 {
		t.Errorf("empty Nearest = %d", got)
	}
}

func TestLocate(t *testing.T) {
	s := pointsSearch(t)
	cases := []struct {
		p    Pos
		want int
	}{
		{Pos{5, 0}, 1},  // exact
		{Pos{3, 0}, 1},  // between: the next
		{Pos{5, 1}, 2},  // just past a start
		{Pos{0, 0}, 0},  // before all
		{Pos{20, 0}, 2}, // past the end: the last
	}
	for _, c := range cases {
		if got := s.Locate(c.p); got != c.want {
			t.Errorf("Locate(%+v) = %d, want %d", c.p, got, c.want)
		}
	}
	var empty Search
	if got := empty.Locate(Pos{}); got != -1 {
		t.Errorf("empty Locate = %d", got)
	}
}

func TestOutputWithEmptyNeedleDoesNothing(t *testing.T) {
	scr := screenWith(20, 5, 100, warnLines(30)...)
	var s Search
	s.SetQuery("", false)
	s.Output(scr)
	s.Step(scr, StepBudget)
	if s.Scanned() != 0 || len(s.Matches()) != 0 {
		t.Fatalf("empty needle read %d cells", s.Scanned())
	}
}

func TestOutputAllocatesNothing(t *testing.T) {
	// Every line has the same shape, so the screen's own ring reuses its storage and
	// only the search is measured.
	lines := make([]string, 2000)
	for i := range lines {
		lines[i] = "line warn"
	}
	scr := screenWith(80, 24, 1000, lines...)
	var s Search
	s.SetQuery("warn", false)
	settle(t, &s, scr)
	s.Matches()
	allocs := testing.AllocsPerRun(20, func() {
		printTo(scr, "\nline warn")
		s.Output(scr)
		for n := 0; n < 64 && !s.Step(scr, StepBudget); n++ {
		}
		s.Matches()
		s.Spans()
		s.Nearest(100, 120)
		s.Locate(Pos{500, 0})
	})
	if allocs != 0 {
		t.Fatalf("output refresh allocates %v per run", allocs)
	}
}
