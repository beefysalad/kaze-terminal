package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/ptyx"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

type controlSpike struct {
	win  *window
	srv  *notify.Server
	a, b *tabs.Tab
	ptys []*controlRecordPTY
	env  []map[string]string
}

func newControlSpike(t *testing.T) *controlSpike {
	t.Helper()
	win, _ := sideWindow(t, true)
	f := &controlSpike{win: win}
	win.mgr = tabs.NewManager(func(env []string, _ string) (ptyx.Session, error) {
		p := &controlRecordPTY{recPTY: newRecPTY(), writes: make(chan string, 8)}
		f.ptys = append(f.ptys, p)
		values := map[string]string{}
		for _, entry := range env {
			for i := range entry {
				if entry[i] == '=' {
					values[entry[:i]] = entry[i+1:]
					break
				}
			}
		}
		f.env = append(f.env, values)
		return p, nil
	}, 40, 3, nil)
	t.Cleanup(win.mgr.CloseAll)
	srv, err := notify.Listen(win.onNotify)
	if err != nil {
		t.Fatal(err)
	}
	f.srv = srv
	win.srv = srv
	srv.SetControlContext(controlContextAdapter(&win.control))
	t.Cleanup(func() { srv.Close() })
	win.mgr.SetNotify(srv.Addr(), srv.Token())
	f.a = openIn(t, win, "")
	f.b = openIn(t, win, "")
	win.publishControl()
	return f
}

// cmd/kaze is a main package and runControl is unexported. Use its exact
// JSON request/reply format and discovery environment against the real listener.
func (f *controlSpike) request(t *testing.T, req ctl.Request) ctl.Reply {
	t.Helper()
	env := f.env[0] // The caller is tab A, using its shell's discovery values.
	if env["KAZE_TAB_ID"] != fmt.Sprint(f.a.ID) {
		t.Fatalf("caller env: %q", env["KAZE_TAB_ID"])
	}
	conn, err := net.DialTimeout("tcp", env["KAZE_SOCKET"], time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	req.Token = env["KAZE_NOTIFY_TOKEN"]
	wire, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(wire); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(io.LimitReader(conn, (1<<20)+1))
	var reply ctl.Reply
	if err := dec.Decode(&reply); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("extra reply: %v", err)
	}
	return reply
}

func TestControlSpikeEndToEnd(t *testing.T) {
	f := newControlSpike(t)
	a, b := fmt.Sprintf("t%d", f.a.ID), fmt.Sprintf("t%d", f.b.ID)
	r := f.request(t, ctl.Request{Op: "ls"})
	if !r.OK || len(r.Tabs) != 2 || r.Tabs[0].ID != a || r.Tabs[1].ID != b {
		t.Fatalf("ls: %+v", r)
	}

	// Output comes through the fake PTY reader, not by feeding the parser directly.
	output := "\x1b[?2004hCodex ready\r\nsecond row"
	done := make(chan error, 1)
	go func() { _, err := f.ptys[1].w.Write([]byte(output)); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("PTY output timeout")
	}
	// Pipe.Write returning only means Read copied it. Wait for the applied output.
	deadline := time.Now().Add(time.Second)
	for {
		r = f.request(t, ctl.Request{Op: "read", Tab: b})
		if r.OK && r.Text == "Codex ready\nsecond row\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("read: %+v", r)
		}
		time.Sleep(time.Millisecond)
	}
	for _, text := range []string{"hello", "first line\nsecond line\nthird line"} {
		r = f.request(t, ctl.Request{Op: "send", Tab: b, Text: text})
		if !r.OK {
			t.Fatalf("send: %+v", r)
		}
		paste := "\x1b[200~" + strings.ReplaceAll(text, "\n", "\r") + "\x1b[201~"
		for _, want := range []string{paste, "\r"} {
			select {
			case got := <-f.ptys[1].writes:
				if got != want {
					t.Fatalf("PTY write %q, want %q", got, want)
				}
			case <-time.After(time.Second):
				t.Fatal("PTY input timeout")
			}
		}
	}
	if got := f.ptys[1].sent(); got != "\x1b[200~hello\x1b[201~\r\x1b[200~first line\rsecond line\rthird line\x1b[201~\r" {
		t.Fatalf("exact input: %q", got)
	}
	if got := f.ptys[0].sent(); got != "" {
		t.Fatalf("sender received input %q", got)
	}
	for _, op := range []string{"read", "send"} {
		r = f.request(t, ctl.Request{Op: op, Tab: "t9999", Text: "missing"})
		if r.OK || r.Error == nil || r.Error.Code != ctl.CodeUnknownTab {
			t.Fatalf("unknown %s: %+v", op, r)
		}
	}
	// Close before republishing to cover an in-flight request's stale snapshot.
	f.b.Session.Close()
	r = f.request(t, ctl.Request{Op: "send", Tab: b, Text: "closed"})
	if r.OK || r.Error == nil || r.Error.Code != ctl.CodeHostError {
		t.Fatalf("closed send: %+v", r)
	}
	f.win.mgr.Close(f.b.ID)
	f.win.publishControl()
	for _, op := range []string{"read", "send"} {
		r = f.request(t, ctl.Request{Op: op, Tab: b, Text: "removed"})
		if r.OK || r.Error == nil || r.Error.Code != ctl.CodeUnknownTab {
			t.Fatalf("removed %s: %+v", op, r)
		}
	}
	if len(f.ptys[1].writes) != 0 {
		t.Fatal("extra target writes")
	}
}
