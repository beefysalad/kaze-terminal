package main

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

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

func TestControlList(t *testing.T) {
	reply := control.Reply{OK: true, Tabs: []control.TabInfo{
		{ID: "t1", Name: "review", Space: "project", Agent: "codex", Status: "working", Folder: "/tmp/my project"},
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
			} else if out.String() != "t1\treview\tproject\tcodex\tworking\t/tmp/my project\nt2\t-\tShell\t-\tshell\t/tmp\n" {
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
		req := <-requests
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
		req := <-requests
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
		{"unknown_tab", 1}, {"unauthorized", 1}, {"control_unavailable", 1}, {"refused", 3}, {"timeout", 124},
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
