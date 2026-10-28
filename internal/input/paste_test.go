package input_test

import (
	"strings"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze/internal/input"
	"github.com/patrick-gogo/kaze/internal/term"
)

func TestBracketedPasteWraps(t *testing.T) {
	chk(t, input.EncodePaste("hi", term.Modes{BracketedPaste: true}), "\x1b[200~hi\x1b[201~")
	chk(t, input.EncodePaste("hi", term.Modes{}), "hi")
}

func TestPasteNormalizesNewlinesAndStripsEndMarker(t *testing.T) {
	chk(t, input.EncodePaste("a\r\nb\x1b[201~c", term.Modes{BracketedPaste: true}), "\x1b[200~a\rbc\x1b[201~")
	chk(t, input.EncodePaste("a\nb", term.Modes{}), "a\rb")
	chk(t, input.EncodePaste("x\x1b[20\x1b[201~1~y", term.Modes{BracketedPaste: true}), "\x1b[200~xy\x1b[201~")
}

// The Session.Write non-blocking half of this test moves to Task 13.
func TestHugePasteIsFastAndAllocatesOneBuffer(t *testing.T) {
	s := strings.Repeat("line of text\n", 90000)[:1<<20]
	m := term.Modes{BracketedPaste: true}
	checkLinear(t, "plain text", "line of text\n", m)
	out := input.EncodePaste(s, m)
	if len(out) < len(s) {
		t.Fatal("short output")
	}
	if n := testing.AllocsPerRun(3, func() { input.EncodePaste(s, m) }); n > 4 {
		t.Fatalf("%v allocs", n)
	}
}

func TestEncodeText(t *testing.T) { chk(t, input.EncodeText("日本"), "日本") }

func TestEncodeFocus(t *testing.T) {
	chk(t, input.EncodeFocus(true, term.Modes{FocusEvents: true}), "\x1b[I")
	chk(t, input.EncodeFocus(false, term.Modes{FocusEvents: true}), "\x1b[O")
	if input.EncodeFocus(true, term.Modes{}) != nil {
		t.Fatal("expected nil")
	}
}

func TestHostilePasteIsLinearAndMarkerFree(t *testing.T) {
	m := term.Modes{BracketedPaste: true}
	for _, unit := range []string{"\x1b[201~", "\x1b[20\x1b[201~1~", "\x1b[200~", "\x1b[20\x1b[200~0~"} {
		checkLinear(t, unit, unit, m)
		s := strings.Repeat(unit, (1<<20)/len(unit))
		out := input.EncodePaste(s, m)
		body := string(out[6 : len(out)-6])
		if strings.Contains(body, "\x1b[201~") || strings.Contains(body, "\x1b[200~") {
			t.Fatalf("%q: marker left in payload", unit)
		}
	}
}

func TestPasteStripsStartMarker(t *testing.T) {
	chk(t, input.EncodePaste("a\x1b[200~b", term.Modes{BracketedPaste: true}), "\x1b[200~ab\x1b[201~")
}

// fastest is the quickest of a few runs, after a warm-up, so one slow
// scheduling slice can't fail the test. A run over the cap ends it early.
func fastest(t *testing.T, s string, m term.Modes) time.Duration {
	t.Helper()
	input.EncodePaste(s, m) // warm-up
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 5; i++ {
		start := time.Now()
		input.EncodePaste(s, m)
		d := time.Since(start)
		best = min(best, d)
		if d > 2*time.Second {
			t.Fatalf("%d bytes took %v", len(s), d)
		}
	}
	return best
}

// checkLinear compares the time for 16 KiB and 64 KiB of unit repeated. Linear
// work grows 4x; quadratic grows 16x and, at these sizes, fails within seconds.
// The slack keeps timer noise on sub-millisecond runs from failing it.
func checkLinear(t *testing.T, name, unit string, m term.Modes) {
	t.Helper()
	small := fastest(t, strings.Repeat(unit, (16<<10)/len(unit)), m)
	big := fastest(t, strings.Repeat(unit, (64<<10)/len(unit)), m)
	if big > 10*small+5*time.Millisecond {
		t.Fatalf("%q: 4x the input took %v vs %v; the work is not linear", name, big, small)
	}
}
