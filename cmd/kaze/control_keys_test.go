package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

func TestControlKeysAndInterrupt(t *testing.T) {
	for _, args := range [][]string{
		{"keys", "review", "EnTeR", "ctrl+c", "up"},
		append([]string{"keys", "t1"}, strings.Fields(strings.Repeat("enter ", 64))...),
		{"interrupt", "t2"}, {"interrupt", "t2", "--ctrl-c"},
	} {
		env, requests := controlServer(t, control.Reply{OK: true})
		var out, errs bytes.Buffer
		handled, code := dispatchControl(args, env, nil, &out, &errs)
		if !handled || code != 0 || out.Len() != 0 || errs.Len() != 0 {
			t.Fatalf("%q: handled %v code %d out %q err %q", args, handled, code, out.String(), errs.String())
		}
		req := receiveControlRequest(t, requests)
		if req.Op != args[0] || req.Tab != args[1] || req.Token != "test-token" {
			t.Fatalf("request %+v", req)
		}
		if req.Op == "keys" && !reflect.DeepEqual(req.Args, args[2:]) {
			t.Fatalf("keys %+v", req)
		}
		if req.CtrlC != (req.Op == "interrupt" && len(args) == 3) {
			t.Fatalf("force %+v", req)
		}
	}
}

func TestControlKeyUsage(t *testing.T) {
	for _, args := range [][]string{
		{"keys"}, {"keys", "t1"}, {"keys", "", "enter"}, {"keys", "--json", "enter"},
		{"keys", "t1", "enter", "bogus"}, {"keys", "t1", ""}, {"keys", "t1", " enter"},
		append([]string{"keys", "t1"}, strings.Fields(strings.Repeat("enter ", 65))...),
		{"interrupt"}, {"interrupt", ""}, {"interrupt", "--ctrl-c"},
		{"interrupt", "t1", "--bad"}, {"interrupt", "t1", "--ctrl-c", "extra"},
	} {
		var out, errs bytes.Buffer
		called := false
		code := runControl(args, func(string) string { called = true; return "" }, nil, &out, &errs)
		if code != 2 || called || out.Len() != 0 || errs.Len() == 0 {
			t.Fatalf("%q: code %d env %v out %q err %q", args, code, called, out.String(), errs.String())
		}
		if args[0] == "keys" && len(args) == 4 && args[3] == "bogus" {
			for _, name := range strings.Split(control.AllowedKeys, ", ") {
				if !strings.Contains(errs.String(), name) {
					t.Fatalf("allowed name %q missing: %q", name, errs.String())
				}
			}
		}
	}
}

func TestControlKeysReplyExitCodes(t *testing.T) {
	for _, op := range []string{"keys", "interrupt"} {
		for _, tc := range []struct {
			wire string
			exit int
		}{
			{"unknown_key", 2}, {"unknown_tab", 1}, {"host_error", 1}, {"refused", 3}, {"timeout", 124},
		} {
			env, _ := controlServer(t, control.Reply{Error: &control.Error{Code: tc.wire, Message: "test explanation"}})
			args := []string{op, "t1"}
			if op == "keys" {
				args = append(args, "enter")
			}
			var out, errs bytes.Buffer
			if code := runControl(args, env, nil, &out, &errs); code != tc.exit || out.Len() != 0 || !strings.Contains(errs.String(), tc.wire) {
				t.Fatalf("%s %s: code %d out %q err %q", op, tc.wire, code, out.String(), errs.String())
			}
			if tc.wire == "unknown_key" && !strings.Contains(errs.String(), control.AllowedKeys) {
				t.Fatalf("missing allowlist: %q", errs.String())
			}
		}
	}
}
