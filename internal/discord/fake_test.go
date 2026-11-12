package discord

import (
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type clockTimer struct {
	at time.Time
	ch chan time.Time
}
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []clockTimer
	changed chan struct{}
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1000, 0), changed: make(chan struct{}, 1)}
}
func (f *fakeClock) Now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }
func (f *fakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	ch := make(chan time.Time, 1)
	f.timers = append(f.timers, clockTimer{f.now.Add(d), ch})
	f.mu.Unlock()
	select {
	case f.changed <- struct{}{}:
	default:
	}
	return ch
}
func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	kept := f.timers[:0]
	for _, tm := range f.timers {
		if !tm.at.After(f.now) {
			tm.ch <- f.now
		} else {
			kept = append(kept, tm)
		}
	}
	f.timers = kept
}
func (f *fakeClock) waitTimer(t *testing.T, d time.Duration) {
	t.Helper()
	limit := time.NewTimer(2 * time.Second)
	defer limit.Stop()
	for {
		f.mu.Lock()
		found := false
		for _, tm := range f.timers {
			if tm.at.Equal(f.now.Add(d)) {
				found = true
			}
		}
		f.mu.Unlock()
		if found {
			return
		}
		select {
		case <-f.changed:
		case <-limit.C:
			t.Fatal("timer was not scheduled")
		}
	}
}

type testFrame struct {
	op   uint32
	body []byte
}
type fakeDiscord struct {
	conn   net.Conn
	frames chan testFrame
	done   chan struct{}
}

func newFakeDiscord(t *testing.T, ready bool) (io.ReadWriteCloser, *fakeDiscord) {
	t.Helper()
	local, peer := net.Pipe()
	f := &fakeDiscord{peer, make(chan testFrame, 32), make(chan struct{})}
	t.Cleanup(func() { peer.Close(); local.Close() })
	go func() {
		defer close(f.done)
		for i := 0; i < 32; i++ {
			peer.SetReadDeadline(time.Now().Add(2 * time.Second))
			op, b, err := readFrame(peer)
			if err != nil {
				return
			}
			select {
			case f.frames <- testFrame{op, b}:
			case <-time.After(2 * time.Second):
				return
			}
			if op == opHandshake && ready {
				peer.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if writeFrame(peer, opFrame, []byte(`{"evt":"READY"}`)) != nil {
					return
				}
			}
		}
	}()
	return local, f
}
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting")
		var zero T
		return zero
	}
}
func quiet[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("unexpected event: %v", v)
	case <-time.After(20 * time.Millisecond):
	}
}
func (f *fakeDiscord) frame(t *testing.T, op uint32) map[string]any {
	t.Helper()
	got := receive(t, f.frames)
	if got.op != op {
		t.Fatalf("opcode %d, want %d", got.op, op)
	}
	var body map[string]any
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	return body
}
func (f *fakeDiscord) activity(t *testing.T) map[string]any {
	t.Helper()
	b := f.frame(t, opFrame)
	if b["cmd"] != "SET_ACTIVITY" || b["nonce"] == "" || b["nonce"] == nil {
		t.Fatalf("bad SET_ACTIVITY: %v", b)
	}
	return b["args"].(map[string]any)
}
func testActivity(s string) *Activity {
	return &Activity{Details: s, State: "1 Claude session", Start: time.Unix(100, 0), LargeImage: "kaze", LargeText: "Kaze 0.41"}
}
func clientOptions(clock *fakeClock, dial func() (io.ReadWriteCloser, error)) Options {
	return Options{ClientID: "id", Dial: dial, Now: clock.Now, After: clock.After, Pid: 42, Spacing: 15 * time.Second, Reply: 5 * time.Second}
}
func waitClosed(t *testing.T, c *Client) { t.Helper(); c.Close(); receive(t, c.done) }
