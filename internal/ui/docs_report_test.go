package ui

import (
	"strings"
	"testing"
	"time"
)

func TestOpenReportDoc(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	d := win.openReportDoc(a.ID, "Explore · Find auth", "# Found\n\n**auth** lives in `mw.go`.")
	if d == nil || d.session != a.ID || d.name != "Explore · Find auth" || win.shownDoc() != d {
		t.Fatalf("report doc not open and shown: %+v", d)
	}
	floatFrames(t, win, r)
	all := strings.Join(win.docs.texts, "")
	if !strings.Contains(all, "Agent report") || !strings.Contains(all, "auth") || strings.Contains(all, "**") {
		t.Fatalf("drawn %q", all)
	}
	again := win.openReportDoc(a.ID, "Explore · Find auth", "new text")
	if again != d || len(win.docs.list) != 1 {
		t.Fatal("reopening made a second doc")
	}
}

func TestReportDocNoReload(t *testing.T) {
	win, _, _, a, _, _, _ := paneWindow(t)
	d := win.openReportDoc(a.ID, "Plan · x", "text")
	win.refreshDoc(d, time.Now())
	if d.removed {
		t.Fatal("a report with no file was marked removed")
	}
	if _, wake := win.docTick(time.Now().Add(time.Hour)); wake {
		t.Fatal("a report doc polls for file changes")
	}
}

func TestDockOpenInReader(t *testing.T) {
	win, _, r, a, _, _, _ := paneWindow(t)
	k := dockAgent(t, win, int(a.ID), "")
	stopAgent(win, int(a.ID), "the report")
	floatFrames(t, win, r)
	win.openDock(a.ID, k)
	floatFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.reader))
	if d := win.shownDoc(); d == nil || d.name != "Explore · Find auth" {
		t.Fatalf("reader not opened: %+v", d)
	}
}
