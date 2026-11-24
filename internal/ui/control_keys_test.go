package ui

import (
	"strings"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/term"
	"github.com/beefysalad/kaze-terminal/internal/vt"
)

func expectControlWrites(t *testing.T, p *controlRecordPTY, wants ...string) {
	t.Helper()
	for _, want := range wants {
		select {
		case got := <-p.writes:
			if got != want {
				t.Fatalf("write %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("key write timeout")
		}
	}
	if len(p.writes) != 0 {
		t.Fatal("extra key write")
	}
}

func TestControlKeysBytes(t *testing.T) {
	f := newControlSpike(t)
	keys := []string{"EnTeR", "ESC", "tab", "up", "down", "left", "right", "backspace", "CTRL+C", "ctrl+d", "ctrl+l"}
	// Keep the recording channel large enough for the whole bounded request.
	f.ptys[1].writes = make(chan string, 64)
	for _, app := range []bool{false, true} {
		seq, prefix := "\x1b[?1l", "\x1b["
		if app {
			seq, prefix = "\x1b[?1h", "\x1bO"
		}
		f.b.Session.Lock()
		vt.NewParser(f.b.Session.Applier()).Feed([]byte(seq))
		f.b.Session.Unlock()
		if err := f.win.control.Keys("t2", keys); err != nil {
			t.Fatal(err)
		}
		expectControlWrites(t, f.ptys[1], "\r", "\x1b", "\t", prefix+"A", prefix+"B", prefix+"D", prefix+"C", "\x7f", "\x03", "\x04", "\x0c")
	}
}

func TestControlInterruptKinds(t *testing.T) {
	f := newControlSpike(t)
	for _, agent := range []string{"", "claude", "codex", "other"} {
		// Publish an immutable snapshot; socket methods must never inspect window state.
		old := f.win.control.snapshot.Load()
		next := &controlSnapshot{tabs: append([]ctl.TabInfo(nil), old.tabs...), sessions: old.sessions}
		next.tabs[1].Agent = agent
		f.win.control.snapshot.Store(next)
		for _, force := range []bool{false, true} {
			want := "\x03"
			if !force && (agent == "claude" || agent == "codex") {
				want = "\x1b"
			}
			if err := f.win.control.Interrupt("t2", force); err != nil {
				t.Fatal(err)
			}
			expectControlWrites(t, f.ptys[1], want)
		}
	}
}

func TestControlKeysStopOnSubmitFailure(t *testing.T) {
	for _, failure := range []term.SubmitResult{term.Closed, term.QueueFull} {
		var got []string
		err := submitControlKeys([]string{"enter", "esc", "tab"}, term.Modes{}, func(b []byte) term.SubmitResult {
			got = append(got, string(b))
			if len(got) == 2 {
				return failure
			}
			return term.Accepted
		})
		if err == nil || strings.Join(got, "") != "\r\x1b" {
			t.Fatalf("failure %s: writes %q error %v", failure, got, err)
		}
	}
}

func TestControlKeysInterruptEndToEnd(t *testing.T) {
	f := newControlSpike(t)
	for _, req := range []ctl.Request{
		{Op: "keys", Tab: "t2", Args: []string{"up", "ENTER", "ctrl+c"}},
		{Op: "interrupt", Tab: "t2"},
	} {
		if r := f.request(t, req); !r.OK {
			t.Fatalf("%s: %+v", req.Op, r)
		}
	}
	expectControlWrites(t, f.ptys[1], "\x1b[A", "\r", "\x03", "\x03")
	if f.ptys[0].sent() != "" {
		t.Fatal("keys reached sender")
	}
	for _, tc := range []struct {
		req  ctl.Request
		code string
	}{
		{ctl.Request{Op: "keys", Tab: "t2", Args: []string{"enter", "bad"}}, ctl.CodeUnknownKey},
		{ctl.Request{Op: "keys", Tab: "t2", Args: strings.Fields(strings.Repeat("enter ", 65))}, ctl.CodeInvalidRequest},
		{ctl.Request{Op: "keys", Tab: "t999", Args: []string{"enter"}}, ctl.CodeUnknownTab},
		{ctl.Request{Op: "interrupt", Tab: "t999"}, ctl.CodeUnknownTab},
	} {
		r := f.request(t, tc.req)
		if r.OK || r.Error == nil || r.Error.Code != tc.code {
			t.Fatalf("%+v: %+v", tc.req, r)
		}
	}
	f.b.Session.Close()
	for _, op := range []string{"keys", "interrupt"} {
		r := f.request(t, ctl.Request{Op: op, Tab: "t2", Args: []string{"enter", "tab"}})
		if r.OK || r.Error == nil || r.Error.Code != ctl.CodeHostError {
			t.Fatalf("closed %s: %+v", op, r)
		}
	}
	f.win.mgr.Close(f.b.ID)
	f.win.publishControl()
	for _, op := range []string{"keys", "interrupt"} {
		r := f.request(t, ctl.Request{Op: op, Tab: "t2", Args: []string{"enter"}})
		if r.OK || r.Error == nil || r.Error.Code != ctl.CodeUnknownTab {
			t.Fatalf("removed %s: %+v", op, r)
		}
	}
	if len(f.ptys[1].writes) != 0 {
		t.Fatal("refused request wrote keys")
	}
}
