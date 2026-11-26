package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/config"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
)

func TestWindowOrchestratorConfig(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		enabled, control, want bool
	}{
		{"disabled", false, true, false},
		{"enabled", true, true, true},
		{"control disabled", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			win, _ := workerWindow(t)
			win.opt.Config = config.Default()
			win.opt.Config.Orchestrator.Enabled = tc.enabled
			win.opt.Config.Control.Enabled = tc.control
			win.initOrchestratorConfig()
			r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "claude", Task: "review"})
			if r.OK != tc.want || (!tc.want && (r.Error == nil || r.Error.Code != ctl.CodeSpawnDisabled)) {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestWindowOrchestratorNoteQuiet(t *testing.T) {
	for _, seconds := range []int{0, 1, 5, 60} {
		t.Run(strconv.Itoa(seconds)+"s", func(t *testing.T) {
			win, _ := workerWindow(t)
			win.opt.Config = config.Default()
			win.opt.Config.Orchestrator.NoteQuiet = seconds
			win.initOrchestratorConfig()
			quiet := time.Duration(seconds) * time.Second
			if seconds == 0 {
				quiet = 5 * time.Second
			}
			if win.noteQuiet != quiet {
				t.Fatalf("quiet = %v, want %v", win.noteQuiet, quiet)
			}
			now := time.Now()
			win.noteAgent(notify.Msg{Kind: notify.KindSession, Agent: "claude"}, 1, 0)
			if err := win.control.EnqueueInbox("t1", "note", "a note"); err != nil {
				t.Fatal(err)
			}
			win.control.recordOwnerKey(1, now.Add(-quiet/2))
			if at, ok := win.noteTick(now, nil); !ok || !at.Equal(now.Add(quiet/2)) {
				t.Fatalf("wake %v %v, want %v", at, ok, now.Add(quiet/2))
			}
			if err := win.control.submitNote("t1", []byte("note")); err != errNoteDeferred {
				t.Fatalf("send-lock recheck: %v", err)
			}
			win.control.recordOwnerKey(1, now.Add(-quiet-time.Second))
			if at, ok := win.noteTick(now, nil); !ok || !at.Equal(now) {
				t.Fatalf("ready wake %v %v", at, ok)
			}
			if seconds == 1 {
				if err := win.control.submitNote("t1", []byte("note")); err != nil {
					t.Fatalf("configured quiet period elapsed: %v", err)
				}
			}
		})
	}
}
