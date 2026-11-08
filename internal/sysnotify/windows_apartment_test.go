package sysnotify

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestWindowsApartmentRetainsRuntimeUntilAllWorkCompletes(t *testing.T) {
	var initialized, uninitialized atomic.Int32
	a, err := newWindowsApartmentWith(func() error { initialized.Add(1); return nil }, func() { uninitialized.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err = a.run(func() error {
			if initialized.Load() != 1 || uninitialized.Load() != 0 {
				return errors.New("apartment ended before object release")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	a.close()
	a.close()
	if uninitialized.Load() != 1 {
		t.Fatal("unbalanced COM lifetime")
	}
	if err = a.run(func() error { t.Fatal("called after close"); return nil }); err == nil {
		t.Fatal("closed apartment accepted work")
	}
}
func TestWindowsApartmentInitializationFailureDoesNotUninitialize(t *testing.T) {
	var uninitialized atomic.Bool
	a, err := newWindowsApartmentWith(func() error { return errors.New("test init failure") }, func() { uninitialized.Store(true) })
	if err == nil || a != nil || uninitialized.Load() {
		t.Fatal("failed COM initialization was not balanced")
	}
}
