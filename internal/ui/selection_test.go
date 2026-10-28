package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/pointer"

	"github.com/patrick-gogo/kaze/internal/render"
	"github.com/patrick-gogo/kaze/internal/screen"
)

// feed prints s, with '\n' as CR LF, like a shell's output.
func feed(scr *screen.Screen, s string) {
	for _, r := range s {
		if r == '\n' {
			scr.CarriageReturn()
			scr.LineFeed()
			continue
		}
		scr.Print(r)
	}
}

func pt(col, row int) render.Point { return render.Point{Col: col, Row: row} }

func TestAbsPointUsesTheViewBase(t *testing.T) {
	for _, tc := range []struct {
		p    image.Point
		want render.Point
	}{
		{image.Pt(25, 33), pt(2, 102)},
		{image.Pt(-40, -5), pt(0, 100)},      // left of and above the grid clamp to its edge
		{image.Pt(5000, 5000), pt(79, 123)},  // past the bottom-right clamps too
		{image.Pt(799, 383), pt(79, 123)},    // last pixel of the last cell
		{image.Pt(800, 384), pt(79, 123)},    // one past it
		{image.Pt(10, 16), pt(1, 101)},       // cell borders belong to the next cell
		{image.Pt(9, 15), pt(0, 100)},        // and not to the one before
		{image.Pt(0, 0), pt(0, 100)},         // origin
		{image.Pt(400, 200), pt(40, 100+12)}, // middle
	} {
		if got := absPoint(tc.p, 10, 16, 80, 24, 100); got != tc.want {
			t.Errorf("absPoint(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}
	if got := absPoint(image.Pt(5, 5), 0, 0, 80, 24, 7); got != pt(0, 7) {
		t.Errorf("no cell size yet = %v, want the base row", got)
	}
}

func TestClicksCountDoubleAndTriple(t *testing.T) {
	var c clicks
	t0 := 10 * time.Second
	steps := []struct {
		at   time.Duration
		p    render.Point
		want int
	}{
		{t0, pt(3, 4), 1},
		{t0 + 100*time.Millisecond, pt(3, 4), 2},
		{t0 + 200*time.Millisecond, pt(3, 4), 3},
		{t0 + 300*time.Millisecond, pt(3, 4), 1},                // a fourth click starts over
		{t0 + 2*time.Second, pt(3, 4), 1},                       // too slow
		{t0 + 2*time.Second + 50*time.Millisecond, pt(9, 4), 1}, // another cell
		{t0 + 2*time.Second + 100*time.Millisecond, pt(9, 4), 2},
	}
	for i, s := range steps {
		if got := c.press(s.at, s.p); got != s.want {
			t.Fatalf("step %d: press = %d, want %d", i, got, s.want)
		}
	}
}

func TestPressAndReleaseReportTheButtonThatChanged(t *testing.T) {
	P, M, S := pointer.ButtonPrimary, pointer.ButtonTertiary, pointer.ButtonSecondary
	for _, tc := range []struct {
		name      string
		prev, now pointer.Buttons
		press     bool
		want      int
	}{
		{"left down", 0, P, true, 0},
		{"right down while left held", P, P | S, true, 2},
		{"middle down", 0, M, true, 1},
		{"right up", S, 0, false, 2},
		{"middle up", M, 0, false, 1},
		{"right up while left held", P | S, P, false, 2},
		{"left up while right held", P | S, S, false, 0},
		{"nothing changed", P, P, false, -1},
	} {
		var got int
		if tc.press {
			got = pressedButton(tc.prev, tc.now)
		} else {
			got = releasedButton(tc.prev, tc.now)
		}
		if got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestEdgeDirection(t *testing.T) {
	for _, tc := range []struct{ y, want int }{{-1, -1}, {-300, -1}, {0, 0}, {383, 0}, {384, 1}, {9000, 1}} {
		if got := edgeDir(tc.y, 384); got != tc.want {
			t.Errorf("edgeDir(%d) = %d, want %d", tc.y, got, tc.want)
		}
	}
}

func TestSingleClickClearsAndDragSelects(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello world")
	var ts tabSel
	ts.sel = render.Selection{Start: pt(0, 0), End: pt(4, 0), Active: true}
	ts.begin(scr, pt(2, 0), 1)
	if ts.sel.Active {
		t.Fatal("a single click left the old selection")
	}
	ts.extend(scr, pt(2, 0))
	if ts.sel.Active {
		t.Fatal("a drag that never left the pressed cell selected something")
	}
	ts.extend(scr, pt(7, 0))
	if got := render.SelectedText(scr, ts.sel); got != "llo wo" {
		t.Fatalf("drag selected %q, want %q", got, "llo wo")
	}
	ts.extend(scr, pt(0, 0)) // back past the anchor
	if got := render.SelectedText(scr, ts.sel); got != "hel" {
		t.Fatalf("drag left selected %q, want %q", got, "hel")
	}
}

func TestDoubleClickSelectsWordsAndDragExtendsByWords(t *testing.T) {
	scr := screen.NewWithScrollback(30, 3, 10)
	feed(scr, "one two three four")
	var ts tabSel
	ts.begin(scr, pt(5, 0), 2)
	if got := render.SelectedText(scr, ts.sel); got != "two" {
		t.Fatalf("double-click selected %q, want two", got)
	}
	ts.extend(scr, pt(10, 0)) // inside "three"
	if got := render.SelectedText(scr, ts.sel); got != "two three" {
		t.Fatalf("word drag selected %q, want %q", got, "two three")
	}
	ts.extend(scr, pt(1, 0)) // back into "one"
	if got := render.SelectedText(scr, ts.sel); got != "one two" {
		t.Fatalf("word drag left selected %q, want %q", got, "one two")
	}
}

func TestTripleClickSelectsLinesAndDragExtendsByLines(t *testing.T) {
	scr := screen.NewWithScrollback(20, 4, 10)
	feed(scr, "first\nsecond\nthird")
	var ts tabSel
	ts.begin(scr, pt(2, 1), 3)
	if got := render.SelectedText(scr, ts.sel); got != "second" {
		t.Fatalf("triple-click selected %q, want second", got)
	}
	ts.extend(scr, pt(0, 2))
	if got := render.SelectedText(scr, ts.sel); got != "second\nthird" {
		t.Fatalf("line drag selected %q", got)
	}
}

// selectWord makes a finished selection of the word at p.
func selectWord(scr *screen.Screen, p render.Point) *tabSel {
	ts := &tabSel{}
	ts.begin(scr, p, 2)
	ts.end()
	return ts
}

func TestCheckClearsOnAltScreenSwitch(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello")
	ts := selectWord(scr, pt(1, 0))
	scr.UseAltScreen(true)
	ts.check(scr)
	if ts.sel.Active {
		t.Fatal("selection survived entering the alt screen")
	}
}

func TestCheckClearsOnResize(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello")
	ts := selectWord(scr, pt(1, 0))
	scr.Resize(30, 3)
	ts.check(scr)
	if ts.sel.Active {
		t.Fatal("selection survived a resize")
	}
}

func TestCheckClearsWhenTheSelectedTextChanges(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello")
	ts := selectWord(scr, pt(1, 0))
	scr.SetCursor(1, 0)
	feed(scr, "a") // hallo
	ts.check(scr)
	if ts.sel.Active {
		t.Fatal("selection survived its text changing")
	}
}

func TestCheckKeepsTheSelectionOnOtherOutput(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello world\n")
	ts := selectWord(scr, pt(1, 0))
	feed(scr, "more output") // row 1
	scr.SetCursor(8, 0)
	feed(scr, "w") // changes "world" outside the selection
	ts.check(scr)
	if got := render.SelectedText(scr, ts.sel); got != "hello" {
		t.Fatalf("selection = %q after unrelated output, want hello", got)
	}
}

func TestCheckKeepsTheSelectionWhenItScrollsIntoHistory(t *testing.T) {
	scr := screen.NewWithScrollback(20, 3, 10)
	feed(scr, "hello\n")
	ts := selectWord(scr, pt(1, 0))
	feed(scr, "a\nb\nc\nd") // row 0 scrolls off the top into history
	if scr.ScrollbackLen() == 0 {
		t.Fatal("test setup: nothing scrolled into history")
	}
	ts.check(scr)
	if got := render.SelectedText(scr, ts.sel); got != "hello" {
		t.Fatalf("selection = %q after scrolling into history, want hello", got)
	}
}

func TestCheckShiftsRowsByEvictedLines(t *testing.T) {
	scr := screen.NewWithScrollback(10, 2, 3)
	feed(scr, "l0\nl1\nl2\nl3\nkeep\n") // history l0 l1 l2, ring full; live: l3 keep
	if scr.ScrollbackLen() != 3 {
		t.Fatalf("test setup: history %d, want 3", scr.ScrollbackLen())
	}
	ts := selectWord(scr, pt(0, 3)) // "keep", absolute row 3
	if got := render.SelectedText(scr, ts.sel); got != "keep" {
		t.Fatalf("test setup: selected %q", got)
	}
	feed(scr, "x\n") // pushes l3 in and evicts l0... keep moves to history
	ts.check(scr)
	if got := render.SelectedText(scr, ts.sel); got != "keep" {
		t.Fatalf("after eviction the selection reads %q, want keep (sel %+v)", got, ts.sel)
	}
	feed(scr, "a\nb\nc\nd\ne\n") // keep falls off the top of the ring
	ts.check(scr)
	if ts.sel.Active {
		t.Fatalf("selection survived its line being evicted: %+v reads %q", ts.sel, render.SelectedText(scr, ts.sel))
	}
}

func TestCheckClampsASelectionPartlyEvicted(t *testing.T) {
	scr := screen.NewWithScrollback(10, 2, 2)
	feed(scr, "aa\nbb\ncc\n") // history aa bb; live cc, blank
	ts := &tabSel{}
	ts.begin(scr, pt(0, 0), 1)
	ts.extend(scr, pt(1, 2)) // aa..cc
	ts.end()
	feed(scr, "dd\n") // evicts aa
	ts.check(scr)
	if got := render.SelectedText(scr, ts.sel); got != "bb\ncc" {
		t.Fatalf("partly evicted selection reads %q, want the surviving lines", got)
	}
}
