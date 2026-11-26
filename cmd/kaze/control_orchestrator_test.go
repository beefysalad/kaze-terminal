package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func TestOrchestratorArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		op   string
	}{
		{[]string{"run", "claude", "review"}, "spawn"},
		{[]string{"run", "--name", "review", "--cwd", "/tmp", "--label", "a=b", "--label", "c=d", "--mode", "read-only", "--wait-start", "codex", "-"}, "spawn"},
		{[]string{"run", "my-template", "task"}, "spawn"},
		{[]string{"kill", "t2"}, "kill"}, {[]string{"kill", "--children"}, "kill"},
		{[]string{"report", "-"}, "report"}, {[]string{"ls", "--children", "--json"}, "ls"},
		{[]string{"ls", "--json", "--children"}, "ls"}, {[]string{"templates"}, "templates"},
	} {
		req, _, ok := controlArgs(tc.args)
		if !ok || req.Op != tc.op {
			t.Errorf("%v: %+v %v", tc.args, req, ok)
		}
		if len(tc.args) > 3 && tc.args[1] == "--name" && (req.Name != "review" || req.CWD != "/tmp" || req.Labels["a"] != "b" || req.Labels["c"] != "d" || req.Mode != "read-only" || !req.WaitStart || req.Task != "-") {
			t.Errorf("flags: %+v", req)
		}
	}
	for _, args := range [][]string{
		{"run"}, {"run", "claude"}, {"run", "claude", ""}, {"run", "claude", "x", "extra"},
		{"run", "--name"}, {"run", "--cwd"}, {"run", "--label"}, {"run", "--mode"},
		{"run", "--outlive-parent", "claude", "x"}, {"run", "--wat", "claude", "x"},
		{"run", "--mode", "unsafe", "claude", "x"}, {"run", "--label", "bad", "claude", "x"},
		{"run", "--label", "a=b", "--label", "a=c", "claude", "x"},
		{"run", "--name", "x", "--name", "y", "claude", "x"},
		{"run", "--name", "-bad", "claude", "x"}, {"run", "--cwd", "", "claude", "x"},
		{"run", "--wait-start", "--wait-start", "claude", "x"},
		{"kill"}, {"kill", "--bogus"}, {"kill", "t2", "t3"}, {"kill", "--children", "t2"},
		{"report"}, {"report", ""}, {"report", "x", "y"}, {"report", "--bad"},
		{"ls", "--children", "--children"}, {"templates", "extra"},
	} {
		if _, _, ok := controlArgs(args); ok {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestOrchestratorCLI(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		op, text string
	}{
		{[]string{"run", "claude", "-"}, "spawn", "started t2 (claude)"},
		{[]string{"kill", "t2"}, "kill", "closed t2"},
		{[]string{"report", "-"}, "report", ""},
	} {
		base, requests := controlServer(t, control.Reply{OK: true, Text: tc.text})
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
		handled, code := dispatchControl(tc.args, env, strings.NewReader("hello"), &out, &errs)
		if !handled || code != 0 || (tc.text != "" && out.String() != tc.text+"\n") {
			t.Fatalf("%v: %v %d %q %q", tc.args, handled, code, out.String(), errs.String())
		}
		req := receiveControlRequest(t, requests)
		if req.Op != tc.op || req.From != "t1" || req.CallerKey != "secret" {
			t.Fatalf("request %+v", req)
		}
		if req.Op == "spawn" && req.Task != "hello" || req.Op == "report" && req.Text != "hello" {
			t.Fatal(req)
		}
	}
}

func TestOrchestratorTextCaps(t *testing.T) {
	for _, tc := range []struct {
		op  string
		cap int
	}{{"run", 16 << 10}, {"report", 4 << 10}} {
		for _, stdin := range []bool{false, true} {
			text := strings.Repeat("x", tc.cap+1)
			arg := text
			if stdin {
				arg = "-"
			}
			args := []string{tc.op, arg}
			if tc.op == "run" {
				args = []string{"run", "claude", arg}
			}
			var out, errs bytes.Buffer
			code := runControl(args, func(string) string { t.Error("env accessed for oversize"); return "" }, strings.NewReader(text), &out, &errs)
			if code != 2 || !strings.Contains(errs.String(), "text_too_large") {
				t.Fatalf("%s: %d %s", tc.op, code, errs.String())
			}
		}
	}
}

func TestOrchestratorExitCodes(t *testing.T) {
	for _, tc := range []struct {
		code string
		exit int
	}{{"spawn_disabled", 3}, {"limit", 3}, {"rate_limited", 3}, {"not_allowed", 3}, {"bad_cwd", 2}, {"unknown_agent", 2}, {"unknown_template", 2}, {"text_too_large", 2}, {"spawn_failed", 1}, {"timeout", 124}, {"consent_denied", 3}, {"consent_timeout", 124}} {
		env, _ := controlServer(t, control.Reply{Error: &control.Error{Code: tc.code, Message: "test"}})
		var out, errs bytes.Buffer
		if code := runControl([]string{"ls"}, env, nil, &out, &errs); code != tc.exit {
			t.Errorf("%s: %d", tc.code, code)
		}
	}
}

func TestOrchestratorChildren(t *testing.T) {
	for _, fail := range []bool{false, true} {
		srv, err := notify.Listen(nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { srv.Close() })
		calls := make(chan string, 4)
		srv.SetControl(func(raw json.RawMessage) any {
			var req control.Request
			if err := json.Unmarshal(raw, &req); err != nil {
				return control.Reply{}
			}
			select {
			case calls <- req.Op + ":" + req.Tab:
			default:
				t.Error("unexpected extra request")
				return control.Reply{Error: &control.Error{Code: "host_error", Message: "fixture request bound"}}
			}
			if req.Op == "ls" {
				return control.Reply{OK: true, Tabs: []control.TabInfo{{ID: "t1"}, {ID: "t2", Parent: "t1", Role: "worker"}, {ID: "t3", Parent: "t9", Role: "worker"}, {ID: "t4", Parent: "t1", Role: "worker"}}}
			}
			if fail {
				return control.Reply{Error: &control.Error{Code: "not_allowed", Message: "test"}}
			}
			return control.Reply{OK: true, Text: "closed " + req.Tab}
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
		code := runControl([]string{"kill", "--children"}, env, nil, &out, &errs)
		wantCode := 0
		wantCalls := 3
		wantOut := "closed t2\nclosed t4\n"
		if fail {
			wantCode = 3
			wantCalls = 2
			wantOut = ""
		}
		if code != wantCode || len(calls) != wantCalls || out.String() != wantOut {
			t.Fatalf("%d %d %q %s", code, len(calls), out.String(), errs.String())
		}
		for i, want := range []string{"ls:", "kill:t2", "kill:t4"} {
			if i == wantCalls {
				break
			}
			if got := <-calls; got != want {
				t.Fatal(got, want)
			}
		}
		out.Reset()
		errs.Reset()
		if code := runControl([]string{"ls", "--children", "--json"}, env, nil, &out, &errs); code != 0 {
			t.Fatal(code, errs.String())
		}
		var r control.Reply
		if err := json.Unmarshal(out.Bytes(), &r); err != nil || len(r.Tabs) != 2 || r.Tabs[0].ID != "t2" || r.Tabs[1].ID != "t4" {
			t.Fatalf("%s %v", out.String(), err)
		}
	}
}

func TestOrchestratorHelpAndTemplates(t *testing.T) {
	var out, errs bytes.Buffer
	env, requests := controlServer(t, control.Reply{OK: true, Text: "no templates (put <name>.md in /tmp/templates)"})
	handled, code := dispatchControl([]string{"templates"}, env, nil, &out, &errs)
	if !handled || code != 0 || out.String() != "no templates (put <name>.md in /tmp/templates)\n" {
		t.Fatalf("%v %d %s %s", handled, code, &out, &errs)
	}
	if req := receiveControlRequest(t, requests); req.Op != "templates" {
		t.Fatal(req)
	}
	out.Reset()
	dispatchControl([]string{"--help"}, func(string) string { return "" }, nil, &out, &errs)
	for _, word := range []string{"run ", "kill ", "report ", "--children", "templates", "--wait-start", "--label", "--mode", "--cwd", "--name"} {
		if !strings.Contains(out.String(), word) {
			t.Errorf("help missing %q", word)
		}
	}
}

func TestOrchestratorFlagTextDoesNotChangeRawSend(t *testing.T) {
	env, requests := controlServer(t, control.Reply{OK: true})
	var out, errs bytes.Buffer
	if code := runControl([]string{"send", "t2", "--", "--children", "--raw"}, env, nil, &out, &errs); code != 2 {
		t.Fatalf("trailing literal should be extra arg: %d", code)
	}
	base := env
	env = func(k string) string {
		if k == "KAZE_TAB_ID" {
			return "1"
		}
		return base(k)
	}
	if code := runControl([]string{"send", "--raw", "t2", "--", "--children"}, env, nil, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if req := receiveControlRequest(t, requests); req.From != "" {
		t.Fatal("raw sender gained caller tag", req)
	}
}

func TestOrchestratorStdinBoundaries(t *testing.T) {
	for _, tc := range []struct {
		op  string
		cap int
	}{{"run", 16 << 10}, {"report", 4 << 10}} {
		for _, size := range []int{tc.cap, tc.cap + 1, tc.cap + 200} {
			input := strings.NewReader(strings.Repeat("x", size))
			env, requests := controlServer(t, control.Reply{OK: true})
			base := env
			env = func(k string) string {
				if k == "KAZE_TAB_ID" {
					return "t1"
				}
				return base(k)
			}
			args := []string{tc.op, "-"}
			if tc.op == "run" {
				args = []string{"run", "claude", "-"}
			}
			var out, errs bytes.Buffer
			code := runControl(args, env, input, &out, &errs)
			if size == tc.cap {
				if code != 0 {
					t.Fatal(code, errs.String())
				}
				req := receiveControlRequest(t, requests)
				if len(req.Text)+len(req.Task) != tc.cap {
					t.Fatal("changed stdin")
				}
			} else if code != 2 || input.Len() != size-tc.cap-1 {
				t.Fatalf("%s size %d code %d remaining %d", tc.op, size, code, input.Len())
			}
		}
	}
}

func TestRunFlagOrders(t *testing.T) {
	flags := []string{"--name", "reviewer", "--cwd", "/tmp", "--label", "a=b", "--label", "c=d", "--mode", "edit", "--wait-start"}
	for _, agent := range []string{"codex", "claude", "my-template"} {
		for _, after := range []bool{false, true} {
			args := []string{"run"}
			if after {
				args = append(args, agent)
			}
			args = append(args, flags...)
			if !after {
				args = append(args, agent)
			}
			args = append(args, "task")
			r, _, ok := controlArgs(args)
			if !ok || r.Agent != agent || r.Task != "task" || r.Name != "reviewer" || r.CWD != "/tmp" || r.Mode != "edit" || !r.WaitStart || len(r.Labels) != 2 {
				t.Errorf("%v: %+v, %v", args, r, ok)
			}
		}
	}
	for _, task := range []string{"--name", "-task"} {
		if r, _, ok := controlArgs([]string{"run", "codex", "--", task}); !ok || r.Task != task {
			t.Errorf("literal task %q: %+v %v", task, r, ok)
		}
	}
	for _, tail := range [][]string{
		{"--name", "a", "--name", "b", "task"}, {"--cwd", "a", "--cwd", "b", "task"},
		{"--mode", "edit", "--mode", "edit", "task"}, {"--wait-start", "--wait-start", "task"},
		{"--label", "a=b", "--label", "a=c", "task"}, {"--name", "a"}, {"--wait-start"}, {"-task"},
	} {
		args := append([]string{"run", "codex"}, tail...)
		if _, _, ok := controlArgs(args); ok {
			t.Errorf("accepted %v", args)
		}
	}
	for _, args := range [][]string{
		{"run", "codex", "--wat", "task"}, {"run", "--wat", "codex", "task"},
		{"run", "codex", "task", "--wat"},
	} {
		var out, errs bytes.Buffer
		code := runControl(args, func(string) string { t.Error("bad flags accessed env"); return "" }, nil, &out, &errs)
		if code != 2 || !strings.Contains(errs.String(), "--wat") || !strings.Contains(errs.String(), "before or immediately after") {
			t.Errorf("%v: %d %q", args, code, errs.String())
		}
	}
	if !strings.Contains(controlUsage, "before or immediately after") {
		t.Error("help missing accepted order")
	}
}

func TestTemplatesCLIListAndFailure(t *testing.T) {
	for _, tt := range []struct {
		reply control.Reply
		code  int
		text  string
	}{
		{control.Reply{OK: true, Text: "review  codex  Review changes\nwriter  claude  Write docs", Warning: "broken: missing agent"}, 0, "review  codex  Review changes\nwriter  claude  Write docs\n"},
		{control.Reply{Error: &control.Error{Code: control.CodeDisabled, Message: "disabled"}}, 3, ""},
	} {
		env, requests := controlServer(t, tt.reply)
		var out, errs bytes.Buffer
		if code := runControl([]string{"templates"}, env, nil, &out, &errs); code != tt.code || out.String() != tt.text {
			t.Fatalf("%d %q %q", code, &out, &errs)
		}
		if req := receiveControlRequest(t, requests); req.Op != "templates" {
			t.Fatal(req)
		}
		if tt.reply.Warning != "" && !strings.Contains(errs.String(), tt.reply.Warning) {
			t.Fatal(&errs)
		}
	}
}
