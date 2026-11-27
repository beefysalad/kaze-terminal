package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func TestControlSendHTMLFitsWireLimit(t *testing.T) {
	srv, err := notify.Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	requests := make(chan control.Request, 1)
	srv.SetControl(func(raw json.RawMessage) any {
		var req control.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return control.Reply{Error: &control.Error{Code: "invalid_request", Message: err.Error()}}
		}
		requests <- req
		return control.Reply{OK: true}
	})
	env := func(key string) string {
		switch key {
		case "KAZE_SOCKET":
			return srv.Addr()
		case "KAZE_NOTIFY_TOKEN":
			return srv.Token()
		}
		return ""
	}
	text := strings.Repeat("<>&", 10000)
	var out, errs bytes.Buffer
	if code := runControl([]string{"send", "t2", text}, env, nil, &out, &errs); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errs.String())
	}
	if got := receiveControlRequest(t, requests).Text; got != text {
		t.Fatalf("received %d bytes, want %d", len(got), len(text))
	}
}

func TestControlRejectsOversizedEncodedRequestBeforeDial(t *testing.T) {
	for _, text := range []string{strings.Repeat("\"", control.MaxTextBytes), strings.Repeat("\x00", 12000)} {
		env := func(key string) string {
			switch key {
			case "KAZE_SOCKET":
				return "invalid-address" // No connection should be attempted.
			case "KAZE_NOTIFY_TOKEN":
				return "test-token"
			}
			return ""
		}
		var out, errs bytes.Buffer
		code := runControl([]string{"send", "t2", text}, env, nil, &out, &errs)
		if code != 2 || !strings.Contains(errs.String(), "encoded request exceeds 64 KiB") || out.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errs.String())
		}
	}
}

// Each fixture owns its listener and bounds accept, read, write and cleanup.
func controlServer(t *testing.T, reply control.Reply) (func(string) string, <-chan control.Request) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := ln.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	requests := make(chan control.Request, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
		var req control.Request
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		requests <- req
		if err := json.NewEncoder(conn).Encode(reply); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		select {
		case <-done:
		case <-time.After(9 * time.Second):
			t.Error("server did not finish")
		}
	})
	return func(key string) string {
		switch key {
		case "KAZE_SOCKET":
			return ln.Addr().String()
		case "KAZE_NOTIFY_TOKEN":
			return "test-token"
		}
		return ""
	}, requests
}

func receiveControlRequest(t *testing.T, requests <-chan control.Request) control.Request {
	t.Helper()
	select {
	case req := <-requests:
		return req
	case <-time.After(time.Second):
		t.Fatal("missing request")
		return control.Request{}
	}
}

func TestControlList(t *testing.T) {
	reply := control.Reply{OK: true, Tabs: []control.TabInfo{
		{ID: "t1", Name: "review", Space: "project", Agent: "codex", Status: "working", Folder: "/tmp/my project", Parent: "t9", Role: "worker"},
		{ID: "t2", Space: "Shell", Status: "shell", Folder: "/tmp"},
	}}
	for _, asJSON := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "json"}[asJSON], func(t *testing.T) {
			env, requests := controlServer(t, reply)
			args := []string{"ls"}
			if asJSON {
				args = append(args, "--json")
			}
			var out, errs bytes.Buffer
			if code := runControl(args, env, nil, &out, &errs); code != 0 || errs.Len() != 0 {
				t.Fatalf("code=%d stderr=%q", code, errs.String())
			}
			if asJSON {
				want, _ := json.Marshal(reply)
				if out.String() != string(want)+"\n" {
					t.Fatalf("json=%q", out.String())
				}
			} else if out.String() != "t1\treview\tproject\tcodex\tworking\t/tmp/my project\t-\tt9\tworker\t-\nt2\t-\tShell\t-\tshell\t/tmp\t-\t-\t-\t-\n" {
				t.Fatalf("plain=%q", out.String())
			}
			select {
			case req := <-requests:
				if req.Op != "ls" || req.Token != "test-token" {
					t.Fatalf("request=%+v", req)
				}
			case <-time.After(time.Second):
				t.Fatal("no request")
			}
		})
	}
}

func TestControlRead(t *testing.T) {
	for _, lines := range []string{"", "12"} {
		env, requests := controlServer(t, control.Reply{OK: true, Text: "first  \nsecond\n\n"})
		args := []string{"read", "t2"}
		if lines != "" {
			args = append(args, "--lines", lines)
		}
		var out, errs bytes.Buffer
		if code := runControl(args, env, nil, &out, &errs); code != 0 || errs.Len() != 0 || out.String() != "first  \nsecond\n\n" {
			t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), errs.String())
		}
		req := receiveControlRequest(t, requests)
		want := 0
		if lines != "" {
			want = 12
		}
		if req.Op != "read" || req.Tab != "t2" || req.Lines != want {
			t.Fatalf("request=%+v", req)
		}
	}
}

func TestControlSend(t *testing.T) {
	for _, arg := range []string{"hello\nworld", "-"} {
		env, requests := controlServer(t, control.Reply{OK: true})
		var out, errs bytes.Buffer
		if code := runControl([]string{"send", "review", arg}, env, strings.NewReader("hello\nworld"), &out, &errs); code != 0 || out.Len() != 0 || errs.Len() != 0 {
			t.Fatalf("code=%d out=%q err=%q", code, out.String(), errs.String())
		}
		req := receiveControlRequest(t, requests)
		if req.Op != "send" || req.Tab != "review" || req.Text != "hello\nworld" {
			t.Fatalf("request=%+v", req)
		}
	}
}

func TestControlMissingEnv(t *testing.T) {
	for _, missing := range []string{"KAZE_SOCKET", "KAZE_NOTIFY_TOKEN"} {
		var out, errs bytes.Buffer
		env := func(key string) string {
			if key == missing {
				return ""
			}
			if key == "KAZE_SOCKET" {
				return "127.0.0.1:1"
			}
			if key == "KAZE_NOTIFY_TOKEN" {
				return "test-token"
			}
			return ""
		}
		code := runControl([]string{"ls"}, env, nil, &out, &errs)
		if code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), missing) {
			t.Fatalf("missing %s: code=%d out=%q err=%q", missing, code, out.String(), errs.String())
		}
	}
}

func TestControlOversize(t *testing.T) {
	for _, stdin := range []bool{false, true} {
		text := strings.Repeat("x", control.MaxTextBytes+1)
		arg := text
		if stdin {
			arg = "-"
		}
		called := false
		env := func(string) string { called = true; return "" }
		var out, errs bytes.Buffer
		code := runControl([]string{"send", "t1", arg}, env, strings.NewReader(text), &out, &errs)
		if code != 2 || called || out.Len() != 0 || !strings.Contains(errs.String(), "32 KiB") {
			t.Fatalf("stdin=%v code=%d envCalled=%v out=%q err=%q", stdin, code, called, out.String(), errs.String())
		}
	}
}

func TestControlErrorReply(t *testing.T) {
	for _, tc := range []struct {
		wire string
		exit int
	}{
		{"unknown_tab", 1}, {"unauthorized", 1}, {"control_unavailable", 1}, {"refused", 3}, {"busy_server", 3}, {"not_allowed", 3}, {"timeout", 124},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			env, _ := controlServer(t, control.Reply{Error: &control.Error{Code: tc.wire, Message: "test explanation"}})
			var out, errs bytes.Buffer
			code := runControl([]string{"ls", "--json"}, env, nil, &out, &errs)
			if code != tc.exit || out.Len() != 0 || !strings.Contains(errs.String(), tc.wire+": test explanation") {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), errs.String())
			}
		})
	}
}

func TestControlUsage(t *testing.T) {
	for _, args := range [][]string{
		nil, {"ls", "extra"}, {"ls", "--json", "--json"},
		{"read"}, {"read", ""}, {"read", "--json"}, {"read", "t1", "--lines"},
		{"read", "t1", "--lines", "-1"}, {"read", "t1", "--lines", "2001"},
		{"read", "t1", "--lines", "bad"}, {"read", "t1", "--bogus", "1"},
		{"send"}, {"send", "t1"}, {"send", "", "hello"}, {"send", "--json", "hello"},
		{"send", "t1", "hello", "extra"}, {"send", "t1", "-"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errs bytes.Buffer
			called := false
			code := runControl(args, func(string) string { called = true; return "" }, nil, &out, &errs)
			if code != 2 || called || out.Len() != 0 || errs.Len() == 0 {
				t.Fatalf("code=%d envCalled=%v out=%q err=%q", code, called, out.String(), errs.String())
			}
		})
	}
}

func TestControlReplyTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.(*net.TCPListener).SetDeadline(time.Now().Add(8 * time.Second))
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
		var req control.Request
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		select {
		case <-stop:
		case <-time.After(8 * time.Second):
		}
	}()
	t.Cleanup(func() {
		close(stop)
		ln.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server cleanup timed out")
		}
	})
	env := func(key string) string {
		if key == "KAZE_SOCKET" {
			return ln.Addr().String()
		}
		if key == "KAZE_NOTIFY_TOKEN" {
			return "test-token"
		}
		return ""
	}
	var out, errs bytes.Buffer
	start := time.Now()
	code := runControl([]string{"ls"}, env, nil, &out, &errs)
	elapsed := time.Since(start)
	if code != 124 || out.Len() != 0 || !strings.Contains(errs.String(), "timeout") || elapsed < 4500*time.Millisecond || elapsed > 7*time.Second {
		t.Fatalf("code=%d elapsed=%s out=%q err=%q", code, elapsed, out.String(), errs.String())
	}
}

func TestControlUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	env := func(key string) string {
		if key == "KAZE_SOCKET" {
			return addr
		}
		if key == "KAZE_NOTIFY_TOKEN" {
			return "test-token"
		}
		return ""
	}
	var out, errs bytes.Buffer
	start := time.Now()
	code := runControl([]string{"ls"}, env, nil, &out, &errs)
	if code != 1 || time.Since(start) > 6*time.Second || out.Len() != 0 || errs.Len() == 0 {
		t.Fatalf("code=%d elapsed=%s out=%q err=%q", code, time.Since(start), out.String(), errs.String())
	}
}

func TestControlDispatch(t *testing.T) {
	for _, op := range []string{"ls", "read", "send"} {
		t.Run(op, func(t *testing.T) {
			var out, errs bytes.Buffer
			// Only a leading subcommand is handled. A workspace may have the same name.
			for _, args := range [][]string{nil, {"--workspace", op}, {"--workspace=" + op}, {"notify", op}, {"statusline", op}, {"--version", op}} {
				called := false
				handled, code := dispatchControl(args, func(string) string { called = true; return "" }, nil, &out, &errs)
				if handled || code != 0 || called || out.Len() != 0 || errs.Len() != 0 {
					t.Fatalf("args=%q handled=%v code=%d envCalled=%v", args, handled, code, called)
				}
				if len(args) > 0 && strings.HasPrefix(args[0], "--workspace") {
					name, code := workspaceArg(args, &errs)
					if name != op || code != 0 {
						t.Fatalf("workspace=%q code=%d", name, code)
					}
				}
			}
			handled, _ := dispatchControl([]string{op}, func(string) string { return "" }, nil, &out, &errs)
			if !handled {
				t.Fatalf("%s was not dispatched", op)
			}
		})
	}
}

func TestControlEnvironmentAndLimits(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		base, requests := controlServer(t, control.Reply{OK: true})
		env := func(key string) string {
			if key == "KAZE_SOCKET" && fallback {
				return ""
			}
			if key == "KAZE_NOTIFY" {
				if fallback {
					return base("KAZE_SOCKET")
				}
				return "127.0.0.1:1"
			}
			return base(key)
		}
		text := strings.Repeat("x", control.MaxTextBytes)
		var out, errs bytes.Buffer
		if code := runControl([]string{"send", "t1", "-"}, env, strings.NewReader(text), &out, &errs); code != 0 || out.Len() != 0 || errs.Len() != 0 {
			t.Fatalf("fallback=%v code=%d out=%q err=%q", fallback, code, out.String(), errs.String())
		}
		select {
		case req := <-requests:
			if req.Text != text {
				t.Fatal("boundary text changed")
			}
		case <-time.After(time.Second):
			t.Fatal("missing request")
		}
	}
}

func TestControlWait(t *testing.T) {
	for _, code := range []int{0, 124, 1} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			reply := control.Reply{OK: true}
			if code == 124 {
				reply = control.Reply{Error: &control.Error{Code: "timeout", Message: "wait timeout"}}
			}
			if code == 1 {
				reply = control.Reply{Error: &control.Error{Code: "unknown_tab", Message: "unknown tab"}}
			}
			env, requests := controlServer(t, reply)
			var out, errs bytes.Buffer
			handled, got := dispatchControl([]string{"wait", "t2", "--for", "ready", "--timeout", "600", "--quiet", "200"}, env, nil, &out, &errs)
			if !handled || got != code {
				t.Fatalf("handled %v code %d stderr %s", handled, got, errs.String())
			}
			req := receiveControlRequest(t, requests)
			if req.Op != "wait" || req.Tab != "t2" || req.For != "ready" || req.Timeout != 600 || req.Quiet != 200 {
				t.Fatalf("%+v", req)
			}
		})
	}
	for _, args := range [][]string{{"wait"}, {"wait", "t1", "--timeout", "601"}, {"wait", "t1", "--timeout", "0"}, {"wait", "t1", "--quiet", "-1"}, {"wait", "t1", "--for", ""}, {"wait", "t1", "--wat", "1"}, {"wait", "t1", "--quiet", "1", "--quiet", "2"}} {
		if _, _, ok := controlArgs(args); ok {
			t.Fatalf("accepted %v", args)
		}
	}
	req, _, ok := controlArgs([]string{"wait", "t1"})
	if !ok || req.Timeout != 120 || req.Quiet != 1500 {
		t.Fatalf("defaults %+v %v", req, ok)
	}
}

func TestControlReadTruncationWarning(t *testing.T) {
	env, _ := controlServer(t, control.Reply{OK: true, Text: "newest answer\n", Truncated: true})
	var out, errs bytes.Buffer
	code := runControl([]string{"read", "t2"}, env, nil, &out, &errs)
	if code != 0 || out.String() != "newest answer\n" || !strings.Contains(errs.String(), "truncated") || !strings.Contains(errs.String(), "newest") {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), errs.String())
	}
}

func TestControlCopiesCallerKeyWithoutChangingLabelKey(t *testing.T) {
	env, requests := controlServer(t, control.Reply{OK: true})
	withKey := func(name string) string {
		if name == "KAZE_TAB_KEY" {
			return "owner-key"
		}
		return env(name)
	}
	var out, errs bytes.Buffer
	if code := runControl([]string{"label", "t2", "team=review"}, withKey, nil, &out, &errs); code != 0 {
		t.Fatalf("code=%d %s", code, errs.String())
	}
	req := receiveControlRequest(t, requests)
	wire, _ := json.Marshal(req)
	var fields map[string]any
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["caller_key"] != "owner-key" || fields["key"] != "team" {
		t.Fatalf("request fields: %s", wire)
	}
}

func TestControlSpaceContext(t *testing.T) {
	for _, args := range [][]string{{"read", "t2"}, {"keys", "t2", "enter"}, {"interrupt", "t2"}, {"wait", "t2", "--timeout", "1"}} {
		for _, caller := range []string{"2", ""} {
			base, requests := controlServer(t, control.Reply{OK: true})
			env := func(k string) string {
				if k == "KAZE_TAB_ID" {
					return caller
				}
				return base(k)
			}
			var out, errs bytes.Buffer
			if code := runControl(args, env, nil, &out, &errs); code != 0 {
				t.Fatalf("%v: %d %s", args, code, &errs)
			}
			want := ""
			if caller != "" {
				want = "t2"
			}
			if req := receiveControlRequest(t, requests); req.From != want {
				t.Fatalf("%v: %+v", args, req)
			}
		}
	}
}
func TestControlLsMarksSelf(t *testing.T) {
	base, _ := controlServer(t, control.Reply{OK: true, Tabs: []control.TabInfo{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}})
	env := func(k string) string {
		if k == "KAZE_TAB_ID" {
			return "2"
		}
		return base(k)
	}
	var out, errs bytes.Buffer
	if code := runControl([]string{"ls"}, env, nil, &out, &errs); code != 0 {
		t.Fatalf("%d %s", code, &errs)
	}
	for i, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		want := "-"
		if i == 1 {
			want = "self"
		}
		if len(fields) != 10 || fields[9] != want {
			t.Fatalf("row %d: %q", i, line)
		}
	}
}
func TestControlSendConfirmationShowsSpace(t *testing.T) {
	base, _ := controlServer(t, control.Reply{OK: true, Tabs: []control.TabInfo{{ID: "t2", Agent: "codex", Status: "idle", Space: "kaze-terminal"}}, Warning: "cross-space test warning"})
	var out, errs bytes.Buffer
	if code := runControl([]string{"send", "t2", "review"}, base, nil, &out, &errs); code != 0 || out.String() != "sent to t2 (codex, idle, space kaze-terminal)\n" || !strings.Contains(errs.String(), "kaze: warning: cross-space test warning") {
		t.Fatalf("%d %q %q", code, &out, &errs)
	}
	if !strings.Contains(controlUsage, "Address tabs by id from ls. ls marks your tab self; tabs in another space belong to another project.") {
		t.Fatal("usage lacks space guidance")
	}
}

// Exercise the CLI wire format and real handler together with a bounded fake host.
func TestControlCrossSpaceScript(t *testing.T) {
	host := &controlSpaceScriptHost{tabs: []control.TabInfo{
		{ID: "t1", Agent: "claude", Space: "A", Status: "idle"},
		{ID: "t2", Agent: "codex", Space: "A", Status: "idle"},
		{ID: "t7", Agent: "codex", Space: "B", Status: "idle"},
	}, submitted: make(chan string, 1)}
	srv, err := notify.Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	srv.SetControl(func(raw json.RawMessage) any {
		var req control.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return control.Reply{Error: &control.Error{Code: "invalid_request", Message: err.Error()}}
		}
		return control.HandleContext(context.Background(), host, req, control.Options{Enabled: true, AgentTag: true, CrossSpace: true})
	})
	env := func(k string) string {
		switch k {
		case "KAZE_SOCKET":
			return srv.Addr()
		case "KAZE_NOTIFY_TOKEN":
			return srv.Token()
		case "KAZE_TAB_ID":
			return "1"
		}
		return ""
	}
	for _, step := range []struct{ target, id, warning string }{{"codex", "t2", ""}, {"t7", "t7", "target t7 is in space B, not your space A; use it only if the owner named that tab"}} {
		var out, errs bytes.Buffer
		if code := runControl([]string{"send", step.target, "review"}, env, nil, &out, &errs); code != 0 {
			t.Fatalf("%+v: %d %s", step, code, &errs)
		}
		if !strings.HasPrefix(out.String(), "sent to "+step.id+" (codex, idle, space ") {
			t.Fatalf("confirmation: %s", &out)
		}
		wantErr := ""
		if step.warning != "" {
			wantErr = "kaze: warning: " + step.warning + "\n"
		}
		if errs.String() != wantErr {
			t.Fatalf("warning: %q", &errs)
		}
		select {
		case id := <-host.submitted:
			if id != step.id {
				t.Fatalf("submitted to %s", id)
			}
		case <-time.After(time.Second):
			t.Fatal("no submission")
		}
	}
}

func TestControlCrossSpaceRefusedByDefault(t *testing.T) {
	host := &controlSpaceScriptHost{tabs: []control.TabInfo{
		{ID: "t1", Agent: "claude", Space: "A", Status: "idle"},
		{ID: "t7", Agent: "codex", Space: "B", Status: "idle"},
	}, submitted: make(chan string, 1)}
	srv, err := notify.Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	srv.SetControl(func(raw json.RawMessage) any {
		var req control.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return control.Reply{Error: &control.Error{Code: "invalid_request", Message: err.Error()}}
		}
		return control.Handle(host, req)
	})
	env := func(k string) string {
		switch k {
		case "KAZE_SOCKET":
			return srv.Addr()
		case "KAZE_NOTIFY_TOKEN":
			return srv.Token()
		case "KAZE_TAB_ID":
			return "1"
		}
		return ""
	}
	var out, errs bytes.Buffer
	if code := runControl([]string{"send", "t7", "review"}, env, nil, &out, &errs); code == 0 {
		t.Fatalf("cross-space send succeeded: %s", &out)
	}
	if !strings.Contains(errs.String(), "another space") {
		t.Fatalf("stderr: %q", &errs)
	}
	select {
	case id := <-host.submitted:
		t.Fatalf("submitted to %s", id)
	case <-time.After(100 * time.Millisecond):
	}
}

type controlSpaceScriptHost struct {
	control.Host
	tabs      []control.TabInfo
	submitted chan string
}

func (h *controlSpaceScriptHost) Tabs() []control.TabInfo          { return h.tabs }
func (h *controlSpaceScriptHost) Submit(id string, _ []byte) error { h.submitted <- id; return nil }

func TestControlCLISendsCallerForEveryOp(t *testing.T) {
	srv, err := notify.Listen(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	got := make(chan control.Request, 1)
	srv.SetControl(func(raw json.RawMessage) any {
		var req control.Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return control.Reply{Error: &control.Error{Code: "invalid_request", Message: err.Error()}}
		}
		got <- req
		return control.Reply{OK: true, Tabs: []control.TabInfo{{ID: "t2", Agent: "codex", Status: "idle"}}}
	})
	env := func(k string) string {
		switch k {
		case "KAZE_SOCKET":
			return srv.Addr()
		case "KAZE_NOTIFY_TOKEN":
			return srv.Token()
		case "KAZE_TAB_ID":
			return "1"
		}
		return ""
	}
	for _, tc := range []struct {
		args []string
		raw  bool
	}{
		{[]string{"ls"}, false},
		{[]string{"name", "t2", "reviewer"}, false},
		{[]string{"label", "t2", "k=v"}, false},
		{[]string{"send", "t2", "hi", "--raw"}, true},
	} {
		var out, errs bytes.Buffer
		runControl(tc.args, env, nil, &out, &errs)
		select {
		case req := <-got:
			if req.From != "t1" || req.Raw != tc.raw {
				t.Fatalf("%v: from %q raw %v", tc.args, req.From, req.Raw)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%v: no request: %s", tc.args, &errs)
		}
	}
}
