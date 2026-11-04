package ui

import (
	"sync/atomic"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/battery"
)

// batteryEvery is how often the battery is read when Windows' change
// notifications could not be registered, on frames that happen anyway.
const batteryEvery = time.Minute

const (
	batUnchecked uint8 = iota
	batNone            // no battery: never read again
	batWatching        // read on a change notification
	batPolling         // read at most every batteryEvery
)

// batteryWork reads the battery for the status bar. It has no timer and no
// goroutine: the status model calls tick on frames that happen anyway, a
// change notification only marks it dirty and asks for one frame, and a
// machine with no battery is read once and never again.
type batteryWork struct {
	src                    battery.Source
	wake                   func()
	state                  uint8
	status                 battery.Status
	readAt                 time.Time
	dirty, paused, stopped atomic.Bool
	stop                   func()
}

// batterySource is the native source unless a test replaces it.
var batterySource = battery.Native

func newBatteryWork(src battery.Source, wake func()) *batteryWork {
	return &batteryWork{src: src, wake: wake}
}

// initBattery starts the battery reader, except in bench mode, so KAZE_BENCH
// frames stay as they are.
func (win *window) initBattery() {
	if win.bench != nil {
		return
	}
	win.battery = newBatteryWork(batterySource(), win.w.Invalidate)
}

// tick returns the battery to show. When nothing is due it is two atomic loads
// and a time compare, and it never allocates.
func (w *batteryWork) tick(now time.Time) battery.Status {
	if w == nil || w.paused.Load() {
		return w.last()
	}
	switch w.state {
	case batUnchecked:
		st, err := w.src.Read()
		w.readAt = now
		if err != nil || !st.Present {
			w.state = batNone
			return w.status
		}
		w.status = st
		w.dirty.Store(false)
		if stop, ok := w.src.Watch(w.changed); ok {
			w.stop, w.state = stop, batWatching
		} else {
			w.state = batPolling
		}
	case batWatching:
		if w.dirty.Swap(false) {
			w.read(now)
		}
	case batPolling:
		if w.dirty.Swap(false) || now.Sub(w.readAt) >= batteryEvery {
			w.read(now)
		}
	}
	return w.status
}

func (w *batteryWork) last() battery.Status {
	if w == nil {
		return battery.Status{}
	}
	return w.status
}

// read takes a fresh reading, keeping the last one when the read fails.
func (w *batteryWork) read(now time.Time) {
	w.readAt = now
	if st, err := w.src.Read(); err == nil {
		w.status = st
	}
}

// changed runs on the OS notification thread: mark dirty, ask for one frame.
func (w *batteryWork) changed() {
	w.dirty.Store(true)
	if !w.paused.Load() && !w.stopped.Load() && w.wake != nil {
		w.wake()
	}
}

// pause stops reading while minimized; restoring reads once on the next frame.
func (w *batteryWork) pause(p bool) {
	if w == nil || w.paused.Swap(p) == p {
		return
	}
	if !p {
		w.dirty.Store(true)
	}
}

// close unregisters the notifications once.
func (w *batteryWork) close() {
	if w == nil || w.stopped.Swap(true) {
		return
	}
	if w.stop != nil {
		w.stop()
	}
}
