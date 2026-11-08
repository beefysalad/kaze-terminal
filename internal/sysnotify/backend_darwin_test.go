//go:build darwin && !ios

package sysnotify

import (
	"testing"
	"time"
)

type fakeDarwinNative struct {
	started, checked, raised, closed int
	retried                          int
	posts                            []Notification
	removed                          []string
	window                           uintptr
	onClose                          func()
}

func (f *fakeDarwinNative) start(uint64)                  { f.started++ }
func (f *fakeDarwinNative) post(_ uint64, n Notification) { f.posts = append(f.posts, n) }
func (f *fakeDarwinNative) remove(_ uint64, id string)    { f.removed = append(f.removed, id) }
func (f *fakeDarwinNative) check(uint64)                  { f.checked++ }
func (f *fakeDarwinNative) retry(uint64)                  { f.retried++ }
func (f *fakeDarwinNative) setWindow(_ uint64, h uintptr) { f.window = h }
func (f *fakeDarwinNative) raise(uint64)                  { f.raised++ }
func (f *fakeDarwinNative) close(uint64) {
	f.closed++
	if f.onClose != nil {
		f.onClose()
	}
}

func TestDarwinBackendUsesFakeWithoutNativePermission(t *testing.T) {
	f := &fakeDarwinNative{}
	b := newDarwinBackend(f, Callbacks{})
	defer b.Close()
	if f.started != 1 || f.checked != 0 {
		t.Fatalf("start=%d permission=%d", f.started, f.checked)
	}
	b.SetWindow(123)
	b.CheckPermission()
	b.Retry()
	if f.checked != 1 || f.retried != 1 {
		t.Fatalf("explicit retry confused with ordinary refresh: check=%d retry=%d", f.checked, f.retried)
	}
	n := Notification{ID: "a-b-c", Title: "Review", Context: "main", Body: "Needs your permission"}
	if err := b.Post(n); err != nil {
		t.Fatal(err)
	}
	b.Remove(n.ID)
	if f.window != 123 || f.checked != 1 || len(f.posts) != 1 || f.posts[0] != n || len(f.removed) != 1 || f.removed[0] != n.ID {
		t.Fatalf("native calls %+v", f)
	}
}

func TestDarwinBackendDropsCallbacksAndOperationsAfterClose(t *testing.T) {
	f := &fakeDarwinNative{}
	activated, changed := 0, 0
	b := newDarwinBackend(f, Callbacks{Activate: func(string) { activated++ }, Changed: func() { changed++ }})
	b.Close()
	b.Close()
	darwinActivated(b.id, "a-b-c")
	darwinStatus(b.id, "late permission")
	b.CheckPermission()
	b.SetWindow(4)
	b.Raise()
	b.Remove("a-b-c")
	if err := b.Post(Notification{ID: "a-b-c"}); err == nil {
		t.Fatal("closed backend accepted post")
	}
	if activated != 0 || changed != 0 || f.closed != 1 || f.checked != 0 || f.window != 0 || f.raised != 0 || len(f.posts) != 0 || len(f.removed) != 0 {
		t.Fatalf("late work reached backend %+v activated=%d changed=%d", f, activated, changed)
	}
}

func TestDarwinBackendStatusAndActivationAreIsolated(t *testing.T) {
	first, second := "", ""
	b1 := newDarwinBackend(&fakeDarwinNative{}, Callbacks{Activate: func(id string) { first = id }})
	b2 := newDarwinBackend(&fakeDarwinNative{}, Callbacks{Activate: func(id string) { second = id }})
	defer b1.Close()
	defer b2.Close()
	darwinStatus(b1.id, "Blocked in macOS notification settings")
	darwinActivated(b2.id, "2-3-4")
	if b1.Status() != "Blocked in macOS notification settings" || b2.Status() != "" || first != "" || second != "2-3-4" {
		t.Fatalf("status/activation crossed instances %q %q %q %q", b1.Status(), b2.Status(), first, second)
	}
}

func TestDarwinBackendCallbackCanCloseWithoutDeadlock(t *testing.T) {
	var b *darwinBackend
	b = newDarwinBackend(&fakeDarwinNative{}, Callbacks{Activate: func(string) { b.Close() }})
	defer b.Close()
	done := make(chan struct{})
	go func() { darwinActivated(b.id, "a-b-c"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("native callback deadlocked")
	}
}

func TestDarwinBundleGuardAndReplacementIdentity(t *testing.T) {
	for _, tc := range []struct {
		path, id string
		want     bool
	}{
		{"", "", false},
		{"/tmp/kaze", "com.patrickgogo.kaze", false},
		{"/Applications/Kaze.app", "", false},
		{"/Applications/Other.app", "com.example.other", false},
		{"/Applications/Kaze.app", "com.patrickgogo.kaze", true},
	} {
		if got := darwinBundleSupported(tc.path, tc.id); got != tc.want {
			t.Errorf("bundle guard (%q, %q)=%v want %v", tc.path, tc.id, got, tc.want)
		}
	}
	for _, tc := range []struct{ id, want string }{
		{"a-b-1", "a-b"}, {"a-b-2", "a-b"}, {"a-c-1", "a-c"},
		{"", ""}, {"a-b", ""}, {"a-b-c-d", ""}, {"a--c", ""},
		{"a-FFFFFFFF-1", ""}, {"aaaaaaaaaaaaaaaaa-b-c", ""},
	} {
		if got := darwinReplacementKey(tc.id); got != tc.want {
			t.Errorf("key(%q)=%q want %q", tc.id, got, tc.want)
		}
	}
}

func TestDarwinCloseAllowsNativeCompletionToReadStatus(t *testing.T) {
	f := &fakeDarwinNative{}
	b := newDarwinBackend(f, Callbacks{})
	f.onClose = func() { _ = b.Status() }
	done := make(chan struct{})
	go func() { b.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close held state lock during native completion")
	}
}

func TestDarwinCloseUsesNativeEventRunnerAndWaitsForCompletion(t *testing.T) {
	runs := make(chan func(), 1)
	f := &fakeDarwinNative{}
	b := newDarwinBackend(f, Callbacks{Run: func(fn func()) { runs <- fn }})
	done := make(chan struct{})
	go func() { b.Close(); close(done) }()
	select {
	case fn := <-runs:
		select {
		case <-done:
			t.Fatal("Close returned before native cleanup")
		default:
		}
		fn()
	case <-done:
		t.Fatal("Close bypassed native event runner")
	case <-time.After(time.Second):
		t.Fatal("Close did not schedule native cleanup")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish after native cleanup")
	}
	if f.closed != 1 {
		t.Fatalf("native close count=%d", f.closed)
	}
}
