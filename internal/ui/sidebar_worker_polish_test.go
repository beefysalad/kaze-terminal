package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/font"
	"gioui.org/io/input"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

func TestSidebarWorkerParentDisplayName(t *testing.T) {
	for _, tc := range []struct {
		name, title, want string
		missing           bool
	}{
		{name: "Lead reviewer", title: "Terminal title", want: "Lead reviewer"},
		{title: "Terminal title", want: "Terminal title"},
		{want: "t1"},
		{title: "Terminal title", want: "t1", missing: true},
		{name: strings.Repeat("Long parent ", 20), want: strings.Repeat("Long parent ", 20)},
	} {
		t.Run(tc.want[:min(len(tc.want), 24)], func(t *testing.T) {
			parent, child := sideTab(1, tc.title), sideTab(2, "helper")
			child.Role, child.Parent = "worker", 1
			ts := []*tabs.Tab{parent, child}
			if tc.missing {
				ts = ts[1:]
			}
			in := sideIn(ts, nil)
			in.Name = func(id tabs.TabID) string {
				if id == 1 {
					return tc.name
				}
				return ""
			}
			row, ok := rowByID(buildSide(in), 2)
			if !ok || row.Detail != "worker of "+tc.want {
				t.Fatalf("detail %q, want %q", row.Detail, "worker of "+tc.want)
			}
		})
	}
}

func TestSidebarWorkerDetailFitsWithEllipsis(t *testing.T) {
	c := newChrome()
	g := pageContext(260, 640)
	for _, room := range []int{0, 1, 60, 120, 180} {
		_, sz, shown := c.fitSideText(g, "worker of "+strings.Repeat("Long parent ", 20), 10, room, c.sideTheme().Muted, font.Normal)
		if sz.X > room {
			t.Fatalf("width %d exceeds room %d", sz.X, room)
		}
		if shown != "" && !strings.HasSuffix(shown, "…") {
			t.Fatalf("truncated detail %q lacks ellipsis", shown)
		}
	}
}

func TestSidebarWorkerCompactHeight(t *testing.T) {
	g := pageContext(260, 640)
	for _, mk := range []mark{markNone, markDone, markNeeds} {
		rows := []agentRow{{ID: 1, State: agentWorking}, {ID: 2, Parent: 1, Worker: true, Mark: mk}}
		want := g.Dp(sideHeadH + sideAgentLabelH + 2*sideAgentRowH + 6)
		if mk == markNeeds {
			want += g.Dp(16)
		}
		if got := agentsSpan(g, rows); got != want {
			t.Fatalf("mark %v: span %d, want %d", mk, got, want)
		}
	}
}

func TestSidebarAgentColumns(t *testing.T) {
	g := pageContext(260, 640)
	rect := image.Rect(8, 0, 251, 30)
	parent := sidebarAgentColumns(g, rect, false)
	child := sidebarAgentColumns(g, rect, true)
	if child.name-parent.name != g.Dp(16) {
		t.Fatal("worker must use one indent step")
	}
	if child.right != parent.right || child.ageRight != parent.ageRight || child.dot != parent.dot {
		t.Fatal("right columns moved for worker")
	}
	if child.name != child.logo+g.Dp(13+8) {
		t.Fatal("name is not aligned after logo")
	}
}

func TestSidebarWorkerRailTooltipFits(t *testing.T) {
	c := newChrome()
	g := pageContext(560, 640)
	for _, right := range []bool{false, true} {
		for _, room := range []int{120, 484} {
			tip := railTipFor{title: "review", word: "worker of " + strings.Repeat("Long parent ", 20), tile: image.Rect(8, 80, 44, 116)}
			box := c.railTip(g, tip, image.Pt(52, 600), right, room, c.sideInk())
			if box.Dx() > room || (!right && (box.Min.X < 52 || box.Max.X > 52+room)) || (right && (box.Min.X < -room || box.Max.X > 0)) {
				t.Fatalf("right %v, room %d: tooltip %v overflows", right, room, box)
			}
		}
	}
}

func TestSidebarWorkerRailStillFrame(t *testing.T) {
	win := railWindow(t)
	parent := openIn(t, win, osPath("/code/kaze/app"))
	child := openIn(t, win, "")
	child.Role, child.Parent = "worker", parent.ID
	win.names = map[tabs.TabID]string{parent.ID: strings.Repeat("Long parent ", 20)}
	win.workers = map[tabs.TabID]*worker{child.ID: {parent: parent.ID, agent: "codex", firstMessage: "delivered"}}
	var router input.Router
	sideFrame(t, win, &router)
	g := pageContext(560, 640)
	g.Now = t0
	draw := func() {
		g.Ops.Reset()
		win.drawSide(g, image.Rect(0, 42, railW, 608))
		win.drawSideTip(g)
	}
	draw()
	win.side.st.hover = child.ID
	for range 3 {
		draw()
	}
	if win.side.hits.tip.Empty() {
		t.Fatal("worker tooltip missing")
	}
	builds, draws := win.side.cache.builds, win.side.cache.draws
	if n := testing.AllocsPerRun(50, draw); n != 0 {
		t.Fatalf("worker rail still frame allocated %g", n)
	}
	if win.side.cache.builds != builds || win.side.cache.draws != draws {
		t.Fatal("worker rail rebuilt while idle")
	}
	router.Frame(g.Ops)
	if _, wake := router.WakeupTime(); wake {
		t.Fatal("worker rail scheduled an idle wake")
	}
}
