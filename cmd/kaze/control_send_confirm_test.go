package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

// A successful send must say where it went, so an agent that ran it knows the
// message was delivered instead of guessing from silence.
func TestSendConfirmsTheTarget(t *testing.T) {
	reply := control.Reply{OK: true, Status: "idle", Tabs: []control.TabInfo{{ID: "t2", Agent: "codex", Name: "reviewer", Status: "idle"}}}
	env, _ := controlServer(t, reply)
	var out, errs bytes.Buffer
	if code := runControl([]string{"send", "codex", "hi"}, env, strings.NewReader(""), &out, &errs); code != 0 || errs.Len() != 0 {
		t.Fatalf("code=%d err=%q", code, errs.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sent to t2 (codex, reviewer, idle)" {
		t.Fatalf("stdout %q", got)
	}
}

func TestSendConfirmationOmitsEmptyParts(t *testing.T) {
	reply := control.Reply{OK: true, Tabs: []control.TabInfo{{ID: "t5", Status: "shell"}}}
	env, _ := controlServer(t, reply)
	var out bytes.Buffer
	if code := runControl([]string{"send", "t5", "hi"}, env, strings.NewReader(""), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if got := strings.TrimSpace(out.String()); got != "sent to t5 (shell)" {
		t.Fatalf("stdout %q", got)
	}
}
