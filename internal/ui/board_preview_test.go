package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
)

// drawPreview draws the board wide at the current time, so the preview reads
// its transcript as a real frame would.
func drawPreview(win *window) layout.Context {
	gtx := pageContext(1400, 860)
	gtx.Now = time.Now()
	win.drawBoard(gtx, boardWide)
	return gtx
}

// previewScene is a paneWindow whose session a has agent "x" in state (see
// dockScene), with the board open on it.
func previewScene(t *testing.T, state string) (*window, subagents.Key) {
	t.Helper()
	win, _, r, a, _, _, _ := paneWindow(t)
	k := dockScene(t, win, int(a.ID), state)
	floatFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	drawPreview(win)
	return win, k
}

func joined(win *window) string { return strings.Join(win.board.texts, "") }

func TestBoardPreviewPickAnAgent(t *testing.T) {
	win, _, _ := liveWindow(t, 2)
	startAgent(win, win.mgr.Tabs()[1].ID, "s", "run")
	win.drainSubs(t0)
	win.openBoard(0)
	drawPreview(win)
	if !boardHas(win, "Pick an agent") || !boardHas(win, "Its steps, messages and report show here.") {
		t.Fatalf("texts %q", win.board.texts)
	}
	if win.board.prev.tr != nil {
		t.Fatal("a transcript was read with nothing selected")
	}
}

func TestBoardPreviewRunning(t *testing.T) {
	win, _ := previewScene(t, "running")
	for _, s := range []string{"RUNNING", "Explore", "Split pane keys", "In ", "Open in session", "ACTIVITY"} {
		if !boardHas(win, s) {
			t.Fatalf("no %q in %q", s, win.board.texts)
		}
	}
	if all := joined(win); !strings.Contains(all, "Looking at") || !strings.Contains(all, "panes.go") {
		t.Fatalf("no transcript note in %q", win.board.texts)
	}
	if boardHas(win, "Copy report") || win.board.openIn.Empty() {
		t.Fatalf("actions: copy shown or no Open in session (%v)", win.board.openIn)
	}
}

func TestBoardPreviewWaiting(t *testing.T) {
	win, _ := previewScene(t, "waiting")
	if !boardHas(win, "NEEDS YOU") {
		t.Fatalf("texts %q", win.board.texts)
	}
	found := false
	for _, s := range win.board.texts {
		found = found || strings.HasPrefix(s, "⏸ ")
	}
	if !found {
		t.Fatalf("no approval box in %q", win.board.texts)
	}
}

func TestBoardPreviewDone(t *testing.T) {
	win, _ := previewScene(t, "report")
	for _, s := range []string{"DONE", "REPORT", "Copy report", "Open in reader"} {
		if !boardHas(win, s) {
			t.Fatalf("no %q in %q", s, win.board.texts)
		}
	}
	if all := joined(win); !strings.Contains(all, "No existing binding") || strings.Contains(all, "**") {
		t.Fatalf("report not rendered: %q", win.board.texts)
	}
	if win.board.copy.Empty() || win.board.reader.Empty() {
		t.Fatal("no Copy report or Open in reader hit")
	}
}

func TestBoardPreviewParsesOnce(t *testing.T) {
	for _, state := range []string{"running", "report"} {
		win, _ := previewScene(t, state)
		n := win.board.prev.cache.parses
		if n == 0 {
			t.Fatalf("%s: nothing parsed", state)
		}
		for range 10 {
			drawPreview(win)
		}
		if win.board.prev.cache.parses != n {
			t.Fatalf("%s: %d parses after 10 unchanged frames, want %d", state, win.board.prev.cache.parses, n)
		}
	}
}

func TestBoardPreviewStopsPollingWhenDone(t *testing.T) {
	win, k := previewScene(t, "report")
	a := win.subs.Get(k)
	if win.board.prev.tr == nil {
		t.Fatal("the preview never read the transcript")
	}
	if _, again := win.dockTick(&win.board.prev, a, a.Ended.Add(time.Minute)); again {
		t.Fatal("a finished agent read to its end is still polled")
	}
}

func TestBoardCopyReportUsesPreviewTranscript(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl")
	writeTranscript(t, subagents.TranscriptPath(main, "x"), "Transcript says done")
	k := dockAgent(t, win, int(a.ID), main)
	stopAgent(win, int(a.ID), "")
	floatFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	gtx := drawPreview(win)
	var copied string
	win.toClipboard = func(s string) { copied = s }
	win.copyReport(gtx, win.subs.Get(k))
	if copied != "Transcript says done" {
		t.Fatalf("copied %q", copied)
	}
}

// Open in reader shows the transcript's report, and closing the board lets go
// of the preview's transcript.
func TestBoardReaderUsesTranscriptAndCloseDropsIt(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	main := filepath.Join(t.TempDir(), "s.jsonl")
	writeTranscript(t, subagents.TranscriptPath(main, "x"), "Transcript says done")
	k := dockAgent(t, win, int(a.ID), main)
	stopAgent(win, int(a.ID), "")
	floatFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	gtx := drawPreview(win)
	if win.board.prev.tr == nil || win.board.reader.Empty() {
		t.Fatal("fixture: no transcript or no Open in reader")
	}
	win.boardPress(gtx, centre(win.board.reader), 0)
	if win.board.open || win.board.prev.tr != nil {
		t.Fatalf("open %v, transcript kept %v", win.board.open, win.board.prev.tr != nil)
	}
	d := win.docs.list[len(win.docs.list)-1]
	var got string
	for _, b := range d.doc.Blocks {
		for _, s := range b.Spans {
			got += s.Text
		}
	}
	if got != "Transcript says done" {
		t.Fatalf("reader shows %q", got)
	}
}
