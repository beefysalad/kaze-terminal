package vt_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/patrick-gogo/kaze-terminal/internal/vt"
)

type recorder struct{ calls []string }

func (r *recorder) Print(c rune) { r.calls = append(r.calls, "print "+string(c)) }
func (r *recorder) Execute(b byte) {
	r.calls = append(r.calls, fmt.Sprintf("exec %02x", b))
}
func (r *recorder) CSI(params []int, inter []byte, private byte, final byte) {
	s := "csi"
	if private != 0 {
		s += " " + string(private)
	}
	s += fmt.Sprintf(" %v", params)
	if len(inter) > 0 {
		s += " " + string(inter)
	}
	s += " " + string(final)
	r.calls = append(r.calls, s)
}
func (r *recorder) ESC(inter []byte, final byte) {
	s := "esc"
	if len(inter) > 0 {
		s += " " + string(inter)
	}
	r.calls = append(r.calls, s+" "+string(final))
}
func (r *recorder) OSC(data []byte) { r.calls = append(r.calls, "osc "+string(data)) }

func feed(s string) []string {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte(s))
	return rec.calls
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUTF8SplitAcrossFeeds(t *testing.T) {
	rec := &recorder{}
	p := vt.NewParser(rec)
	b := []byte("日")
	p.Feed(b[:1])
	p.Feed(b[1:])
	if !reflect.DeepEqual(rec.calls, []string{"print 日"}) {
		t.Fatalf("%v", rec.calls)
	}
}

func TestCSIWithPrivateMarkerAndParams(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte("\x1b[?1049h\x1b[38;2;10;20;30m\x1b[H"))
	want := []string{"csi ? [1049] h", "csi [38 2 10 20 30] m", "csi [-1] H"}
	if !reflect.DeepEqual(rec.calls, want) {
		t.Fatalf("%v", rec.calls)
	}
}

func TestCSISplitAcrossFeeds(t *testing.T) {
	rec := &recorder{}
	p := vt.NewParser(rec)
	p.Feed([]byte("\x1b[3"))
	p.Feed([]byte("1m"))
	if !reflect.DeepEqual(rec.calls, []string{"csi [31] m"}) {
		t.Fatalf("%v", rec.calls)
	}
}

func TestOSCTerminatedByBELAndST(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte("\x1b]0;one\x07\x1b]2;two\x1b\\"))
	if !reflect.DeepEqual(rec.calls, []string{"osc 0;one", "osc 2;two"}) {
		t.Fatalf("%v", rec.calls)
	}
}

func TestInvalidUTF8BecomesReplacementChar(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte{0xff, 'a'})
	if !reflect.DeepEqual(rec.calls, []string{"print �", "print a"}) {
		t.Fatalf("%v", rec.calls)
	}
}

func TestTruncatedUTF8ThenASCII(t *testing.T) {
	expect(t, feed("\xe6\x97a"), "print �", "print a")
}

func TestControlsExecute(t *testing.T) {
	expect(t, feed("a\r\nb\x07\x08\t"), "print a", "exec 0d", "exec 0a", "print b", "exec 07", "exec 08", "exec 09")
}

func TestColonIsParamSeparator(t *testing.T) {
	expect(t, feed("\x1b[4:3m"), "csi [4 3] m")
	expect(t, feed("\x1b[58:2::1:2:3m"), "csi [58 2 -1 1 2 3] m")
	expect(t, feed("\x1b[1;;3m"), "csi [1 -1 3] m")
	expect(t, feed("\x1b[1;m"), "csi [1 -1] m")
}

func TestCSIIntermediates(t *testing.T) {
	expect(t, feed("\x1b[2 q"), "csi [2]   q")
	expect(t, feed("\x1b[>c"), "csi > [-1] c")
}

func TestControlInsideCSIExecutes(t *testing.T) {
	expect(t, feed("\x1b[1\n2m"), "exec 0a", "csi [12] m")
}

func TestCANAndSUBCancelSequence(t *testing.T) {
	expect(t, feed("\x1b[12\x18a"), "print a")
	expect(t, feed("\x1b[12\x1aa"), "print a")
}

func TestESCInsideCSIRestarts(t *testing.T) {
	expect(t, feed("\x1b[12\x1b[3m"), "csi [3] m")
}

func TestCSIIgnoreOnMalformed(t *testing.T) {
	// private marker after a param is invalid; whole sequence dropped
	expect(t, feed("\x1b[1?2mX"), "print X")
	// digit after intermediate is invalid
	expect(t, feed("\x1b[ 1mX"), "print X")
}

func TestESCSequences(t *testing.T) {
	expect(t, feed("\x1b7\x1b(B\x1bM"), "esc 7", "esc ( B", "esc M")
}

func TestParamCap(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("\x1b[")
	for i := 0; i < 40; i++ {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString("1")
	}
	sb.WriteString("m")
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte(sb.String()))
	if len(rec.calls) != 1 || !strings.HasPrefix(rec.calls[0], "csi [1 1") {
		t.Fatalf("%v", rec.calls)
	}
	if n := strings.Count(rec.calls[0], "1"); n != 32 {
		t.Fatalf("want 32 params, got %d: %v", n, rec.calls[0])
	}
}

func TestParamOverflowClamped(t *testing.T) {
	got := feed("\x1b[99999999999999999999m")
	if len(got) != 1 || !strings.HasPrefix(got[0], "csi [") || strings.Contains(got[0], "-") {
		t.Fatalf("%v", got)
	}
}

func TestOSCCap(t *testing.T) {
	var gotLen int
	h := &oscLen{n: &gotLen}
	data := "\x1b]0;" + strings.Repeat("a", 100000) + "\x07"
	vt.NewParser(h).Feed([]byte(data))
	if gotLen != 64*1024 {
		t.Fatalf("osc len %d", gotLen)
	}
}

type oscLen struct {
	recorder
	n *int
}

func (o *oscLen) OSC(d []byte) { *o.n = len(d) }

func TestOSCSplitAcrossFeedsAndESCSplit(t *testing.T) {
	rec := &recorder{}
	p := vt.NewParser(rec)
	p.Feed([]byte("\x1b]0;ti"))
	p.Feed([]byte("tle\x1b"))
	p.Feed([]byte("\\x"))
	expect(t, rec.calls, "osc 0;title", "print x")
}

func TestOSCInterruptedByESCSequence(t *testing.T) {
	// ESC followed by something other than '\' ends the OSC and starts a new escape
	expect(t, feed("\x1b]0;t\x1b[1m"), "osc 0;t", "csi [1] m")
}

func TestDCSAndAPCIgnored(t *testing.T) {
	expect(t, feed("a\x1bPq#0;2;0;0;0\x1b\\b"), "print a", "print b")
	expect(t, feed("a\x1b_Gf=100;AAAA\x1b\\b"), "print a", "print b")
	expect(t, feed("a\x1bXjunk\x1b\\b\x1b^pm\x1b\\c"), "print a", "print b", "print c")
}

func TestOSCSharedBufferReused(t *testing.T) {
	expect(t, feed("\x1b]1;a\x07\x1b]2;bb\x07"), "osc 1;a", "osc 2;bb")
}

type nullHandler struct{}

func (nullHandler) Print(rune)                    {}
func (nullHandler) Execute(byte)                  {}
func (nullHandler) CSI([]int, []byte, byte, byte) {}
func (nullHandler) ESC([]byte, byte)              {}
func (nullHandler) OSC([]byte)                    {}

func TestFeedDoesNotAllocate(t *testing.T) {
	p := vt.NewParser(nullHandler{})
	in := []byte("hello 日本語\x1b[1;31mred\x1b[0m\x1b]0;title\x07\r\n")
	allocs := testing.AllocsPerRun(100, func() { p.Feed(in) })
	if allocs != 0 {
		t.Fatalf("allocs per Feed = %v", allocs)
	}
}

func mixedInput() []byte {
	chunk := []byte("hello 日本語 world\x1b[1;38;2;10;20;30mcolor\x1b[0m\r\n\x1b[?25l\x1b]0;title\x07 line of text here\r\n")
	return bytes.Repeat(chunk, (1<<20)/len(chunk)+1)[:1<<20]
}

func BenchmarkFeedASCII(b *testing.B) {
	in := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\r\n"), (1<<20)/45+1)[:1<<20]
	p := vt.NewParser(nullHandler{})
	b.SetBytes(int64(len(in)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Feed(in)
	}
}

func BenchmarkFeedMixed(b *testing.B) {
	in := mixedInput()
	p := vt.NewParser(nullHandler{})
	b.SetBytes(int64(len(in)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Feed(in)
	}
}

type subRecorder struct {
	params []int
	colon  uint32
}

func (r *subRecorder) Print(rune)                    {}
func (r *subRecorder) Execute(byte)                  {}
func (r *subRecorder) CSI([]int, []byte, byte, byte) {}
func (r *subRecorder) ESC([]byte, byte)              {}
func (r *subRecorder) OSC([]byte)                    {}
func (r *subRecorder) CSISub(params []int, colon uint32, _ []byte, _ byte, _ byte) {
	r.params = append([]int(nil), params...)
	r.colon = colon
}

func TestColonMaskMarksSubParams(t *testing.T) {
	r := &subRecorder{}
	vt.NewParser(r).Feed([]byte("\x1b[1;4:3;38:2::10:20:30m"))
	if want := []int{1, 4, 3, 38, 2, -1, 10, 20, 30}; !reflect.DeepEqual(r.params, want) {
		t.Fatalf("params = %v", r.params)
	}
	want := uint32(1<<2 | 1<<4 | 1<<5 | 1<<6 | 1<<7 | 1<<8)
	if r.colon != want {
		t.Fatalf("colon = %b, want %b", r.colon, want)
	}
}

func TestPlainHandlerStillGetsCSI(t *testing.T) {
	expect(t, feed("\x1b[4:3m"), "csi [4 3] m")
}
