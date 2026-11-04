package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/battery"
	"github.com/patrick-gogo/kaze-terminal/internal/bench"
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
