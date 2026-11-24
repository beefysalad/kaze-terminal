package notify

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

func TestControlReplyTruncated(t *testing.T) {
	text := strings.Repeat("あ<\n\"", 200<<10)
	s := controlServer(t, func(json.RawMessage) any {
		return struct {
			OK   bool   `json:"ok"`
			Text string `json:"text"`
		}{true, text}
	})
	got := controlExchange(t, s, `{"op":"read","token":"test-control-token"}`)
	prefix, _ := got["text"].(string)
	if got["ok"] != true || got["truncated"] != true || prefix == "" || !strings.HasPrefix(text, prefix) || !utf8.ValidString(prefix) || prefix == text {
		t.Fatalf("bad truncation: ok=%v truncated=%v prefix bytes=%d", got["ok"], got["truncated"], len(prefix))
	}
}

func TestControlStructuredReplyTruncated(t *testing.T) {
	s := controlServer(t, func(json.RawMessage) any {
		return map[string]any{"ok": true, "tabs": []string{strings.Repeat("x", (1<<20)+100)}}
	})
	got := controlExchange(t, s, `{"op":"ls","token":"test-control-token"}`)
	err, _ := got["error"].(map[string]any)
	if got["ok"] != false || got["truncated"] != true || err["code"] != "reply_too_large" {
		t.Fatalf("reply: %v", got)
	}
}

func TestControlSlowClientTimesOut(t *testing.T) {
	called := make(chan struct{}, 1)
	s := controlServer(t, func(json.RawMessage) any { called <- struct{}{}; return nil })
	c := controlConn(t, s)
	if _, err := io.WriteString(c, `{"op":"ls","token":`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var b [1]byte
	if n, err := c.Read(b[:]); n != 0 || err != io.EOF {
		t.Fatalf("read: n=%d err=%v", n, err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond || elapsed > time.Second {
		t.Fatalf("deadline took %v", elapsed)
	}
	select {
	case <-called:
		t.Fatal("incomplete request dispatched")
	default:
	}
}

func TestControlHookMessagesUnaffected(t *testing.T) {
	called := make(chan struct{}, 1)
	hooks := make(chan Msg, 2)
	s := controlServer(t, func(json.RawMessage) any { called <- struct{}{}; return nil })
	s.onMsg = func(m Msg) { hooks <- m }
	c := controlConn(t, s)
	if _, err := io.WriteString(c, `{"kind":"done","tab":3,"token":"test-control-token"}`); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := c.Read(b[:]); n != 0 || err != io.EOF {
		t.Fatalf("hook received reply: n=%d err=%v", n, err)
	}
	select {
	case m := <-hooks:
		if m.Tab != 3 || m.Kind != KindDone || m.Token != "" {
			t.Fatalf("hook: %+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("hook not delivered")
	}
	select {
	case <-called:
		t.Fatal("hook sent to control")
	default:
	}
	// The existing 32 KiB hook cap still applies with control registered.
	c = controlConn(t, s)
	request := `{"kind":"done","tab":3,"token":"test-control-token","reason":"` + strings.Repeat("x", 33<<10) + `"}`
	if _, err := io.WriteString(c, request); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Read(b[:]); n != 0 || err != io.EOF {
		t.Fatalf("oversize hook: n=%d err=%v", n, err)
	}
	select {
	case <-hooks:
		t.Fatal("oversized hook delivered")
	default:
	}
}

func TestControlBudgetsPreserveHooks(t *testing.T) {
	entered := make(chan struct{}, 32)
	release := make(chan struct{})
	s := controlServer(t, nil)
	s.deadline = 5 * time.Second
	s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return map[string]any{"ok": true}
	})
	defer close(release)
	hooks := make(chan Msg, 1)
	s.onMsg = func(m Msg) { hooks <- m }
	hold := func(op string) {
		c := controlConn(t, s)
		if err := json.NewEncoder(c).Encode(map[string]any{"op": op, "token": s.Token(), "timeout": 10}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("request not admitted")
		}
	}
	refuse := func(op string) {
		c := controlConn(t, s)
		_ = c.SetDeadline(time.Now().Add(400 * time.Millisecond))
		if err := json.NewEncoder(c).Encode(map[string]any{"op": op, "token": s.Token()}); err != nil {
			t.Fatal(err)
		}
		var reply struct {
			OK    bool
			Error struct{ Code string }
		}
		if err := json.NewDecoder(c).Decode(&reply); err != nil {
			t.Fatalf("excess %s did not get immediate refusal: %v", op, err)
		}
		if reply.OK || reply.Error.Code != "busy_server" {
			t.Fatalf("excess %s: %+v", op, reply)
		}
	}
	for i := 0; i < 8; i++ {
		hold("wait")
	}
	refuse("wait")
	for i := 0; i < 8; i++ {
		hold("read")
	}
	refuse("read")
	refuse("wait")
	c := controlConn(t, s)
	_ = c.SetDeadline(time.Now().Add(400 * time.Millisecond))
	if err := json.NewEncoder(c).Encode(Msg{Kind: KindDone, Tab: 3, Token: s.Token()}); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-hooks:
		if m.Tab != 3 {
			t.Fatal(m)
		}
	case <-time.After(400 * time.Millisecond):
		t.Fatal("control saturation starved legacy hook")
	}
	select {
	case <-entered:
		t.Fatal("excess request dispatched")
	default:
	}
}
