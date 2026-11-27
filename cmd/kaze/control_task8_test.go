package main

import (
	"bytes"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"slices"
	"strings"
	"testing"
)

func TestControlTagFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		from string
		idle bool
	}{
		{[]string{"send", "t2", "hello"}, "t1", false},
		{[]string{"send", "t2", "hello", "--raw"}, "t1", false},
		{[]string{"send", "--if-idle", "t2", "hello"}, "t1", true},
		{[]string{"send", "t2", "--raw", "hello", "--if-idle"}, "t1", true},
	} {
		base, requests := controlServer(t, ctl.Reply{OK: true, Warning: "busy"})
		env := func(k string) string {
			if k == "KAZE_TAB_ID" {
				return "1"
			}
			return base(k)
		}
		var out, errs bytes.Buffer
		if c := runControl(tc.args, env, nil, &out, &errs); c != 0 || !strings.Contains(errs.String(), "busy") {
			t.Fatalf("exit %d stderr %q", c, errs.String())
		}
		r := receiveControlRequest(t, requests)
		raw := slices.Contains(tc.args, "--raw")
		if r.From != tc.from || r.Raw != raw || r.IfIdle != tc.idle || r.Text != "hello" || r.Tab != "t2" {
			t.Fatalf("%+v", r)
		}
	}
	for _, code := range []string{"busy", "disabled"} {
		env, _ := controlServer(t, ctl.Reply{Error: &ctl.Error{Code: code, Message: "refused"}})
		var out, errs bytes.Buffer
		if c := runControl([]string{"send", "t1", "hello", "--if-idle"}, env, nil, &out, &errs); c != 3 {
			t.Fatalf("%s exit %d stderr %s", code, c, errs.String())
		}
	}
}
func TestControlMetadataCLI(t *testing.T) {
	for _, args := range [][]string{{"name", "t2", "review"}, {"label", "t2", "role=review=one"}} {
		env, requests := controlServer(t, ctl.Reply{OK: true})
		var out, errs bytes.Buffer
		handled, c := dispatchControl(args, env, nil, &out, &errs)
		if !handled || c != 0 {
			t.Fatalf("%v handled %v exit %d stderr %s", args, handled, c, errs.String())
		}
		r := receiveControlRequest(t, requests)
		if r.Tab != "t2" || (r.Op == "name" && r.Name != "review") || (r.Op == "label" && (r.Key != "role" || r.Value != "review=one")) {
			t.Fatal(r)
		}
	}
	for _, args := range [][]string{{"name"}, {"name", "t1", ""}, {"name", "t1", "t3"}, {"label", "t1", "noequals"}, {"label", "t1", "=v"}, {"label", "t1", "bad key=v"}, {"send", "--raw", "--raw", "t1", "hi"}, {"send", "t1", "hi", "--bogus"}, {"send", "t1", "hi", "--if-idle", "--if-idle"}} {
		var out, errs bytes.Buffer
		called := false
		c := runControl(args, func(string) string { called = true; return "" }, nil, &out, &errs)
		if c != 2 || called {
			t.Fatalf("accepted %v exit %d", args, c)
		}
	}
}
func TestControlListLabels(t *testing.T) {
	reply := ctl.Reply{OK: true, Tabs: []ctl.TabInfo{{ID: "t1", Labels: map[string]string{"role": "review", "a": "b"}}, {ID: "t2"}}}
	for _, args := range [][]string{{"ls"}, {"ls", "--json"}} {
		env, _ := controlServer(t, reply)
		var out, errs bytes.Buffer
		if c := runControl(args, env, nil, &out, &errs); c != 0 {
			t.Fatal(c, errs.String())
		}
		if len(args) == 1 {
			if !strings.Contains(out.String(), "a=b,role=review\t-\t-\t-\n") || !strings.HasSuffix(out.String(), "\t-\n") {
				t.Fatalf("plain %q", out.String())
			}
		} else if !strings.Contains(out.String(), `"labels":{"a":"b","role":"review"}`) {
			t.Fatalf("json %s", out.String())
		}
	}
}

func TestControlNameAddressability(t *testing.T) {
	for _, name := range []string{"-review", " -review "} {
		called := false
		var out, errs bytes.Buffer
		code := runControl([]string{"name", "t2", name}, func(string) string { called = true; return "" }, nil, &out, &errs)
		if code != 2 || called || !strings.Contains(errs.String(), "hyphen") {
			t.Fatalf("name %q: code %d env %v stderr %q", name, code, called, errs.String())
		}
	}
	// A permitted internal hyphen remains usable in both creation and addressing.
	for _, args := range [][]string{{"name", "t2", "peer-review"}, {"read", "peer-review"}} {
		env, requests := controlServer(t, ctl.Reply{OK: true})
		var out, errs bytes.Buffer
		if code := runControl(args, env, nil, &out, &errs); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errs.String())
		}
		req := receiveControlRequest(t, requests)
		if args[0] == "name" && req.Name != "peer-review" || args[0] == "read" && req.Tab != "peer-review" {
			t.Fatal(req)
		}
	}
}
