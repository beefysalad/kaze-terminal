package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestControlSpawnWaitBudget(t *testing.T) {
	for _, waitStart := range []bool{false, true} {
		s := controlServer(t, nil)
		s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
			deadline, ok := ctx.Deadline()
			remaining := time.Until(deadline)
			if !ok {
				t.Error("request has no deadline")
			}
			budget := 65 * time.Second
			if waitStart {
				budget = 100 * time.Second
			}
			if remaining < budget-time.Second || remaining > budget {
				t.Errorf("spawn wait budget: %s", remaining)
			}
			if len(s.waitSem) != 1 || len(s.controlSem) != 0 {
				t.Errorf("wrong admission: wait=%d control=%d", len(s.waitSem), len(s.controlSem))
			}
			return map[string]any{"ok": true}
		})
		wire := `{"op":"spawn","token":"test-control-token","wait_start":false}`
		if waitStart {
			wire = `{"op":"spawn","token":"test-control-token","wait_start":true}`
		}
		if got := controlExchange(t, s, wire); got["ok"] != true {
			t.Fatal(got)
		}
	}
}

func TestControlSpawnWaitAdmission(t *testing.T) {
	s := controlServer(t, nil)
	s.deadline = 5 * time.Second
	entered := make(chan context.Context, 24)
	hooks := make(chan Msg, 1)
	s.onMsg = func(m Msg) { hooks <- m }
	release := make(chan struct{})
	defer close(release)
	s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
		entered <- ctx
		select {
		case <-release:
		case <-ctx.Done():
		}
		return map[string]any{"ok": true}
	})
	request := func(op string, waitStart bool) context.Context {
		c := controlConn(t, s)
		if err := json.NewEncoder(c).Encode(map[string]any{"op": op, "wait_start": waitStart, "token": s.Token()}); err != nil {
			t.Fatal(err)
		}
		select {
		case ctx := <-entered:
			return ctx
		case <-time.After(time.Second):
			t.Fatal("not admitted")
			return nil
		}
	}
	for range 8 {
		request("spawn", false)
	}
	for _, wire := range []string{`{"op":"spawn","wait_start":true,"token":"test-control-token"}`, `{"op":"wait","token":"test-control-token"}`} {
		got := controlExchange(t, s, wire)
		e, _ := got["error"].(map[string]any)
		if e["code"] != "busy_server" {
			t.Fatalf("wait budget not enforced: %v", got)
		}
	}
	for range 16 {
		request("read", false)
	}
	got := controlExchange(t, s, `{"op":"read","token":"test-control-token"}`)
	e, _ := got["error"].(map[string]any)
	if e["code"] != "busy_server" {
		t.Fatal(got)
	}
	c := controlConn(t, s)
	if err := json.NewEncoder(c).Encode(Msg{Kind: KindDone, Tab: 3, Token: s.Token()}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-hooks:
	case <-time.After(time.Second):
		t.Fatal("spawn/control saturation starved hook")
	}
}

func TestControlSpawnWaitDisconnect(t *testing.T) {
	s := controlServer(t, nil)
	entered := make(chan context.Context, 1)
	s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
		entered <- ctx
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		return map[string]any{"ok": true}
	})
	c := controlConn(t, s)
	if _, err := c.Write([]byte(`{"op":"spawn","wait_start":true,"token":"test-control-token"}`)); err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	select {
	case ctx = <-entered:
	case <-time.After(time.Second):
		t.Fatal("not admitted")
	}
	c.Close()
	select {
	case <-ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("disconnect did not cancel spawn")
	}
}
