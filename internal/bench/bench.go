// Package bench is kaze's in-app benchmark mode, switched on by KAZE_BENCH=1.
//
// It times work inside the process only: from a key event reaching kaze's
// routing to the end of the frame that shows its echo, and from process
// creation to the first frame with text. OS input delivery and the
// compositor's present latency are outside both numbers.
//
// With KAZE_BENCH unset FromEnv returns nil, every method is a no-op on a nil
// *Recorder, and the window skips its bench hooks behind one nil check.
package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxKeys    = 2000
	maxTabs    = 16
	maxSeconds = 600
	maxScroll  = 1_000_000
	maxType    = 1024
	minPace    = 10 * time.Millisecond
	maxPace    = time.Second
	// maxSamples bounds the frame-time samples kept per phase; counters keep going past it.
	maxSamples = 200_000
	maxPending = 64
	// maxScreen bounds the screen text written to the log at the end of a run.
	maxScreen = 200
	// LateFrame is one 60Hz refresh: a frame that took longer missed it.
	LateFrame = 16700 * time.Microsecond
	// keyTimeout drops a key whose echo never showed (a key the shell doesn't echo).
	keyTimeout = time.Second
	// settle is the wait after the first prompt before typing, so the shell's line editor is up.
	settle = 2 * time.Second
	// quiet is how long the history must stop growing before the scroll run starts.
	quiet = 1500 * time.Millisecond
	// backstop is how long past the deadline the process may live if the window never closes.
	backstop = 15 * time.Second
)

// Config is what the KAZE_BENCH_* variables asked for, clamped to safe bounds.
type Config struct {
	Log     string
	Keys    int           // synthetic keys to type, alternating "x" and Backspace
	Pace    time.Duration // between synthetic keys
	Tabs    int           // tabs to open at start
	Type    string        // a command typed once into every tab after the first prompt
	Scroll  int           // page to the top once history holds this many lines; 0 is off
	Seconds int           // the window closes itself after this long
	// IgnoreInput drops real keyboard, mouse and paste input, so a person typing
	// into a bench window that took focus can't run anything in its shells.
	IgnoreInput bool
}

// Cursor identifies where the cursor was, in which tab.
type Cursor struct{ Tab, Col, Row int }

// Frame is what the window reports after submitting one frame.
type Frame struct {
	Drawn   bool   // not held back by synchronized output
	Rows    int    // rows rasterized in this frame (render.Stats.RowsRedrawn delta)
	Cursor  Cursor // the active tab's cursor as drawn
	HasText bool   // the drawn screen has a non-blank cell; only filled while WantText
}

// SynthKey is one injected key.
type SynthKey int

const (
	KeyText      SynthKey = iota // the letter x, sent as a key press plus its text event
	KeyBackspace                 // Backspace, which erases the x
)

type pending struct {
	at  time.Time
	cur Cursor
	out time.Time // first shell output after the key: the echo reaching kaze
	// last is the latest shell output before the echo frame; from it to the frame is kaze's own share
	last time.Time
	// frames drawn after out that did not show the echo yet (the shell sent it in pieces)
	between int
}

// phase is frame statistics over one stretch of the run.
type phase struct {
	Frames int
	Drawn  int
	Late   int
	Rows   int
	Max    time.Duration
	durs   []time.Duration
}

func (p *phase) add(d time.Duration, f Frame) {
	p.Frames++
	if f.Drawn {
		p.Drawn++
	}
	p.Rows += f.Rows
	if d > LateFrame {
		p.Late++
	}
	p.Max = max(p.Max, d)
	if len(p.durs) < maxSamples {
		p.durs = append(p.durs, d)
	}
}

func (p *phase) summary() map[string]any {
	return map[string]any{
		"frames": p.Frames, "drawn": p.Drawn, "late": p.Late, "rows_rasterized": p.Rows,
		"p50_ms": ms(Percentile(p.durs, 50)), "p99_ms": ms(Percentile(p.durs, 99)), "max_ms": ms(p.Max),
	}
}

// Recorder collects one run. Its methods are called from the UI goroutine,
// except Close, which the backstop timer may call too.
type Recorder struct {
	cfg   Config
	start time.Time // process creation

	mu       sync.Mutex
	log      *os.File
	closed   bool
	launched bool
	launchAt time.Time
	// firstFrame and firstOut split launch into the window coming up and the shell starting.
	firstFrame time.Time
	firstOut   time.Time

	all, flood, scroll phase

	pending []pending
	keys    []time.Duration
	draws   []time.Duration // echo output reaching kaze to the frame that shows it
	between []time.Duration // frames drawn between that output and the echo frame, as a count
	lasts   []time.Duration // the last output before the echo frame to that frame
	dropped int
	sent    int
	nextKey time.Time
	lastKey time.Time

	typed bool

	scrollState int // 0 waiting, 1 running, 2 done
	lastLen     int
	changedAt   time.Time
	scrollStart time.Time
	scrollDur   time.Duration
	scrollLines int

	quit atomic.Bool
	stop chan struct{}
}

// FromEnv reads the KAZE_BENCH_* variables through getenv (os.Getenv in kaze).
// It returns nil, and so costs nothing, unless KAZE_BENCH is set and not "0".
func FromEnv(getenv func(string) string) *Recorder {
	if v := getenv("KAZE_BENCH"); v == "" || v == "0" {
		return nil
	}
	c := Config{
		Log:     getenv("KAZE_BENCH_LOG"),
		Keys:    intIn(getenv("KAZE_BENCH_KEYS"), 0, 0, maxKeys),
		Pace:    time.Duration(intIn(getenv("KAZE_BENCH_PACE_MS"), 50, int(minPace/time.Millisecond), int(maxPace/time.Millisecond))) * time.Millisecond,
		Tabs:    intIn(getenv("KAZE_BENCH_TABS"), 1, 1, maxTabs),
		Type:    getenv("KAZE_BENCH_TYPE"),
		Scroll:  intIn(getenv("KAZE_BENCH_SCROLL"), 0, 0, maxScroll),
		Seconds: intIn(getenv("KAZE_BENCH_SECONDS"), 60, 1, maxSeconds),

		IgnoreInput: getenv("KAZE_BENCH_IGNORE_INPUT") == "1",
	}
	if len(c.Type) > maxType {
		c.Type = c.Type[:maxType]
	}
	if c.Log == "" {
		c.Log = filepath.Join(os.TempDir(), "kaze-bench.log")
	}
	r := &Recorder{cfg: c, start: processStart(), stop: make(chan struct{})}
	if f, err := os.Create(c.Log); err == nil {
		r.log = f
	}
	r.write(map[string]any{"event": "config", "keys": c.Keys, "pace_ms": ms(c.Pace), "tabs": c.Tabs,
		"type": c.Type, "scroll": c.Scroll, "seconds": c.Seconds,
		"note": "in-process timing: excludes OS input delivery and compositor present latency"})
	return r
}

func intIn(s string, def, lo, hi int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Percentile is the nearest-rank p-th percentile of ds; it leaves ds unchanged.
func Percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	rank := int(p/100*float64(len(s)) + 0.999999999)
	return s[min(max(rank, 1), len(s))-1]
}

// write appends one JSON line; call with mu held or before the recorder is shared.
func (r *Recorder) write(v map[string]any) {
	if r.log == nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	r.log.Write(append(b, '\n'))
}

// Start arms the deadline and, only while there is scripted work, a ticker that
// asks for frames. mem reads private memory in MB for the backstop's summary.
func (r *Recorder) Start(invalidate func(), mem func() float64) {
	if r == nil {
		return
	}
	deadline := time.Duration(r.cfg.Seconds) * time.Second
	time.AfterFunc(deadline, func() {
		r.quit.Store(true)
		invalidate()
	})
	time.AfterFunc(deadline+backstop, func() {
		mb := 0.0
		if mem != nil {
			mb = mem()
		}
		r.Close(mb)
		os.Exit(3)
	})
	if r.cfg.Keys == 0 && r.cfg.Type == "" && r.cfg.Scroll == 0 {
		return // an idle run: no wakeups of our own
	}
	tick := min(r.cfg.Pace/2, 25*time.Millisecond)
	go func() {
		t := time.NewTicker(tick)
		defer t.Stop()
		end := time.After(deadline)
		for {
			select {
			case <-t.C:
				invalidate()
			case <-end:
				return
			case <-r.stop:
				return
			}
		}
	}()
}

// ExtraTabs is how many tabs to open beyond the first.
func (r *Recorder) ExtraTabs() int {
	if r == nil {
		return 0
	}
	return r.cfg.Tabs - 1
}

// WantText reports whether the window should check the frame for text (launch not yet seen).
func (r *Recorder) WantText() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.launched
}

// IgnoreInput reports whether real input is to be dropped (synthetic input still goes through).
func (r *Recorder) IgnoreInput() bool { return r != nil && r.cfg.IgnoreInput }

// Screen logs the active tab's text at the end of a run, so a run can be checked afterwards.
func (r *Recorder) Screen(lines []string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.write(map[string]any{"event": "screen", "lines": lines[:min(len(lines), maxScreen)]})
}

// ShouldQuit reports whether the run is over and the window should close.
func (r *Recorder) ShouldQuit() bool {
	return r != nil && r.quit.Load()
}

// KeyArrived records a key that was just routed to the shell, with the cursor at that moment.
func (r *Recorder) KeyArrived(now time.Time, c Cursor) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) >= maxPending {
		r.pending = r.pending[1:]
		r.dropped++
	}
	r.pending = append(r.pending, pending{at: now, cur: c})
}

// Output records that the active tab's shell wrote something; it may be called from any goroutine.
func (r *Recorder) Output(now time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.firstOut.IsZero() {
		r.firstOut = now
	}
	for i := range r.pending {
		if r.pending[i].out.IsZero() {
			r.pending[i].out = now
		}
		r.pending[i].last = now
	}
}

// FrameDone records a frame from its event (start) to the return of its submit (end).
func (r *Recorder) FrameDone(start, end time.Time, f Frame) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d := end.Sub(start)
	r.all.add(d, f)
	if r.typed {
		r.flood.add(d, f)
	}
	if r.scrollState == 1 {
		r.scroll.add(d, f)
	}
	if f.Drawn && r.firstFrame.IsZero() {
		r.firstFrame = end
	}
	if !r.launched && f.Drawn && f.HasText {
		r.launched, r.launchAt = true, end
		r.write(map[string]any{"event": "launch", "launch_ms": ms(end.Sub(r.start)),
			"first_frame_ms": ms(r.firstFrame.Sub(r.start)), "first_output_ms": ms(r.firstOut.Sub(r.start)),
			"process_start_unix_ns": r.start.UnixNano(), "first_text_frame_unix_ns": end.UnixNano()})
	}
	if !f.Drawn {
		return
	}
	keep := r.pending[:0]
	for _, p := range r.pending {
		switch {
		case p.cur != f.Cursor:
			r.keys = append(r.keys, end.Sub(p.at))
			if !p.out.IsZero() {
				r.draws = append(r.draws, end.Sub(p.out))
				r.between = append(r.between, time.Duration(p.between))
				r.lasts = append(r.lasts, end.Sub(p.last))
			}
		case end.Sub(p.at) > keyTimeout:
			r.dropped++
		default:
			if !p.out.IsZero() {
				p.between++
			}
			keep = append(keep, p)
		}
	}
	r.pending = keep
}

// TakeKeys returns the synthetic key due now, if any.
func (r *Recorder) TakeKeys(now time.Time) []SynthKey {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.launched || r.cfg.Keys == 0 {
		return nil
	}
	if r.sent >= r.cfg.Keys {
		if now.Sub(r.lastKey) >= keyTimeout {
			r.quit.Store(true)
		}
		return nil
	}
	if r.nextKey.IsZero() {
		r.nextKey = r.launchAt.Add(settle)
	}
	if now.Before(r.nextKey) {
		return nil
	}
	k := KeyText
	if r.sent%2 == 1 {
		k = KeyBackspace
	}
	r.sent++
	r.lastKey, r.nextKey = now, now.Add(r.cfg.Pace)
	return []SynthKey{k}
}

// TakeType returns the command to type into every tab, once, settle after launch.
func (r *Recorder) TakeType(now time.Time) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.typed || r.cfg.Type == "" || !r.launched || now.Before(r.launchAt.Add(settle)) {
		return "", false
	}
	r.typed = true
	r.write(map[string]any{"event": "typed", "at_ms": ms(now.Sub(r.start))})
	return r.cfg.Type + "\r", true
}

// ScrollStep drives the scroll run from the active tab's history length, view
// offset and rows. It returns how many wheel notches (3 lines each) to scroll
// up this frame: a page at a time once history is full and quiet, until the top.
func (r *Recorder) ScrollStep(now time.Time, sbLen, viewOff, rows int) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg.Scroll == 0 || r.scrollState == 2 {
		return 0
	}
	if r.scrollState == 0 {
		if sbLen != r.lastLen {
			r.lastLen, r.changedAt = sbLen, now
			return 0
		}
		if sbLen < r.cfg.Scroll || now.Sub(r.changedAt) < quiet {
			return 0
		}
		r.scrollState, r.scrollStart = 1, now
		r.write(map[string]any{"event": "scroll_start", "history": sbLen})
	}
	if viewOff >= sbLen {
		r.scrollState, r.scrollDur, r.scrollLines = 2, now.Sub(r.scrollStart), sbLen
		r.quit.Store(true)
		return 0
	}
	return max(rows/3, 1)
}

// Close writes the summary once and closes the log. privateMB is the process's private memory.
func (r *Recorder) Close(privateMB float64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	close(r.stop)
	s := map[string]any{
		"event": "summary", "private_mb": privateMB, "frames": r.all.summary(),
		"keys": map[string]any{
			"samples": len(r.keys), "dropped": r.dropped, "sent": r.sent,
			"p50_ms": ms(Percentile(r.keys, 50)), "p99_ms": ms(Percentile(r.keys, 99)),
			"min_ms": ms(Percentile(r.keys, 0)), "max_ms": ms(Percentile(r.keys, 100)),
			"draw_samples": len(r.draws), "draw_p50_ms": ms(Percentile(r.draws, 50)),
			"draw_p99_ms": ms(Percentile(r.draws, 99)), "draw_max_ms": ms(Percentile(r.draws, 100)),
			"last_out_p50_ms": ms(Percentile(r.lasts, 50)), "last_out_p99_ms": ms(Percentile(r.lasts, 99)),
			"last_out_max_ms":    ms(Percentile(r.lasts, 100)),
			"frames_between_p50": int64(Percentile(r.between, 50)), "frames_between_max": int64(Percentile(r.between, 100)),
		},
	}
	if r.launched {
		s["launch_ms"] = ms(r.launchAt.Sub(r.start))
	}
	if r.typed {
		s["after_type"] = r.flood.summary()
	}
	if r.cfg.Scroll > 0 {
		sc := r.scroll.summary()
		sc["lines"], sc["duration_ms"], sc["done"] = r.scrollLines, ms(r.scrollDur), r.scrollState == 2
		s["scroll"] = sc
	}
	r.write(s)
	if r.log != nil {
		r.log.Close()
		r.log = nil
	}
}
