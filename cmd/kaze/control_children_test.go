package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

func TestControlChildrenWaitArgs(t *testing.T) {
	for _, mode := range []string{"all", "any"} {
		r, _, _, ok := controlArgsWithRaw([]string{"wait", "--children", mode, "--timeout", "3"})
		if !ok || r.Children != mode || r.Timeout != 3 || r.Tab != "" {
			t.Fatalf("%+v %v", r, ok)
		}
	}
	for _, args := range [][]string{{"wait", "--children"}, {"wait", "--children", "bad"}, {"wait", "--children", "all", "--for", "x"}, {"wait", "--children", "all", "--quiet", "2"}, {"wait", "--children", "any", "--timeout", "601"}} {
		if _, _, _, ok := controlArgsWithRaw(args); ok {
			t.Fatal(args)
		}
	}
}

func TestControlChildrenWaitWire(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		r := control.Reply{OK: true, Reason: "note", Text: "worker note", Tabs: []control.TabInfo{{ID: "t2", Status: "needs", Name: "review"}}}
		want := 0
		if timeout {
			r.OK = false
			r.Reason = "timeout"
			r.Text = ""
			r.Error = &control.Error{Code: "timeout", Message: "wait timeout"}
			want = 124
		}
		base, requests := controlServer(t, r)
		env := func(k string) string {
			if k == "KAZE_TAB_ID" {
				return "1"
			}
			if k == "KAZE_TAB_KEY" {
				return "secret"
			}
			return base(k)
		}
		var out, errs bytes.Buffer
		code := runControl([]string{"wait", "--children", "all", "--timeout", "2"}, env, nil, &out, &errs)
		if code != want || !strings.Contains(out.String(), "t2\tneeds\treview\n") || !strings.Contains(out.String(), r.Reason+"\n") {
			t.Fatalf("%d %q %q", code, out.String(), errs.String())
		}
		req := receiveControlRequest(t, requests)
		if req.From != "t1" || req.CallerKey != "secret" || req.Children != "all" || req.Timeout != 2 {
			t.Fatal(req)
		}
	}
}
