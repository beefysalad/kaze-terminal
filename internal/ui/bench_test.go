package ui

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/bench"
)

func TestBenchDropsRealInputButRoutesSyntheticKeys(t *testing.T) {
	st := &recStarter{}
	win := testWindow(t, st)
	tab, err := win.mgr.New()
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"KAZE_BENCH": "1", "KAZE_BENCH_LOG": filepath.Join(t.TempDir(), "b.log"), "KAZE_BENCH_KEYS": "1"}
	rec := bench.FromEnv(func(k string) string { return vars[k] })
	t.Cleanup(func() { rec.Close(0) })
	win.bench = rec
	// The launch was long ago, so the one synthetic key is due now.
	past := time.Now().Add(-5 * time.Second)
	rec.FrameDone(past, past, bench.Frame{Drawn: true, HasText: true})

	real := []event.Event{
		key.Event{Name: key.NameReturn, State: key.Press},
		key.EditEvent{Text: "rm -rf"},
		transfer.DataEvent{Type: "application/text", Open: func() io.ReadCloser { return io.NopCloser(strings.NewReader("pasted")) }},
		pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary, Position: cellPos(1, 1)},
	}
	batch := win.benchBatch(layout.Context{}, tab, real, testCH)
	win.route(layout.Context{}, batch, testCW, testCH)

	st.mu.Lock()
	p := st.ptys[0]
	st.mu.Unlock()
	waitSent(t, p, "x") // the synthetic key, and nothing typed, pasted or pressed
	time.Sleep(50 * time.Millisecond)
	if got := p.sent(); got != "x" {
		t.Fatalf("shell got %q, want only the synthetic x", got)
	}
	if ts := win.sels[tab.ID]; ts != nil && (ts.dragging || ts.sel.Active) {
		t.Fatal("a real mouse press started a selection")
	}
}
