package battery

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestNativeReadOnWindows(t *testing.T) {
	st, err := Native().Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent < 0 || st.Percent > 100 {
		t.Fatalf("percent %d", st.Percent)
	}
	if !st.Present && (st.Percent != 0 || st.Charging) {
		t.Fatalf("no battery but %+v", st)
	}
	t.Logf("this machine: %+v", st)
}

func BenchmarkRead(b *testing.B) {
	src := Native()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := src.Read(); err != nil {
			b.Fatal(err)
		}
	}
}

// fakeProcs swaps the power procs and the read for fakes until the test ends.
type fakeProcs struct {
	st       Status
	failAt   int // 1-based registration that fails; 0 never
	regs     []windows.GUID
	unregs   []uintptr
	nextHand uintptr
}

func useFakes(t *testing.T, f *fakeProcs) {
	t.Helper()
	oldRead, oldReg, oldUnreg := readPower, register, unregister
	t.Cleanup(func() { readPower, register, unregister = oldRead, oldReg, oldUnreg })
	readPower = func() (Status, error) { return f.st, nil }
	register = func(g *windows.GUID) (uintptr, error) {
		if len(f.regs)+1 == f.failAt {
			return 0, errors.New("refused")
		}
		f.regs = append(f.regs, *g)
		f.nextHand++
		return f.nextHand, nil
	}
	unregister = func(h uintptr) error { f.unregs = append(f.unregs, h); return nil }
}

func TestWatchRegistersBothSettings(t *testing.T) {
	f := &fakeProcs{st: Status{Present: true, Percent: 82}}
	useFakes(t, f)
	stop, ok := Native().Watch(func() {})
	if !ok || stop == nil {
		t.Fatal("Watch failed with a battery")
	}
	if len(f.regs) != 2 || f.regs[0] != guidBatteryPercent || f.regs[1] != guidPowerSource {
		t.Fatalf("registered %v", f.regs)
	}
	stop()
	if len(f.unregs) != 2 {
		t.Fatalf("unregistered %v", f.unregs)
	}
	stop()
	if len(f.unregs) != 2 {
		t.Fatalf("second stop unregistered again: %v", f.unregs)
	}
}

func TestWatchNotRegisteredWithoutBattery(t *testing.T) {
	f := &fakeProcs{}
	useFakes(t, f)
	stop, ok := Native().Watch(func() {})
	if ok || stop != nil || len(f.regs) != 0 {
		t.Fatalf("ok %v, registered %v on a desktop", ok, f.regs)
	}
}

func TestWatchFailureUnregistersAndFallsBack(t *testing.T) {
	f := &fakeProcs{st: Status{Present: true, Percent: 50}, failAt: 2}
	useFakes(t, f)
	stop, ok := Native().Watch(func() {})
	if ok || stop != nil {
		t.Fatal("Watch reported ok after a failed registration")
	}
	if len(f.regs) != 1 || len(f.unregs) != 1 || f.unregs[0] != 1 {
		t.Fatalf("regs %v, unregs %v", f.regs, f.unregs)
	}
	if current.Load() != nil {
		t.Fatal("a failed Watch left its callback in place")
	}
}

func TestCallbackCallsChanged(t *testing.T) {
	f := &fakeProcs{st: Status{Present: true, Percent: 50}}
	useFakes(t, f)
	n := 0
	stop, ok := Native().Watch(func() { n++ })
	if !ok {
		t.Fatal("Watch failed")
	}
	if r := dispatch(0, 0, 0); r != 0 || n != 1 {
		t.Fatalf("dispatch returned %d, changed ran %d times", r, n)
	}
	stop()
	dispatch(0, 0, 0)
	if n != 1 {
		t.Fatalf("changed ran after stop (%d)", n)
	}
}

func TestWatchRealSmoke(t *testing.T) {
	done := make(chan string, 1)
	go func() {
		stop, ok := Native().Watch(func() {})
		if ok {
			stop()
			stop()
		}
		st, _ := Native().Read()
		done <- "watch ok=" + map[bool]string{true: "true", false: "false"}[ok] + ", present=" + map[bool]string{true: "true", false: "false"}[st.Present]
	}()
	select {
	case s := <-done:
		t.Log(s)
	case <-time.After(2 * time.Second):
		t.Fatal("Watch and stop took over 2 s")
	}
}
