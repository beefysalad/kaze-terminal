package notify

import (
	"net"
	"strings"
	"testing"
	"time"
)

func start(t *testing.T) (*Server, chan Msg) {
	t.Helper()
	ch := make(chan Msg, 8)
	s, err := Listen(func(m Msg) { ch <- m })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, ch
}

func expectNone(t *testing.T, ch chan Msg) {
	t.Helper()
	select {
	case m := <-ch:
		t.Fatalf("unexpected message: %+v", m)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRoundTrip(t *testing.T) {
	s, ch := start(t)
	err := Send(s.Addr(), s.Token(), Msg{Tab: 3, Kind: KindNeeds, Reason: "permission"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-ch:
		if m.Tab != 3 || m.Kind != KindNeeds || m.Reason != "permission" {
			t.Fatalf("got %+v", m)
		}
		if m.Token != "" {
			t.Fatalf("token leaked: %q", m.Token)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestRejectsBadToken(t *testing.T) {
	s, ch := start(t)
	Send(s.Addr(), "deadbeef", Msg{Tab: 1, Kind: KindDone}, time.Second)
	expectNone(t, ch)
}

func TestRejectsOversized(t *testing.T) {
	s, ch := start(t)
	Send(s.Addr(), s.Token(), Msg{Tab: 1, Kind: KindDone, Reason: strings.Repeat("x", 5*1024)}, time.Second)
	expectNone(t, ch)
}

func TestRejectsUnknownKind(t *testing.T) {
	s, ch := start(t)
	Send(s.Addr(), s.Token(), Msg{Tab: 1, Kind: Kind("bogus")}, time.Second)
	expectNone(t, ch)
}

func TestListensOnLoopbackOnly(t *testing.T) {
	s, _ := start(t)
	if !strings.HasPrefix(s.Addr(), "127.0.0.1:") {
		t.Fatalf("addr %q", s.Addr())
	}
	if len(s.Token()) != 64 {
		t.Fatalf("token len %d", len(s.Token()))
	}
}

func TestSlowClientDoesNotBlockOthers(t *testing.T) {
	s, ch := start(t)
	c, err := net.DialTimeout("tcp", s.Addr(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := Send(s.Addr(), s.Token(), Msg{Tab: 2, Kind: KindDone}, time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-ch:
		if m.Tab != 2 {
			t.Fatalf("got %+v", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked by slow client")
	}
}
