package term

import (
	"io"
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/screen"
	"github.com/patrick-gogo/kaze-terminal/internal/vt"
)

func TestOSC7Unix(t *testing.T) {
	got, ok := parseOSC7([]byte("7;file://mac/Users/p/My%20Code"), "linux")
	if !ok || got != "/Users/p/My Code" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestOSC7Windows(t *testing.T) {
	got, ok := parseOSC7([]byte("7;file://PC/C:/Users/p/src"), "windows")
	if !ok || got != `C:\Users\p\src` {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestOSC7DecodesPercentAndUTF8(t *testing.T) {
	got, ok := parseOSC7([]byte("7;file://h/%E6%97%A5%E6%9C%AC"), "linux")
	if !ok || got != "/日本" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestOSC7RejectsNonFile(t *testing.T) {
	for _, in := range []string{"7;http://x/y", "7;file://h", "7;", "7;file://h/%zz", "7;file://h/a%00b"} {
		if got, ok := parseOSC7([]byte(in), "linux"); ok {
			t.Fatalf("%q accepted as %q", in, got)
		}
	}
}

func TestOSC7CapsLength(t *testing.T) {
	long := "7;file://h/" + strings.Repeat("a", 5000)
	if _, ok := parseOSC7([]byte(long), "linux"); ok {
		t.Fatal("oversized path accepted")
	}
}

func TestOSC7ViaParser(t *testing.T) {
	a := NewApplier(screen.New(10, 3), io.Discard)
	a.goos = "linux"
	if a.Cwd() != "" {
		t.Fatalf("cwd before report: %q", a.Cwd())
	}
	vt.NewParser(a).Feed([]byte("\x1b]7;file://h/tmp\x07"))
	if a.Cwd() != "/tmp" {
		t.Fatalf("got %q", a.Cwd())
	}
	vt.NewParser(a).Feed([]byte("\x1b]7;http://x/y\x07"))
	if a.Cwd() != "/tmp" {
		t.Fatalf("bad OSC 7 changed cwd to %q", a.Cwd())
	}
}
