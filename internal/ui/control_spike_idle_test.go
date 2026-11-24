package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestControlSpikeIdle(t *testing.T) {
	win, starter := sideWindow(t, true)
	win.opt.Config.CursorBlink = false
	var frames, hooks, requests atomic.Int64
	win.mgr = tabs.NewManager(starter.start, 80, 24, func() { frames.Add(1) })
	t.Cleanup(win.mgr.CloseAll)
	openIn(t, win, "")
	openIn(t, win, "")
	var ops op.Ops
	frame := func() {
		ops.Reset()
		if err := win.frame(layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1000, 700))}); err != nil {
			t.Fatal(err)
		}
	}
	for range 100 {
		frame()
	}
	before := testing.AllocsPerRun(50, frame)
	srv, err := notify.Listen(func(m notify.Msg) { hooks.Add(1); win.onNotify(m) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	win.srv = srv
	adapter := controlAdapter(&win.control)
	srv.SetControl(func(raw json.RawMessage) any { requests.Add(1); return adapter(raw) })
	win.mgr.SetNotify(srv.Addr(), srv.Token())
	win.publishControl()
	after := testing.AllocsPerRun(50, frame)
	if before != after {
		t.Fatalf("still frame allocations before=%g registered=%g", before, after)
	}
	t.Logf("still frame allocations before=%g registered=%g", before, after)

	// Give the newly created accept worker time to enter its blocking call.
	time.Sleep(20 * time.Millisecond)
	ids := controlWorkerIDs(t)
	if len(ids) < 9 {
		t.Fatalf("expected accept plus 4 workers per session, found %d", len(ids))
	}
	path := filepath.Join(t.TempDir(), "idle.trace")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := trace.Start(out); err != nil {
		out.Close()
		t.Fatal(err)
	}
	startFrames := frames.Load()
	time.Sleep(100 * time.Millisecond)
	endFrames := frames.Load()
	trace.Stop()
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if endFrames != startFrames || hooks.Load() != 0 || requests.Load() != 0 {
		t.Fatalf("idle frames=%d hooks=%d requests=%d", endFrames-startFrames, hooks.Load(), requests.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	parsed, err := exec.CommandContext(ctx, "go", "tool", "trace", "-d=parsed", path).CombinedOutput()
	if err != nil {
		t.Fatalf("decode trace: %v: %s", err, parsed)
	}
	// Fail if the trace format changes or a worker was missed. The test's
	// own bounded sleep supplies a positive wake-up control in this trace.
	allIDs := map[string]bool{}
	for _, m := range regexp.MustCompile(`GoID=([0-9]+)`).FindAllStringSubmatch(string(parsed), -1) {
		allIDs[m[1]] = true
	}
	for id := range ids {
		if !allIDs[id] {
			t.Fatalf("worker %s absent from trace", id)
		}
	}
	if countControlWakes(string(parsed), allIDs) == 0 {
		t.Fatal("trace missed positive wake-up control")
	}
	wakes := countControlWakes(string(parsed), ids)
	if wakes != 0 {
		t.Fatalf("idle worker wake-ups=%d", wakes)
	}
	t.Logf("100 ms idle: %d workers, %d wake-ups, %d frames, %d requests", len(ids), wakes, endFrames-startFrames, requests.Load())
}

// Inspect goroutine stacks to identify the listener and Session workers, rather
// than counting unrelated runtime, test runner or trace-reader activity.
func controlWorkerIDs(t *testing.T) map[string]bool {
	t.Helper()
	var stacks bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&stacks, 2); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, stack := range strings.Split(stacks.String(), "\n\n") {
		if !strings.Contains(stack, "internal/notify.(*Server).accept(") && !strings.Contains(stack, "internal/term.(*Session).") {
			continue
		}
		fields := strings.Fields(stack)
		if len(fields) > 1 && fields[0] == "goroutine" {
			ids[fields[1]] = true
		}
	}
	return ids
}

func countControlWakes(parsed string, ids map[string]bool) int {
	re := regexp.MustCompile(`GoID=([0-9]+).*(?:Waiting|Syscall)->(?:Runnable|Running)`)
	wakes := 0
	for _, line := range strings.Split(parsed, "\n") {
		m := re.FindStringSubmatch(line)
		if len(m) > 1 && ids[m[1]] {
			wakes++
		}
	}
	return wakes
}

func TestControlWakeCounter(t *testing.T) {
	ids := map[string]bool{"42": true}
	for _, transition := range []string{"Waiting->Runnable", "Syscall->Runnable", "Syscall->Running"} {
		input := "StateTransition GoID=42 " + transition + "\nStateTransition GoID=43 Waiting->Runnable\nStateTransition GoID=42 Running->Waiting"
		if got := countControlWakes(input, ids); got != 1 {
			t.Errorf("%s: wake count=%d, want 1", transition, got)
		}
	}
}
