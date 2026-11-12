package discord

import (
	"errors"
	"io"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientHandshakeAndSet(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("Working on kaze"))
	h := f.frame(t, opHandshake)
	if h["v"] != float64(1) || h["client_id"] != "id" {
		t.Fatal(h)
	}
	args := f.activity(t)
	if args["pid"] != float64(42) || args["activity"].(map[string]any)["details"] != "Working on kaze" {
		t.Fatal(args)
	}
}
func TestClientEmptyIDNeverDials(t *testing.T) {
	var calls atomic.Int32
	c := New(Options{Dial: func() (io.ReadWriteCloser, error) { calls.Add(1); return nil, errors.New("absent") }})
	c.Set(testActivity("x"))
	waitClosed(t, c)
	if calls.Load() != 0 {
		t.Fatal("empty ID dialed")
	}
	var absent *Client
	absent.Set(nil)
	absent.Close()
}
func TestClientCoalescesBurst(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("first"))
	f.frame(t, opHandshake)
	f.activity(t)
	c.Set(testActivity("A"))
	c.Set(testActivity("B"))
	c.Set(testActivity("C"))
	clock.waitTimer(t, 15*time.Second)
	quiet(t, f.frames)
	clock.advance(15 * time.Second)
	if got := f.activity(t)["activity"].(map[string]any)["details"]; got != "C" {
		t.Fatalf("got %v", got)
	}
	quiet(t, f.frames)
}
func TestClientSkipsSameActivity(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	a := testActivity("same")
	c.Set(a)
	f.frame(t, opHandshake)
	f.activity(t)
	c.Set(testActivity("same"))
	clock.advance(15 * time.Second)
	quiet(t, f.frames)
}
func TestClientBacksOff(t *testing.T) {
	clock := newFakeClock()
	attempts := make(chan struct{}, 4)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { attempts <- struct{}{}; return nil, errors.New("absent") }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	receive(t, attempts)
	clock.waitTimer(t, 15*time.Second)
	quiet(t, attempts)
	clock.advance(15 * time.Second)
	receive(t, attempts)
}
func TestClientResendsAfterReconnect(t *testing.T) {
	clock := newFakeClock()
	peers := make(chan *fakeDiscord, 4)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { local, f := newFakeDiscord(t, true); peers <- f; return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("persist"))
	f := receive(t, peers)
	f.frame(t, opHandshake)
	f.activity(t)
	f.conn.Close()
	clock.waitTimer(t, 15*time.Second)
	clock.advance(15 * time.Second)
	f2 := receive(t, peers)
	f2.frame(t, opHandshake)
	if got := f2.activity(t)["activity"].(map[string]any)["details"]; got != "persist" {
		t.Fatal(got)
	}
}
func TestClientHandshakeTimeout(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, false)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	f.frame(t, opHandshake)
	clock.waitTimer(t, 5*time.Second)
	clock.advance(5 * time.Second)
	receive(t, f.done)
	clock.waitTimer(t, 15*time.Second)
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close waited")
	}
}
func TestClientDisableClears(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	f.frame(t, opHandshake)
	f.activity(t)
	c.Set(nil)
	if args := f.activity(t); args["activity"] != nil {
		t.Fatal("clear has activity")
	}
	receive(t, f.done)
}
func TestClientDisableWhileDisconnected(t *testing.T) {
	clock := newFakeClock()
	attempts := make(chan struct{}, 4)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { attempts <- struct{}{}; return nil, errors.New("absent") }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	receive(t, attempts)
	clock.waitTimer(t, 15*time.Second)
	c.Set(nil)
	clock.advance(15 * time.Second)
	quiet(t, attempts)
	c.Set(testActivity("again"))
	receive(t, attempts)
}
func TestClientPong(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	f.frame(t, opHandshake)
	f.activity(t)
	f.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := writeFrame(f.conn, opPing, []byte(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	if b := f.frame(t, opPong); b["n"] != float64(1) {
		t.Fatal(b)
	}
}
func TestClientRespectsLock(t *testing.T) {
	clock1, clock2 := newFakeClock(), newFakeClock()
	one, two := make(chan *fakeDiscord, 4), make(chan *fakeDiscord, 4)
	path := filepath.Join(t.TempDir(), "discord.lock")
	o := clientOptions(clock1, func() (io.ReadWriteCloser, error) { local, f := newFakeDiscord(t, true); one <- f; return local, nil })
	o.LockPath = path
	c1 := New(o)
	t.Cleanup(func() { waitClosed(t, c1) })
	c1.Set(testActivity("one"))
	f := receive(t, one)
	f.frame(t, opHandshake)
	f.activity(t)
	o = clientOptions(clock2, func() (io.ReadWriteCloser, error) { local, f := newFakeDiscord(t, true); two <- f; return local, nil })
	o.LockPath = path
	c2 := New(o)
	t.Cleanup(func() { waitClosed(t, c2) })
	c2.Set(testActivity("two"))
	clock2.waitTimer(t, 15*time.Second)
	quiet(t, two)
	c1.Close()
	clock2.advance(15 * time.Second)
	f2 := receive(t, two)
	f2.frame(t, opHandshake)
	f2.activity(t)
}
func TestClientCloseNeverWaits(t *testing.T) {
	clock := newFakeClock()
	local, peer := net.Pipe()
	defer peer.Close()
	called := make(chan struct{}, 1)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { called <- struct{}{}; return local, nil }))
	c.Set(testActivity("x"))
	receive(t, called)
	done := make(chan struct{})
	go func() { c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close waited for a peer that never reads")
	}
	receive(t, c.done)
}
func TestClientFrozenActivityWriteTimesOut(t *testing.T) {
	clock := newFakeClock()
	local, peer := net.Pipe()
	defer peer.Close()
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	c.Set(testActivity("x"))
	peer.SetDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := readFrame(peer); err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(peer, opFrame, []byte(`{"evt":"READY"}`)); err != nil {
		t.Fatal(err)
	}
	// Consume the activity header so its writer is known to be blocked on the
	// body before advancing the fake timeout.
	var header [8]byte
	if _, err := io.ReadFull(peer, header[:]); err != nil {
		t.Fatal(err)
	}
	clock.waitTimer(t, 5*time.Second)
	clock.advance(5 * time.Second)
	clock.waitTimer(t, 15*time.Second)
}

func TestClientCopiesWantedActivity(t *testing.T) {
	clock := newFakeClock()
	local, f := newFakeDiscord(t, true)
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
	t.Cleanup(func() { waitClosed(t, c) })
	a := testActivity("original")
	c.Set(a)
	a.Details = "changed by caller"
	f.frame(t, opHandshake)
	if got := f.activity(t)["activity"].(map[string]any)["details"]; got != "original" {
		t.Fatal(got)
	}
}
func TestClientRejectsInvalidReady(t *testing.T) {
	for _, op := range []uint32{opClose, opPing, opFrame} {
		t.Run(string(rune('0'+op)), func(t *testing.T) {
			clock := newFakeClock()
			local, peer := net.Pipe()
			defer peer.Close()
			c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) { return local, nil }))
			t.Cleanup(func() { waitClosed(t, c) })
			c.Set(testActivity("x"))
			peer.SetDeadline(time.Now().Add(2 * time.Second))
			if _, _, err := readFrame(peer); err != nil {
				t.Fatal(err)
			}
			if err := writeFrame(peer, op, []byte(`{"evt":"NOT_READY"}`)); err != nil {
				t.Fatal(err)
			}
			clock.waitTimer(t, 15*time.Second)
		})
	}
}
func TestClientCloseDuringDial(t *testing.T) {
	clock := newFakeClock()
	entered, finish := make(chan struct{}, 1), make(chan struct{})
	local, peer := net.Pipe()
	defer peer.Close()
	defer local.Close()
	c := New(clientOptions(clock, func() (io.ReadWriteCloser, error) {
		entered <- struct{}{}
		select {
		case <-finish:
		case <-time.After(2 * time.Second):
		}
		return local, nil
	}))
	c.Set(testActivity("x"))
	receive(t, entered)
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close waited on dial")
	}
	close(finish)
	receive(t, c.done)
	peer.SetReadDeadline(time.Now().Add(2 * time.Second))
	var b [1]byte
	if _, err := peer.Read(b[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("late dial connection not closed: %v", err)
	}
}
