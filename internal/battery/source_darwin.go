package battery

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include "iops_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// The two system notify alerts Watch registers for.
const (
	keyPercent = 0 // com.apple.system.powersources.percent
	keySource  = 1 // kIOPSNotifyPowerSource: plugged in or unplugged
)

var errNoPowerInfo = errors.New("battery: IOKit returned no power sources")

type darwinSource struct{}

// Native is the macOS battery source.
func Native() Source { return darwinSource{} }

// Read asks IOKit for the internal battery once, without allocating.
func (darwinSource) Read() (Status, error) { return readPower() }

// readPower, register and unregister are variables so tests can fake them.
var (
	readPower  = readIOPS
	register   = registerNotify
	unregister = cancelNotify
)

func readIOPS() (Status, error) {
	b := C.kaze_battery_read()
	if b.err != 0 {
		return Status{}, errNoPowerInfo
	}
	return fromIOPS(b.found != 0, b.charging != 0, b.onAC != 0, int(b.cur), int(b.full)), nil
}

func registerNotify(key int) (int32, error) {
	r := C.kaze_battery_register(C.int(key))
	if r.status != 0 {
		return 0, fmt.Errorf("battery: notify_register_dispatch status %d", int(r.status))
	}
	return int32(r.token), nil
}

func cancelNotify(token int32) error {
	if s := C.kaze_battery_cancel(C.int(token)); s != 0 {
		return fmt.Errorf("battery: notify_cancel status %d", int(s))
	}
	return nil
}

// current is the one watcher the notify handler reaches.
var current atomic.Pointer[func()]

//export kazeBatteryChanged
func kazeBatteryChanged() { notifyChanged() }

// notifyChanged runs on a dispatch queue thread and does nothing but call the
// watcher's changed: no read, no lock, no allocation.
func notifyChanged() {
	if f := current.Load(); f != nil {
		(*f)()
	}
}

// Watch registers for the battery percent and power source alerts. With no
// battery it registers nothing; if either registration fails, it undoes the
// other and reports ok false so the caller falls back to a lazy poll. One
// watcher per process: a later Watch replaces the earlier one's changed.
func (darwinSource) Watch(changed func()) (func(), bool) {
	if st, err := readPower(); err != nil || !st.Present {
		return nil, false
	}
	fn := &changed
	current.Store(fn)
	var toks [2]int32
	for i, key := range [2]int{keyPercent, keySource} {
		tok, err := register(key)
		if err != nil {
			for _, done := range toks[:i] {
				unregister(done)
			}
			current.CompareAndSwap(fn, nil)
			return nil, false
		}
		toks[i] = tok
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			current.CompareAndSwap(fn, nil)
			for _, tok := range toks {
				unregister(tok)
			}
		})
	}, true
}
