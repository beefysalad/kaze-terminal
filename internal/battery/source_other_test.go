//go:build !windows

package battery

import "testing"

func TestNoBatteryOffWindows(t *testing.T) {
	st, err := Native().Read()
	if err != nil || st != (Status{}) {
		t.Fatalf("Read = %+v, %v", st, err)
	}
	stop, ok := Native().Watch(func() {})
	if ok || stop != nil {
		t.Fatal("Watch reported notifications off Windows")
	}
}
