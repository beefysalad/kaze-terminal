package bench

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnvOffIsNil(t *testing.T) {
	for _, v := range []string{"", "0"} {
		if r := FromEnv(env(map[string]string{"KAZE_BENCH": v, "KAZE_BENCH_KEYS": "200"})); r != nil {
			t.Fatalf("KAZE_BENCH=%q: got a recorder, want nil", v)
		}
	}
}

func TestNilRecorderIsNoOp(t *testing.T) {
	var r *Recorder
	now := time.Now()
	r.Start(func() {}, nil)
	r.KeyArrived(now, Cursor{Tab: 1})
	r.FrameDone(now, now.Add(time.Millisecond), Frame{Drawn: true, Rows: 3})
	if r.WantText() || r.ShouldQuit() || r.ExtraTabs() != 0 {
		t.Fatal("nil recorder reported work")
	}
	if k := r.TakeKeys(now); k != nil {
		t.Fatalf("nil recorder injected keys: %v", k)
	}
	if s, ok := r.TakeType(now); ok || s != "" {
		t.Fatal("nil recorder wants to type")
	}
	if n := r.ScrollStep(now, 10000, 0, 40); n != 0 {
		t.Fatalf("nil recorder scrolls %d", n)
	}
	r.Close(1)
}

func TestFromEnvClampsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	r := FromEnv(env(map[string]string{
		"KAZE_BENCH": "1", "KAZE_BENCH_LOG": filepath.Join(dir, "b.log"),
		"KAZE_BENCH_KEYS": "999999", "KAZE_BENCH_TABS": "500", "KAZE_BENCH_SECONDS": "100000",
		"KAZE_BENCH_PACE_MS": "1", "KAZE_BENCH_SCROLL": "-4",
	}))
	if r == nil {
		t.Fatal("KAZE_BENCH=1 gave nil")
	}
	defer r.Close(0)
	c := r.cfg
	if c.Keys != maxKeys || c.Tabs != maxTabs || c.Seconds != maxSeconds || c.Pace != minPace || c.Scroll != 0 {
		t.Fatalf("not clamped: %+v", c)
	}
	d := FromEnv(env(map[string]string{"KAZE_BENCH": "1", "KAZE_BENCH_LOG": filepath.Join(dir, "d.log")}))
	defer d.Close(0)
	if d.cfg.Pace != 50*time.Millisecond || d.cfg.Seconds != 60 || d.cfg.Tabs != 1 || d.cfg.Keys != 0 {
		t.Fatalf("bad defaults: %+v", d.cfg)
	}
}

func TestPercentile(t *testing.T) {
	ms := func(v ...int) []time.Duration {
		out := make([]time.Duration, len(v))
		for i, x := range v {
			out[i] = time.Duration(x) * time.Millisecond
		}
		return out
	}
	if p := Percentile(nil, 50); p != 0 {
		t.Fatalf("empty: %v", p)
	}
	// Nearest rank: p50 of 1..10 is the 5th value, p99 the 10th.
	s := ms(10, 9, 8, 7, 6, 5, 4, 3, 2, 1)
	if p := Percentile(s, 50); p != 5*time.Millisecond {
		t.Fatalf("p50 = %v", p)
	}
	if p := Percentile(s, 99); p != 10*time.Millisecond {
		t.Fatalf("p99 = %v", p)
	}
	if p := Percentile(ms(7), 99); p != 7*time.Millisecond {
		t.Fatalf("single = %v", p)
	}
	if s[0] != 10*time.Millisecond {
		t.Fatal("Percentile reordered its input")
	}
	big := make([]time.Duration, 200)
	for i := range big {
		big[i] = time.Duration(i+1) * time.Millisecond
	}
	if p := Percentile(big, 99); p != 198*time.Millisecond {
		t.Fatalf("p99 of 1..200 = %v", p)
	}
}

func newTest(t *testing.T, extra map[string]string) (*Recorder, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bench.log")
	m := map[string]string{"KAZE_BENCH": "1", "KAZE_BENCH_LOG": path}
	for k, v := range extra {
		m[k] = v
	}
	r := FromEnv(env(m))
	if r == nil {
		t.Fatal("nil recorder")
	}
	return r, path
}

func readLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad line %q: %v", sc.Text(), err)
		}
		out = append(out, m)
	}
	return out
}

func TestKeyLatencyWaitsForTheEcho(t *testing.T) {
	r, path := newTest(t, nil)
	t0 := time.Now()
	at := Cursor{Tab: 1, Col: 5, Row: 2}
	r.KeyArrived(t0, at)
	// A frame that still shows the old cursor does not show the echo.
	r.FrameDone(t0, t0.Add(2*time.Millisecond), Frame{Drawn: true, Cursor: at})
	// A held (not drawn) frame does not either, even with the cursor moved.
	moved := Cursor{Tab: 1, Col: 6, Row: 2}
	r.FrameDone(t0, t0.Add(4*time.Millisecond), Frame{Drawn: false, Cursor: moved})
	r.FrameDone(t0.Add(5*time.Millisecond), t0.Add(7*time.Millisecond), Frame{Drawn: true, Cursor: moved})
	if len(r.keys) != 1 || r.keys[0] != 7*time.Millisecond {
		t.Fatalf("samples = %v, want [7ms]", r.keys)
	}
	// A key whose echo never comes is dropped after keyTimeout.
	r.KeyArrived(t0.Add(10*time.Millisecond), moved)
	r.FrameDone(t0, t0.Add(10*time.Millisecond+keyTimeout+time.Millisecond), Frame{Drawn: true, Cursor: moved})
	if r.dropped != 1 || len(r.pending) != 0 {
		t.Fatalf("dropped=%d pending=%d", r.dropped, len(r.pending))
	}
	r.Close(12.5)
	lines := readLog(t, path)
	sum := lines[len(lines)-1]
	if sum["event"] != "summary" {
		t.Fatalf("last line %v", sum)
	}
	keys := sum["keys"].(map[string]any)
	if keys["samples"].(float64) != 1 || keys["p50_ms"].(float64) != 7 || keys["dropped"].(float64) != 1 {
		t.Fatalf("keys summary %v", keys)
	}
	if sum["private_mb"].(float64) != 12.5 {
		t.Fatalf("private_mb %v", sum["private_mb"])
	}
}

func TestFramesCountLateAndRows(t *testing.T) {
	r, _ := newTest(t, nil)
	defer r.Close(0)
	t0 := time.Now()
	r.FrameDone(t0, t0.Add(3*time.Millisecond), Frame{Drawn: true, Rows: 10})
	r.FrameDone(t0, t0.Add(20*time.Millisecond), Frame{Drawn: true, Rows: 40})
	r.FrameDone(t0, t0.Add(1*time.Millisecond), Frame{Drawn: false})
	f := r.all
	if f.Frames != 3 || f.Drawn != 2 || f.Late != 1 || f.Rows != 50 {
		t.Fatalf("frames %+v", f)
	}
}

func TestLaunchLoggedOnceOnFirstTextFrame(t *testing.T) {
	r, path := newTest(t, nil)
	r.start = time.Now().Add(-300 * time.Millisecond)
	if !r.WantText() {
		t.Fatal("should want text before launch")
	}
	now := time.Now()
	r.FrameDone(now, now, Frame{Drawn: true, HasText: false})
	if !r.WantText() {
		t.Fatal("a blank frame is not the launch")
	}
	r.FrameDone(now, now, Frame{Drawn: true, HasText: true})
	if r.WantText() {
		t.Fatal("launch should be recorded")
	}
	r.FrameDone(now, now.Add(time.Second), Frame{Drawn: true, HasText: true})
	r.Close(0)
	n := 0
	for _, l := range readLog(t, path) {
		if l["event"] == "launch" {
			n++
			if ms := l["launch_ms"].(float64); ms < 299 || ms > 2000 {
				t.Fatalf("launch_ms = %v", ms)
			}
		}
	}
	if n != 1 {
		t.Fatalf("launch logged %d times", n)
	}
}

func TestSyntheticKeysArePacedAndAlternate(t *testing.T) {
	r, _ := newTest(t, map[string]string{"KAZE_BENCH_KEYS": "3"})
	defer r.Close(0)
	t0 := time.Now()
	if k := r.TakeKeys(t0); k != nil {
		t.Fatal("keys before launch")
	}
	r.FrameDone(t0, t0, Frame{Drawn: true, HasText: true})
	if k := r.TakeKeys(t0.Add(settle - time.Millisecond)); k != nil {
		t.Fatal("keys before settle")
	}
	base := t0.Add(settle)
	var got []SynthKey
	for i := 0; i < 20; i++ {
		got = append(got, r.TakeKeys(base.Add(time.Duration(i)*10*time.Millisecond))...)
	}
	// 0..190ms at 50ms pace: keys at 0, 50, 100, then the count runs out.
	want := []SynthKey{KeyText, KeyBackspace, KeyText}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if r.ShouldQuit() {
		t.Fatal("quit before the last echo was waited for")
	}
	r.TakeKeys(base.Add(time.Second + keyTimeout))
	if !r.ShouldQuit() {
		t.Fatal("should quit once keys are done")
	}
}

func TestTypeOnceAfterLaunch(t *testing.T) {
	r, _ := newTest(t, map[string]string{"KAZE_BENCH_TYPE": "flood -lines 10"})
	defer r.Close(0)
	t0 := time.Now()
	if _, ok := r.TakeType(t0); ok {
		t.Fatal("typed before launch")
	}
	r.FrameDone(t0, t0, Frame{Drawn: true, HasText: true})
	if s, ok := r.TakeType(t0.Add(settle)); !ok || s != "flood -lines 10\r" {
		t.Fatalf("TakeType = %q %v", s, ok)
	}
	if _, ok := r.TakeType(t0.Add(2 * settle)); ok {
		t.Fatal("typed twice")
	}
}

func TestScrollWaitsForQuietHistoryThenPagesToTheTop(t *testing.T) {
	r, path := newTest(t, map[string]string{"KAZE_BENCH_SCROLL": "100"})
	t0 := time.Now()
	if n := r.ScrollStep(t0, 50, 0, 10); n != 0 {
		t.Fatal("scrolled before history was full")
	}
	if n := r.ScrollStep(t0.Add(time.Millisecond), 100, 0, 10); n != 0 {
		t.Fatal("scrolled while history still changing")
	}
	start := t0.Add(time.Millisecond + quiet)
	n := r.ScrollStep(start, 100, 0, 10)
	if n != 3 { // 10 rows / 3 lines per notch
		t.Fatalf("notches = %d", n)
	}
	r.FrameDone(start, start.Add(5*time.Millisecond), Frame{Drawn: true, Rows: 10})
	r.FrameDone(start, start.Add(30*time.Millisecond), Frame{Drawn: true, Rows: 10})
	if n := r.ScrollStep(start.Add(40*time.Millisecond), 100, 100, 10); n != 0 {
		t.Fatal("kept scrolling at the top")
	}
	if !r.ShouldQuit() {
		t.Fatal("should quit after the scroll")
	}
	r.Close(0)
	lines := readLog(t, path)
	sc := lines[len(lines)-1]["scroll"].(map[string]any)
	if sc["lines"].(float64) != 100 || sc["frames"].(float64) != 2 || sc["late"].(float64) != 1 || sc["duration_ms"].(float64) != 40 {
		t.Fatalf("scroll summary %v", sc)
	}
}

func TestKeyLatencySplitsShellEchoFromKazeDraw(t *testing.T) {
	r, path := newTest(t, nil)
	t0 := time.Now()
	at := Cursor{Tab: 1, Col: 5, Row: 2}
	r.KeyArrived(t0, at)
	r.Output(t0.Add(20 * time.Millisecond)) // the echo reaches kaze
	r.Output(t0.Add(21 * time.Millisecond)) // later output does not move the mark
	// A frame drawn after the output that still shows the old cursor: the echo came in pieces.
	r.FrameDone(t0.Add(21*time.Millisecond), t0.Add(22*time.Millisecond), Frame{Drawn: true, Cursor: at})
	r.FrameDone(t0.Add(22*time.Millisecond), t0.Add(23*time.Millisecond), Frame{Drawn: true, Cursor: Cursor{Tab: 1, Col: 6, Row: 2}})
	r.Close(0)
	lines := readLog(t, path)
	keys := lines[len(lines)-1]["keys"].(map[string]any)
	if keys["p50_ms"].(float64) != 23 || keys["draw_p50_ms"].(float64) != 3 || keys["draw_samples"].(float64) != 1 || keys["frames_between_p50"].(float64) != 1 ||
		keys["last_out_p50_ms"].(float64) != 2 {
		t.Fatalf("keys summary %v", keys)
	}
}

func TestNilRecorderOutputIsNoOp(t *testing.T) {
	var r *Recorder
	r.Output(time.Now())
}

func TestRealInputDroppedByDefault(t *testing.T) {
	var none *Recorder
	if none.IgnoreInput() {
		t.Fatal("nil recorder (bench off) drops input")
	}
	r, _ := newTest(t, nil)
	defer r.Close(0)
	if !r.IgnoreInput() {
		t.Fatal("bench mode should drop real input unless KAZE_BENCH_ALLOW_INPUT=1")
	}
	q, _ := newTest(t, map[string]string{"KAZE_BENCH_ALLOW_INPUT": "1"})
	defer q.Close(0)
	if q.IgnoreInput() {
		t.Fatal("KAZE_BENCH_ALLOW_INPUT=1 not honored")
	}
}

func TestLastOutputCountsOnlyOutputBeforeTheFrame(t *testing.T) {
	r, path := newTest(t, nil)
	t0 := time.Now()
	r.KeyArrived(t0, Cursor{Tab: 1, Col: 1})
	r.Output(t0.Add(20 * time.Millisecond))
	r.Output(t0.Add(21 * time.Millisecond))
	r.Output(t0.Add(22500 * time.Microsecond)) // after the frame started: not in its rows for sure
	r.FrameDone(t0.Add(22*time.Millisecond), t0.Add(23*time.Millisecond), Frame{Drawn: true, Cursor: Cursor{Tab: 1, Col: 2}})
	r.Close(0)
	lines := readLog(t, path)
	keys := lines[len(lines)-1]["keys"].(map[string]any)
	if keys["last_out_p50_ms"].(float64) != 2 {
		t.Fatalf("last_out_p50_ms = %v, want 2", keys["last_out_p50_ms"])
	}
}

func TestSummaryFlagsTruncatedSamples(t *testing.T) {
	r, path := newTest(t, nil)
	t0 := time.Now()
	for range maxSamples + 1 {
		r.FrameDone(t0, t0.Add(time.Millisecond), Frame{Drawn: true})
	}
	r.Close(0)
	lines := readLog(t, path)
	f := lines[len(lines)-1]["frames"].(map[string]any)
	if f["truncated"] != true || f["frames"].(float64) != maxSamples+1 {
		t.Fatalf("frames %v", f)
	}
}

func TestLaunchSplitsWindowFromShell(t *testing.T) {
	r, path := newTest(t, nil)
	r.start = time.Now()
	base := r.start
	r.FrameDone(base, base.Add(100*time.Millisecond), Frame{Drawn: true}) // empty window up
	r.Output(base.Add(400 * time.Millisecond))                            // shell's first bytes
	r.FrameDone(base, base.Add(450*time.Millisecond), Frame{Drawn: true, HasText: true})
	r.Close(0)
	for _, l := range readLog(t, path) {
		if l["event"] == "launch" {
			if l["first_frame_ms"].(float64) != 100 || l["first_output_ms"].(float64) != 400 || l["launch_ms"].(float64) != 450 {
				t.Fatalf("launch %v", l)
			}
			return
		}
	}
	t.Fatal("no launch line")
}

func TestScreenWritesBoundedLines(t *testing.T) {
	r, path := newTest(t, nil)
	lines := make([]string, maxScreen+50)
	for i := range lines {
		lines[i] = "row"
	}
	r.Screen(lines)
	var none *Recorder
	none.Screen(lines)
	r.Close(0)
	for _, l := range readLog(t, path) {
		if l["event"] == "screen" {
			if n := len(l["lines"].([]any)); n != maxScreen {
				t.Fatalf("%d lines", n)
			}
			return
		}
	}
	t.Fatal("no screen line")
}
