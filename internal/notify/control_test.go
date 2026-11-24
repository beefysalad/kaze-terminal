package notify

import (
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func controlServer(t *testing.T, handler func(json.RawMessage) any) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := serve(ln, nil, 100*time.Millisecond)
	s.token = "test-control-token"
	s.SetControl(handler)
	t.Cleanup(func() { s.Close() })
	return s
}

func controlConn(t *testing.T, s *Server) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", s.Addr(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func controlExchange(t *testing.T, s *Server, request string) map[string]any {
	t.Helper()
	c := controlConn(t, s)
	if _, err := io.WriteString(c, request); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(io.LimitReader(c, (1<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 1<<20 {
		t.Fatal("reply exceeds cap")
	}
	var reply map[string]any
	if err := json.Unmarshal(b, &reply); err != nil {
		t.Fatalf("reply %q: %v", b, err)
	}
	// ReadAll observes EOF, and Unmarshal rejects a second reply.
	return reply
}

func TestControlRoundTrip(t *testing.T) {
	calls := make(chan json.RawMessage, 2)
	s := controlServer(t, func(raw json.RawMessage) any {
		calls <- raw
		return map[string]any{"ok": true, "text": "screen"}
	})
	got := controlExchange(t, s, `{"op":"read","token":"test-control-token","tab":"t3"}`)
	if got["ok"] != true || got["text"] != "screen" {
		t.Fatalf("reply: %v", got)
	}
	select {
	case raw := <-calls:
		var req map[string]any
		if err := json.Unmarshal(raw, &req); err != nil || req["tab"] != "t3" {
			t.Fatalf("request: %s", raw)
		}
	case <-time.After(time.Second):
		t.Fatal("handler not called")
	}
	select {
	case <-calls:
		t.Fatal("handled a second request")
	default:
	}
}

func TestControlBadToken(t *testing.T) {
	called := make(chan struct{}, 1)
	s := controlServer(t, func(json.RawMessage) any { called <- struct{}{}; return nil })
	got := controlExchange(t, s, `{"op":"ls","token":"wrong"}`)
	err, _ := got["error"].(map[string]any)
	if got["ok"] != false || err["code"] != "unauthorized" {
		t.Fatalf("reply: %v", got)
	}
	select {
	case <-called:
		t.Fatal("unauthenticated handler called")
	default:
	}
}

func TestControlNoHandler(t *testing.T) {
	s := controlServer(t, nil)
	got := controlExchange(t, s, `{"op":"ls","token":"test-control-token"}`)
	err, _ := got["error"].(map[string]any)
	if got["ok"] != false || err["code"] != "control_unavailable" {
		t.Fatalf("reply: %v", got)
	}
}

func TestControlRequestCap(t *testing.T) {
	called := make(chan struct{}, 2)
	s := controlServer(t, func(json.RawMessage) any { called <- struct{}{}; return map[string]any{"ok": true} })
	base := `{"op":"ls","token":"test-control-token","padding":""}`
	for _, size := range []int{64 << 10, (64 << 10) + 1} {
		request := base[:len(base)-2] + strings.Repeat("x", size-len(base)) + `"}`
		c := controlConn(t, s)
		if _, err := io.WriteString(c, request); err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(io.LimitReader(c, 1024))
		if err != nil {
			t.Fatal(err)
		}
		if size == 64<<10 {
			if !json.Valid(b) {
				t.Fatalf("boundary reply: %q", b)
			}
			select {
			case <-called:
			default:
				t.Fatal("boundary not dispatched")
			}
		} else {
			select {
			case <-called:
				t.Fatal("oversized request dispatched")
			default:
			}
			var reply struct{ Error struct{ Code string } }
			if json.Unmarshal(b, &reply) != nil || reply.Error.Code != "request_too_large" {
				t.Fatalf("oversized request reply: %s", b)
			}
		}
	}
}
