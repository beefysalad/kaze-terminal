package ui

import (
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/battery"
	"github.com/patrick-gogo/kaze-terminal/internal/bench"
	"github.com/patrick-gogo/kaze-terminal/internal/render"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"

	"gioui.org/io/input"
	"gioui.org/op"
	"gioui.org/unit"
)

// fakeBattery is a battery.Source that counts what it is asked.
type fakeBattery struct {
	st                    battery.Status
	watchOK               bool
	reads, watches, stops int
	changed               func()
}

func (f *fakeBattery) Read() (battery.Status, error) { f.reads++; return f.st, nil }
func (f *fakeBattery) Watch(changed func()) (func(), bool) {
	f.watches++
	if !f.watchOK {
		return nil, false
	}
	f.changed = changed
	return func() { f.stops++ }, true
}

func newFakeBatteryWork(f *fakeBattery) (*batteryWork, *int) {
	wakes := 0
	return newBatteryWork(f, func() { wakes++ }), &wakes
}

func TestBatteryDesktopReadsOnce(t *testing.T) {
	f := &fakeBattery{}
	w, _ := newFakeBatteryWork(f)
	now := t0
	for range 1000 {
		if st := w.tick(now); st.Present {
			t.Fatal("a desktop showed a battery")
		}
		now = now.Add(7200 * time.Millisecond) // 1000 ticks over 2 h
	}
	if f.reads != 1 || f.watches != 0 {
		t.Fatalf("reads %d, watches %d on a desktop", f.reads, f.watches)
	}
}

func TestBatteryNotifyWakesOnce(t *testing.T) {
	f := &fakeBattery{st: battery.Status{Present: true, Percent: 82}, watchOK: true}
	w, wakes := newFakeBatteryWork(f)
	now := t0
	for range 50 {
		w.tick(now)
		now = now.Add(time.Second)
	}
	if f.reads != 1 || f.watches != 1 {
		t.Fatalf("reads %d, watches %d", f.reads, f.watches)
	}
	f.st.Percent = 81
	f.changed()
	if *wakes != 1 {
		t.Fatalf("%d wakes", *wakes)
	}
	if st := w.tick(now); st.Percent != 81 || f.reads != 2 {
		t.Fatalf("after a change: %+v, %d reads", st, f.reads)
	}
	for range 49 {
		now = now.Add(time.Minute)
		w.tick(now)
	}
	if f.reads != 2 {
		t.Fatalf("watching read %d times without a change", f.reads)
	}
}

func TestBatteryPollsAtMostOncePerMinute(t *testing.T) {
	f := &fakeBattery{st: battery.Status{Present: true, Percent: 50}}
	w, wakes := newFakeBatteryWork(f)
	now := t0
	for range 151 { // every 2 s from 0 to 5 min
		w.tick(now)
		now = now.Add(2 * time.Second)
	}
	if f.reads != 1+5 {
		t.Fatalf("%d reads in 5 min of polling", f.reads)
	}
	if *wakes != 0 {
		t.Fatalf("polling scheduled %d wakes", *wakes)
	}
}

func TestBatteryPausedReadsNothing(t *testing.T) {
	f := &fakeBattery{st: battery.Status{Present: true, Percent: 50}, watchOK: true}
	w, wakes := newFakeBatteryWork(f)
	w.tick(t0)
	w.pause(true)
	f.changed()
	if *wakes != 0 {
		t.Fatal("a change woke a minimized window")
	}
	now := t0
	for range 100 {
		now = now.Add(time.Minute)
		w.tick(now)
	}
	if f.reads != 1 {
		t.Fatalf("%d reads while minimized", f.reads)
	}
	w.pause(false)
	w.tick(now)
	w.tick(now)
	if f.reads != 2 {
		t.Fatalf("restoring read %d times, want once", f.reads-1)
	}
}

func TestBatteryCloseStopsWatching(t *testing.T) {
	f := &fakeBattery{st: battery.Status{Present: true, Percent: 50}, watchOK: true}
	w, wakes := newFakeBatteryWork(f)
	w.tick(t0)
	w.close()
	w.close()
	if f.stops != 1 {
		t.Fatalf("%d stops", f.stops)
	}
	f.changed()
	if *wakes != 0 {
		t.Fatal("a change woke a closed window")
	}
	var nilWork *batteryWork
	nilWork.close()
	nilWork.pause(true)
	if st := nilWork.tick(t0); st.Present {
		t.Fatal("no battery work showed a battery")
	}
}

func TestBatteryTickAllocatesNothing(t *testing.T) {
	f := &fakeBattery{st: battery.Status{Present: true, Percent: 50}, watchOK: true}
	w, _ := newFakeBatteryWork(f)
	w.tick(t0)
	if n := testing.AllocsPerRun(100, func() { w.tick(t0) }); n != 0 {
		t.Fatalf("%.0f allocations per tick", n)
	}
}

func TestNoBatteryInBenchMode(t *testing.T) {
	win := testWindow(t, &recStarter{})
	vars := map[string]string{"KAZE_BENCH": "1", "KAZE_BENCH_LOG": filepath.Join(t.TempDir(), "b.log")}
	rec := bench.FromEnv(func(k string) string { return vars[k] })
	t.Cleanup(func() { rec.Close(0) })
	win.bench = rec
	win.initBattery()
	if win.battery != nil {
		t.Fatal("bench mode started the battery")
	}
	win.statusModel(time.Now())
}

func TestStatusBatteryColors(t *testing.T) {
	th := theme.All()[0]
	c := newChrome()
	c.setTheme(th)
	u := th.UI
	cases := []struct {
		pct      int
		charging bool
		text     color.NRGBA // before the readable nudge
		bolt     bool
	}{
		{82, false, u.Folder, false},
		{64, true, u.Folder, true},
		{18, false, u.Needs, false},
		{7, false, u.UsageHigh, false},
		{10, false, u.UsageHigh, false},
		{20, false, u.Needs, false},
		{21, false, u.Folder, false},
	}
	for _, k := range cases {
		ink := c.batteryInk(k.pct, k.charging)
		if want := render.Readable(k.text, u.Chrome); ink.text != want || ink.fill != want {
			t.Errorf("%d%%: text %v fill %v, want %v", k.pct, ink.text, ink.fill, want)
		}
		outline := u.StatusFG
		if k.pct <= 20 {
			outline = k.text
		}
		if want := render.Readable(outline, u.Chrome); ink.outline != want {
			t.Errorf("%d%%: outline %v, want %v", k.pct, ink.outline, want)
		}
		if ink.bolt != k.bolt {
			t.Errorf("%d%%: bolt %v", k.pct, ink.bolt)
		}
		if k.bolt && ink.boltFill != render.Readable(u.Done, u.Chrome) {
			t.Errorf("%d%%: bolt colour %v", k.pct, ink.boltFill)
		}
	}
}

func TestStatusBatteryReadableInEveryTheme(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		for _, pct := range []int{82, 18, 7} {
			ink := c.batteryInk(pct, true)
			for name, col := range map[string]color.NRGBA{"text": ink.text, "fill": ink.fill, "outline": ink.outline, "bolt": ink.boltFill} {
				if r := contrast(col, th.UI.Chrome); r < 4.5 {
					t.Errorf("%s %d%%: %s %.2f:1 on the bar", th.Key, pct, name, r)
				}
			}
		}
	}
}

func TestStatusBatteryCacheHits(t *testing.T) {
	var cache statusLayoutCache
	runs := 0
	measure := func(s string, mono bool) int { runs++; return seven(s, mono) }
	m := batteryModel()
	cache.layout(m, 1400, statusTestDims, unit.Metric{}, nil, measure)
	n := runs
	cache.layout(m, 1400, statusTestDims, unit.Metric{}, nil, measure)
	if runs != n {
		t.Fatal("the same model with a battery laid out again")
	}
	m.Battery.Percent = 81
	cache.layout(m, 1400, statusTestDims, unit.Metric{}, nil, measure)
	if runs == n {
		t.Fatal("a new percent did not lay out again")
	}
	n = runs
	m.Battery.Charging = true
	cache.layout(m, 1400, statusTestDims, unit.Metric{}, nil, measure)
	if runs == n {
		t.Fatal("charging did not lay out again")
	}
}

// statusFrameAllocs is the allocations of an unchanged status bar frame.
func statusFrameAllocs(t *testing.T, st battery.Status) float64 {
	t.Helper()
	win, _ := sideWindow(t, true)
	openIn(t, win, osPath("/code/alpha/a"))
	if st.Present {
		win.battery = newBatteryWork(&fakeBattery{st: st, watchOK: true}, func() {})
	}
	var r input.Router
	sizedFrame(t, win, &r, 1600, 600)
	var ops op.Ops
	g := pageContext(1600, 600)
	g.Ops = &ops
	now := time.Now()
	draw := func() { ops.Reset(); win.drawStatus(g, 568, now) }
	for range 10 {
		draw()
	}
	if st.Present && win.statusRect(grpBattery).Empty() {
		t.Fatal("the battery was not drawn")
	}
	return testing.AllocsPerRun(50, draw)
}

func TestStatusBatteryAddsNoAllocations(t *testing.T) {
	without := statusFrameAllocs(t, battery.Status{})
	for _, st := range []battery.Status{{Present: true, Percent: 82}, {Present: true, Percent: 64, Charging: true}, {Present: true, Percent: 7}} {
		if with := statusFrameAllocs(t, st); with != without {
			t.Fatalf("%+v: %g allocations per unchanged frame, %g without a battery", st, with, without)
		}
	}
	t.Logf("%g allocations per unchanged status frame", without)
}
