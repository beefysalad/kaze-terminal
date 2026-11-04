package ui

import (
	"image"
	"strconv"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

// rdBashRun is n finished Bash steps, each with a "$ command" detail.
func rdBashRun(n int) string {
	s := ""
	for i := range n {
		id := "b" + strconv.Itoa(i)
		s += rdUse(id, "Bash", map[string]any{"command": "ls " + strconv.Itoa(i)}) + rdResult(id, false, "out "+strconv.Itoa(i))
	}
	return s
}

func rdStepRect(h dockHits, id string) (image.Rectangle, bool) {
	for _, x := range h.steps {
		if x.id == id {
			return x.rect, true
		}
	}
	return image.Rectangle{}, false
}

// Opening a step in the middle of a followed list keeps the clicked row where
// it was: the panel stops following instead of snapping to the new bottom.
func TestOpenStepWhileFollowingKeepsRow(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdBashRun(40))
	s.follow = true
	twoFrames(t, win, r)
	if len(win.dock.hits.steps) < 3 {
		t.Fatalf("too few steps drawn: %+v", win.dock.hits.steps)
	}
	x := win.dock.hits.steps[1]
	clickAt(t, win, r, centre(x.rect))
	if !s.open[x.id] {
		t.Fatalf("step %q not open", x.id)
	}
	got, ok := rdStepRect(win.dock.hits, x.id)
	if !ok || got.Min.Y != x.rect.Min.Y {
		t.Fatalf("clicked row moved from y %d to %v (drawn %v)", x.rect.Min.Y, got, ok)
	}
	clickAt(t, win, r, centre(got))
	if s.open[x.id] {
		t.Fatalf("step %q still open", x.id)
	}
	if got, ok := rdStepRect(win.dock.hits, x.id); !ok || got.Min.Y != x.rect.Min.Y {
		t.Fatalf("closing moved the row from y %d to %v (drawn %v)", x.rect.Min.Y, got, ok)
	}
}

// A step row scrolled partly under the mode row only takes clicks inside the
// list: a press on the ACTIVITY label opens nothing.
func TestStepHitClippedToList(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdBashRun(40))
	s.scroll = gtxDp(win, 12)
	twoFrames(t, win, r)
	h := win.dock.hits
	for _, x := range h.steps {
		if x.rect.Overlaps(h.mode) || x.rect.Empty() {
			t.Fatalf("step %q hit %v reaches the mode row %v", x.id, x.rect, h.mode)
		}
	}
	p := image.Pt(h.mode.Min.X+gtxDp(win, 5), h.mode.Max.Y-gtxDp(win, 2))
	if win.readPress(s, &h, p) || len(s.open) != 0 {
		t.Fatalf("a press on the ACTIVITY label opened %v", s.open)
	}
}

// A step click ends a sidebar rename, as any other press in the panel does.
func TestStepClickEndsSidebarRename(t *testing.T) {
	win, r, a, _, _ := rdDock(t, rdTranscript())
	startRename(t, win, r, a.ID)
	clickAt(t, win, r, rdHit(t, win.dock.hits, "e1"))
	if win.side.renaming != 0 {
		t.Fatalf("rename still open after a step click: renaming %d", win.side.renaming)
	}
}

// A board too narrow for its preview holds no transcript, the read cache
// included.
func TestNarrowBoardDropsReadCache(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k, _ := readScene(t, win, int(a.ID), rdTranscript())
	twoFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	drawPreview(win)
	if win.board.prev.read.tr == nil {
		t.Fatal("the preview built nothing from its transcript")
	}
	gtx := pageContext(1400, 860)
	gtx.Now = time.Now()
	win.drawBoard(gtx, boardNarrow)
	if p := &win.board.prev; p.tr != nil || p.read.tr != nil || p.read.items != nil || p.read.every != nil {
		t.Fatalf("narrow board still holds the transcript: tr %v cache %v items %d", p.tr != nil, p.read.tr != nil, len(p.read.items))
	}
}

// A dock panel whose agent has no transcript path drops the read cache too.
func TestDockTickDropsReadCache(t *testing.T) {
	win, _, _, _, _, _, _ := memberWindow(t)
	tr := &subagents.Transcript{}
	s := &dockSession{tr: tr}
	s.read.tr, s.read.items = tr, []readItem{{kind: readNote, text: "x"}}
	win.dockTick(s, &subagents.Agent{Engine: "codex"}, time.Now())
	if s.tr != nil || s.read.tr != nil || s.read.items != nil {
		t.Fatalf("tr %v cache %v items %d", s.tr != nil, s.read.tr != nil, len(s.read.items))
	}
}
