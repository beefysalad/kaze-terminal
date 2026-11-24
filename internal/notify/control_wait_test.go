package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestControlWaitDisconnect(t *testing.T) {
	s := controlServer(t, nil)
	started := make(chan struct{})
	finished := make(chan struct{})
	s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
		close(started)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		close(finished)
		return controlError("cancelled", "cancelled")
	})
	c := controlConn(t, s)
	if _, err := c.Write([]byte(`{"op":"wait","timeout":1,"token":"test-control-token"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not started")
	}
	// Longer than the fixture's 100 ms request-read deadline, still connected.
	select {
	case <-finished:
		t.Fatal("handler cancelled at read deadline")
	case <-time.After(150 * time.Millisecond):
	}
	c.Close()
	select {
	case <-finished:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("disconnect did not cancel")
	}
}
func TestControlWaitLongReply(t *testing.T) {
	s := controlServer(t, nil)
	s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
		select {
		case <-ctx.Done():
			return controlError("cancelled", "early cancellation")
		case <-time.After(150 * time.Millisecond):
			return map[string]any{"ok": true}
		}
	})
	got := controlExchange(t, s, `{"op":"wait","timeout":1,"token":"test-control-token"}`+"\n")
	if got["ok"] != true {
		t.Fatalf("reply: %v", got)
	}
}
func TestControlWaitImmediateReply(t *testing.T) {
 s:=controlServer(t,func(json.RawMessage)any{return map[string]any{"ok":true}})
 for i:=0;i<10;i++ {controlExchange(t,s,`{"for":"finished","timeout":1,"op":"wait","tab":"t2","token":"test-control-token"}`+"\n")}
}
