package sysnotify

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeWindowsToast struct{ released atomic.Int32 }

func (t *fakeWindowsToast) Close() { t.released.Add(1) }

type fakeWindowsDriver struct {
	initErr       error
	permission    string
	inits, closes int
	posts         []Notification
	removed       []string
	toasts        []*fakeWindowsToast
	events        []func(windowsEvent)
}

func (d *fakeWindowsDriver) Init() error { d.inits++; return d.initErr }
func (d *fakeWindowsDriver) Post(n Notification, group, tag string, event func(windowsEvent)) (windowsToast, error) {
	d.posts = append(d.posts, n)
	d.events = append(d.events, event)
	r := &fakeWindowsToast{}
	d.toasts = append(d.toasts, r)
	return r, nil
}
func (d *fakeWindowsDriver) Remove(group, tag string) error {
	d.removed = append(d.removed, group+"-"+tag)
	return nil
}
func (d *fakeWindowsDriver) Permission() (string, error) { return d.permission, nil }
func (d *fakeWindowsDriver) Raise(uintptr)               {}
func (d *fakeWindowsDriver) Close()                      { d.closes++ }

func TestWindowsBackendReplacesAndRejectsStaleRemoveAndClick(t *testing.T) {
	d := &fakeWindowsDriver{}
	activated := make(chan string, 2)
	b := newWindowsBackend(d, Callbacks{Activate: func(id string) { activated <- id }})
	defer b.Close()
	if err := b.Post(Notification{ID: "1-2-3"}); err != nil {
		t.Fatal(err)
	}
	old := b.live["1-2"]
	if err := b.Post(Notification{ID: "1-2-4"}); err != nil {
		t.Fatal(err)
	}
	if d.toasts[0].released.Load() != 1 || len(d.removed) != 1 {
		t.Fatal("replacement leaked native resource")
	}
	b.Remove("1-2-3")
	b.event(old, windowsEvent{kind: "activate"})
	if len(b.live) != 1 || d.toasts[1].released.Load() != 0 {
		t.Fatal("stale remove/click affected latest")
	}
	select {
	case <-activated:
		t.Fatal("stale activation")
	default:
	}
	current := b.live["1-2"]
	b.event(current, windowsEvent{kind: "timeout"})
	if d.toasts[1].released.Load() != 0 {
		t.Fatal("banner timeout detached Notification Center callback")
	}
	d.events[1](windowsEvent{kind: "activate"})
	select {
	case id := <-activated:
		if id != "1-2-4" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("callback did not enqueue activation")
	}
	if d.toasts[1].released.Load() != 1 {
		t.Fatal("activated resource retained")
	}
}

func TestWindowsBackendFailureDismissalCloseAndInstanceIsolation(t *testing.T) {
	d := &fakeWindowsDriver{}
	b := newWindowsBackend(d, Callbacks{})
	for _, id := range []string{"1-2-3", "a-2-3", "1-3-3"} {
		if err := b.Post(Notification{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	b.event(b.live["1-2"], windowsEvent{kind: "dismiss"})
	b.event(b.live["1-3"], windowsEvent{kind: "failed", err: errors.New("test native failure")})
	if b.Status() == "" || len(b.live) != 1 {
		t.Fatal("failure missing or other instance removed")
	}
	b.Close()
	b.Close()
	if d.closes != 1 {
		t.Fatal("driver closed twice")
	}
	for _, r := range d.toasts {
		if r.released.Load() != 1 {
			t.Fatal("resource leak or double release")
		}
	}
	if err := b.Post(Notification{ID: "1-4-1"}); err == nil {
		t.Fatal("posted after close")
	}
}

func TestWindowsBackendRegistrationIsOnceAndPolicyIsExposed(t *testing.T) {
	d := &fakeWindowsDriver{initErr: errors.New("unrelated shortcut")}
	b := newWindowsBackend(d, Callbacks{})
	b.CheckPermission()
	b.CheckPermission()
	if d.inits != 1 || b.Status() == "" {
		t.Fatal("initialization retried automatically or missing status")
	}
	if err := b.Post(Notification{ID: "1-2-3"}); err == nil || len(d.posts) != 0 {
		t.Fatal("registration error ignored")
	}
	d.initErr = nil
	b.Retry()
	if d.inits != 2 || b.Status() != "" {
		t.Fatal("explicit retry did not recover registration")
	}
	b.Close()
	d = &fakeWindowsDriver{permission: "Blocked in Windows notification settings"}
	b = newWindowsBackend(d, Callbacks{})
	defer b.Close()
	if err := b.Post(Notification{ID: "1-2-3"}); err == nil || len(d.posts) != 0 {
		t.Fatal("OS policy ignored")
	}
	if err := b.Post(Notification{ID: "bad"}); err == nil {
		t.Fatal("malformed ID accepted")
	}
}

func TestWindowsBackendDeliveryFailureRecoversOnNextAlert(t *testing.T) {
	d := &fakeWindowsDriver{}
	b := newWindowsBackend(d, Callbacks{})
	defer b.Close()
	if err := b.Post(Notification{ID: "1-2-3"}); err != nil {
		t.Fatal(err)
	}
	b.event(b.live["1-2"], windowsEvent{kind: "failed", err: errors.New("native delivery failed")})
	b.CheckPermission()
	if b.Status() != "native delivery failed" {
		t.Fatal("permission refresh lost native error")
	}
	if len(d.posts) != 1 {
		t.Fatal("native delivery retried while idle")
	}
	if err := b.Post(Notification{ID: "1-2-4"}); err != nil || len(d.posts) != 2 {
		t.Fatal("one failed toast blocked the next alert", err)
	}
	if b.Status() != "" {
		t.Fatalf("recovered backend still reports %q", b.Status())
	}
	b.CheckPermission()
	if err := b.Post(Notification{ID: "1-2-5"}); err != nil || len(d.posts) != 3 {
		t.Fatal("recovered backend stopped posting", err)
	}
}
