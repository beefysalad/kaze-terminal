package sysnotify

import (
	"errors"
	"sync"
)

// The native boundary is narrow so resource lifetime, replacement, callbacks and
// policy are tested without registering a shortcut or posting a real toast.
type windowsToast interface{ Close() }
type windowsEvent struct {
	kind string
	err  error
}
type windowsDriver interface {
	Init() error
	Post(Notification, string, string, func(windowsEvent)) (windowsToast, error)
	Remove(group, tag string) error
	Permission() (string, error)
	Raise(uintptr)
	Close()
}
type windowsLive struct {
	n          Notification
	group, tag string
	toast      windowsToast
}
type windowsBackend struct {
	mu              sync.Mutex
	driver          windowsDriver
	callbacks       Callbacks
	started, closed bool
	initErr         error
	deliveryErr     error
	status          string
	hwnd            uintptr
	live            map[string]*windowsLive
}

func newWindowsBackend(d windowsDriver, cb Callbacks) *windowsBackend {
	return &windowsBackend{driver: d, callbacks: cb, live: make(map[string]*windowsLive)}
}
func (b *windowsBackend) ensure() error {
	if b.closed {
		return errors.New("system notifications are closed")
	}
	if !b.started {
		b.started = true
		b.initErr = b.driver.Init()
		if b.initErr != nil {
			b.status = b.initErr.Error()
		}
	}
	return b.initErr
}
func (b *windowsBackend) changed() {
	if f := b.callbacks.Changed; f != nil {
		f()
	}
}
func (b *windowsBackend) Post(n Notification) error {
	group, tag, err := windowsToastIdentity(n.ID)
	if err != nil {
		return err
	}
	b.mu.Lock()
	err = b.post(n, group, tag)
	if err != nil {
		b.status = err.Error()
	}
	b.mu.Unlock()
	if err != nil {
		b.changed()
	}
	return err
}
func (b *windowsBackend) post(n Notification, group, tag string) error {
	if err := b.ensure(); err != nil {
		return err
	}
	if b.deliveryErr != nil {
		return b.deliveryErr
	}
	status, err := b.driver.Permission()
	if err != nil {
		b.deliveryErr = err
		return err
	}
	b.status = status
	if status != "" {
		return errors.New(status)
	}
	key := group + "-" + tag
	if old := b.live[key]; old != nil {
		b.drop(key, old)
	}
	if len(b.live) >= 256 {
		return errors.New("too many live system notifications")
	}
	r := &windowsLive{n: n, group: group, tag: tag}
	r.toast, err = b.driver.Post(n, group, tag, func(e windowsEvent) {
		// Never call native removal or the UI from inside a COM callback. A
		// finite event schedules cleanup after the callback can return.
		go b.event(r, e)
	})
	if err != nil {
		b.deliveryErr = err
		return err
	}
	b.live[key] = r
	return nil
}
func (b *windowsBackend) drop(key string, r *windowsLive) {
	delete(b.live, key)
	r.toast.Close()
	if err := b.driver.Remove(r.group, r.tag); err != nil {
		b.status = err.Error()
	}
}
func (b *windowsBackend) event(r *windowsLive, e windowsEvent) {
	b.mu.Lock()
	key := r.group + "-" + r.tag
	if b.closed || b.live[key] != r || e.kind == "timeout" {
		b.mu.Unlock()
		return
	}
	b.drop(key, r)
	if e.err != nil {
		b.deliveryErr = e.err
		b.status = e.err.Error()
	}
	b.mu.Unlock()
	if e.kind == "activate" && b.callbacks.Activate != nil {
		b.callbacks.Activate(r.n.ID)
	}
	if e.err != nil {
		b.changed()
	}
}
func (b *windowsBackend) Remove(id string) {
	group, tag, err := windowsToastIdentity(id)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := group + "-" + tag
	if r := b.live[key]; !b.closed && r != nil && r.n.ID == id {
		b.drop(key, r)
	}
}
func (b *windowsBackend) CheckPermission() {
	b.mu.Lock()
	before := b.status
	if err := b.ensure(); err == nil && b.deliveryErr == nil {
		status, err := b.driver.Permission()
		if err != nil {
			b.deliveryErr = err
			b.status = err.Error()
		} else {
			b.status = status
		}
	}
	changed := before != b.status
	b.mu.Unlock()
	if changed {
		b.changed()
	}
}
func (b *windowsBackend) Retry() {
	b.mu.Lock()
	if !b.closed && b.initErr != nil {
		b.driver.Close()
		b.started, b.initErr, b.status = false, nil, ""
	}
	if !b.closed {
		b.deliveryErr = nil
		b.status = ""
	}
	b.mu.Unlock()
	b.CheckPermission()
}
func (b *windowsBackend) Status() string         { b.mu.Lock(); defer b.mu.Unlock(); return b.status }
func (b *windowsBackend) SetWindow(hwnd uintptr) { b.mu.Lock(); b.hwnd = hwnd; b.mu.Unlock() }
func (b *windowsBackend) Raise() {
	b.mu.Lock()
	hwnd, closed := b.hwnd, b.closed
	b.mu.Unlock()
	if !closed && hwnd != 0 {
		b.driver.Raise(hwnd)
	}
}
func (b *windowsBackend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for key, r := range b.live {
		b.drop(key, r)
	}
	b.driver.Close()
}
