//go:build windows

package sysnotify

import (
	"sync/atomic"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// These tests use only synthetic COM objects. They never initialize the native
// notification backend, register a shortcut, request permission or show a toast.
func TestWindowsNativeDelegateRetainsItsExactInterfaceAndReleases(t *testing.T) {
	called := false
	h := newNativeToastHandler(iidActivated, func(*winObject) { called = true })
	var out unsafe.Pointer
	unknown := winGUID("ffffffff-ffff-ffff-ffff-ffffffffffff")
	if hr := nativeHandlerQuery(h, &unknown, &out); hr != 0x80004002 || out != nil {
		t.Fatal("unknown interface accepted")
	}
	for _, iid := range []windows.GUID{iidActivated, iidUnknown, iidAgile} {
		if hr := nativeHandlerQuery(h, &iid, &out); hr != 0 || out != unsafe.Pointer(h) {
			t.Fatal("delegate QI failed")
		}
		if h.refs.Load() != 2 {
			t.Fatal("QueryInterface did not AddRef")
		}
		if nativeHandlerRelease(h) != 1 {
			t.Fatal("QueryInterface reference leaked")
		}
	}
	if hr := nativeHandlerInvoke(h, nil, nil); hr != 0 || !called {
		t.Fatal("delegate did not invoke")
	}
	if nativeHandlerRelease(h) != 0 {
		t.Fatal("initial reference leaked")
	}
	nativeHandlers.Lock()
	_, present := nativeHandlers.all[h]
	nativeHandlers.Unlock()
	if present {
		t.Fatal("released delegate remains rooted")
	}
}

type fakeWindowsCOM struct {
	vtable *[32]uintptr
	refs   atomic.Int32
}

func TestWindowsNativeObjectQueryAndReleaseUseIUnknownSlots(t *testing.T) {
	var table [32]uintptr
	table[0] = syscall.NewCallback(func(obj *fakeWindowsCOM, iid *windows.GUID, out **winObject) uintptr {
		obj.refs.Add(1)
		*out = (*winObject)(unsafe.Pointer(obj))
		return 0
	})
	table[2] = syscall.NewCallback(func(obj *fakeWindowsCOM) uintptr { return uintptr(obj.refs.Add(-1)) })
	obj := &fakeWindowsCOM{vtable: &table}
	obj.refs.Store(1)
	base := (*winObject)(unsafe.Pointer(obj))
	queried, err := base.query(&iidToast2)
	if err != nil || queried != base || obj.refs.Load() != 2 {
		t.Fatal("IUnknown query dispatch failed", err)
	}
	queried.release()
	base.release()
	if obj.refs.Load() != 0 {
		t.Fatal("IUnknown release dispatch failed")
	}
}

func TestWindowsPropertyABIAndKnownDelegateIIDs(t *testing.T) {
	if unsafe.Offsetof(winPropVariant{}.value) != 8 || unsafe.Sizeof(winPropVariant{}) != 24 || unsafe.Sizeof(winPropertyKey{}) != 20 {
		t.Fatal("incorrect 64-bit property ABI")
	}
	if iidActivated != winGUID("ab54de2d-97d9-5528-b6ad-105afe156530") || iidDismissed == iidActivated || iidFailed == iidActivated {
		t.Fatal("typed delegate identity mismatch")
	}
	if err := winHRESULT("test", uintptr(0x80004005)); err == nil {
		t.Fatal("HRESULT failure swallowed")
	}
	if err := winHRESULT("test", 1); err != nil {
		t.Fatal("HRESULT S_FALSE rejected")
	}
}
