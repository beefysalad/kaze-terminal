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
	start := time.Now()
	out := input.EncodePaste(s, m)
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("took %v", d)
	}
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
		s := strings.Repeat(unit, (1<<20)/len(unit))
		start := time.Now()
		out := input.EncodePaste(s, m)
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Fatalf("%q took %v", unit, d)
		}
		body := string(out[6 : len(out)-6])
		if strings.Contains(body, "\x1b[201~") || strings.Contains(body, "\x1b[200~") {
			t.Fatalf("%q: marker left in payload", unit)
		}
	}
}

func TestPasteStripsStartMarker(t *testing.T) {
	chk(t, input.EncodePaste("a\x1b[200~b", term.Modes{BracketedPaste: true}), "\x1b[200~ab\x1b[201~")
}
