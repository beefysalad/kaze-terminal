//go:build darwin && !ios

package sysnotify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UserNotifications
#include <stdlib.h>
#include "notifications_darwin.h"
*/
import "C"

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type darwinNative interface {
	start(uint64)
	post(uint64, Notification)
	remove(uint64, string)
	check(uint64)
	retry(uint64)
	setWindow(uint64, uintptr)
	raise(uint64)
	close(uint64)
}

type darwinBackend struct {
	mu        sync.Mutex
	id        uint64
	native    darwinNative
	callbacks Callbacks
	status    string
	closed    bool
}

var darwinNextID atomic.Uint64
var darwinBackends sync.Map

func New(cb Callbacks) Backend { return newDarwinBackend(appKitNotifications{}, cb) }

func newDarwinBackend(native darwinNative, cb Callbacks) *darwinBackend {
	b := &darwinBackend{id: darwinNextID.Add(1), native: native, callbacks: cb}
	darwinBackends.Store(b.id, b)
	native.start(b.id)
	return b
}

func (b *darwinBackend) Post(n Notification) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("notification backend is closed")
	}
	b.native.post(b.id, n)
	return nil
}
func (b *darwinBackend) Remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.native.remove(b.id, id)
	}
}
func (b *darwinBackend) CheckPermission() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.native.check(b.id)
	}
}

func (b *darwinBackend) Retry() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.native.retry(b.id)
	}
}
func (b *darwinBackend) Status() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}
func (b *darwinBackend) SetWindow(handle uintptr) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.native.setWindow(b.id, handle)
	}
}
func (b *darwinBackend) Raise() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.native.raise(b.id)
	}
}
func (b *darwinBackend) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	darwinBackends.Delete(b.id)
	// Native close waits for AppKit cleanup. A callback already in flight may
	// need this lock before the main queue can reach that cleanup.
	b.mu.Unlock()
	if run := b.callbacks.Run; run != nil {
		done := make(chan struct{})
		go run(func() { b.native.close(b.id); close(done) })
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	} else {
		b.native.close(b.id)
	}
}

func darwinActivated(id uint64, notification string) {
	v, ok := darwinBackends.Load(id)
	if !ok {
		return
	}
	b := v.(*darwinBackend)
	b.mu.Lock()
	cb := b.callbacks.Activate
	if b.closed {
		cb = nil
	}
	b.mu.Unlock()
	if cb != nil {
		cb(notification)
	}
}
func darwinStatus(id uint64, status string) {
	v, ok := darwinBackends.Load(id)
	if !ok {
		return
	}
	b := v.(*darwinBackend)
	b.mu.Lock()
	cb := b.callbacks.Changed
	if b.closed || b.status == status {
		b.mu.Unlock()
		return
	}
	b.status = status
	b.mu.Unlock()
	if cb != nil {
		cb()
	}
}

//export kazeNotificationActivated
func kazeNotificationActivated(id C.uint64_t, notification *C.char) {
	darwinActivated(uint64(id), C.GoString(notification))
}

//export kazeNotificationStatus
func kazeNotificationStatus(id C.uint64_t, status *C.char) {
	darwinStatus(uint64(id), C.GoString(status))
}

type appKitNotifications struct{}

// These two pure bridge helpers also let tests verify the native identity and
// missing-bundle guards without initializing UserNotifications or AppKit.
func darwinBundleSupported(path, identifier string) bool {
	p, i := C.CString(path), C.CString(identifier)
	defer C.free(unsafe.Pointer(p))
	defer C.free(unsafe.Pointer(i))
	return C.kzNotificationSupportsBundle(p, i) != 0
}
func darwinReplacementKey(activation string) string {
	a := C.CString(activation)
	defer C.free(unsafe.Pointer(a))
	key := C.kzNotificationRequestKey(a)
	if key == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(key))
	return C.GoString(key)
}

func (appKitNotifications) start(id uint64) { C.kzNotificationStart(C.uint64_t(id)) }
func (appKitNotifications) post(id uint64, n Notification) {
	i, t, c, b := C.CString(n.ID), C.CString(n.Title), C.CString(n.Context), C.CString(n.Body)
	defer C.free(unsafe.Pointer(i))
	defer C.free(unsafe.Pointer(t))
	defer C.free(unsafe.Pointer(c))
	defer C.free(unsafe.Pointer(b))
	C.kzNotificationPost(C.uint64_t(id), i, t, c, b)
}
func (appKitNotifications) remove(id uint64, notification string) {
	i := C.CString(notification)
	defer C.free(unsafe.Pointer(i))
	C.kzNotificationRemove(C.uint64_t(id), i)
}
func (appKitNotifications) check(id uint64) { C.kzNotificationCheck(C.uint64_t(id)) }
func (appKitNotifications) retry(id uint64) { C.kzNotificationRetry(C.uint64_t(id)) }
func (appKitNotifications) setWindow(id uint64, h uintptr) {
	C.kzNotificationWindow(C.uint64_t(id), C.uintptr_t(h))
}
func (appKitNotifications) raise(id uint64) { C.kzNotificationRaise(C.uint64_t(id)) }
func (appKitNotifications) close(id uint64) { C.kzNotificationClose(C.uint64_t(id)) }
