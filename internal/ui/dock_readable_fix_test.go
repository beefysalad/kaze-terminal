package ui

import (
	"image"
	"image/color"
	"strconv"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/theme"
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

// Messages trimmed or a skipped line are not "earlier steps hidden".
func TestStepsHiddenOnlyWhenStepsDropped(t *testing.T) {
	tr := rdTr(subagents.TranscriptStep{Tool: "Read", Path: "a.go"})
	tr.Dropped = true
	for _, it := range readableItems(rdAgent(), tr) {
		if it.text == "… earlier steps hidden" {
			t.Fatalf("steps banner with no step dropped: %+v", it)
		}
	}
	tr.StepsDropped = true
	if items := readableItems(rdAgent(), tr); items[0].text != "… earlier steps hidden" {
		t.Fatalf("no steps banner: %+v", items[0])
	}
}

// Test runners are matched on command words, not substrings.
func TestTestCommandMatchesWords(t *testing.T) {
	for cmd, want := range map[string]bool{
		"go test ./...":                         true,
		"cd x && go vet ./... && go test ./ui/": true,
		"npx jest --watch=false":                true,
		"python -m pytest -q":                   true,
		"npm test":                              true,
		"npm run test":                          true,
		"pnpm test":                             true,
		"yarn test":                             true,
		"cargo test --all":                      true,
		"npx vitest run":                        true,
		"cat jest.config.js":                    false,
		"ls pytest_cache":                       false,
		"vim vitest.config.ts":                  false,
		"git log --grep 'go testing'":           false,
		"grep -r cargo src/test":                false,
		"echo gotest":                           false,
	} {
		if got := testCommand(cmd); got != want {
			t.Errorf("testCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}

// The battery's ink follows any theme change, not only Chrome and Folder.
func TestBatteryInkCacheFollowsTheme(t *testing.T) {
	c := newChrome()
	c.setTheme(theme.All()[0])
	var k batteryInkCache
	first := k.get(c, 15, false)
	th := c.th
	th.Needs = color.NRGBA{R: 0x10, G: 0x80, B: 0xff, A: 0xff}
	c.th = th
	if got, want := k.get(c, 15, false), c.batteryInk(15, false); got != want {
		t.Fatalf("ink %+v after a Needs change, want %+v (was %+v)", got, want, first)
	}
}
