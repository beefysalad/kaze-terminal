package ui

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/vt"
)

// Reproducible issue #101 probe. Includes the whole frame so the sidebar's
// improvement cannot hide extra work moved into another drawing path.
func TestSidebarUnchangedFrameProbe(t *testing.T) {
	win, _ := sideWindow(t, true)
	win.opt.Config.CursorBlink = false
	for i := 0; i < 7; i++ {
		tab := openIn(t, win, osPath(fmt.Sprintf("/code/repo%d/work", i%3)))
		tab.Session.Lock()
		p := vt.NewParser(tab.Session.Applier())
		for j := 0; j < 3000; j++ {
			p.Feed([]byte("probe output line\r\n"))
		}
		tab.Session.Unlock()
		for j := 0; j < 3; j++ {
			win.subs.Apply(subagents.Event{Kind: subagents.EvStart, Tab: int(tab.ID), AgentID: fmt.Sprintf("%d-%d", i, j), AgentType: "Explore", Desc: "Inspect sidebar", At: time.Now()})
		}
	}
	var ops op.Ops
	frame := func() {
		ops.Reset()
		if err := win.frame(layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1600, 1000))}); err != nil {
			t.Fatal(err)
		}
	}
	for range 200 {
		frame()
	}
	b := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			frame()
		}
	})
	t.Logf("1600x1000 unchanged frame: %s, %s", b.String(), b.MemString())
}
