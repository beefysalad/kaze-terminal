package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

var tl0 = time.Date(2026, 11, 3, 14, 2, 0, 0, time.UTC)

func timelineKinds(items []dockItem) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString([]string{"dropped", "step", "note", "waiting", "live", "finished"}[it.kind])
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}

func avail(msgs ...subagents.TranscriptMessage) *subagents.Transcript {
	return &subagents.Transcript{Available: true, Messages: msgs}
}

func TestDockTimelineInterleaves(t *testing.T) {
	a := &subagents.Agent{State: subagents.Running, Started: tl0,
		Steps: []subagents.Step{{At: tl0, Tool: "Read", Summary: "a.go"}, {At: tl0.Add(2 * time.Second), Tool: "Grep", Summary: "auth"}}}
	tr := avail(subagents.TranscriptMessage{Text: "Now **auth**.", At: tl0.Add(time.Second)}, subagents.TranscriptMessage{Tools: subagents.ToolCounts{Reads: 1}, At: tl0})
	got := dockTimeline(a, tr)
	if k := timelineKinds(got); k != "step note step live" {
		t.Fatalf("kinds %q", k)
	}
	if got[0].tool != "Read" || got[0].target != "a.go" || got[1].text != "Now **auth**." || got[1].msg != 0 || got[0].msg != -1 {
		t.Fatalf("items %+v", got)
	}
}

func TestDockTimelineUntimedNote(t *testing.T) {
	a := &subagents.Agent{State: subagents.Running, Started: tl0,
		Steps: []subagents.Step{{At: tl0, Tool: "Read"}, {At: tl0.Add(5 * time.Second), Tool: "Grep"}}}
	tr := avail(subagents.TranscriptMessage{Text: "first", At: tl0.Add(time.Second)}, subagents.TranscriptMessage{Text: "untimed"})
	if k := timelineKinds(dockTimeline(a, tr)); k != "step note note step live" {
		t.Fatalf("kinds %q", k)
	}
}

func TestDockTimelineWaitingBash(t *testing.T) {
	a := &subagents.Agent{State: subagents.Waiting, Started: tl0,
		Steps: []subagents.Step{{At: tl0, Tool: "Bash", Summary: "go test ./internal/mdoc/"}}}
	got := dockTimeline(a, nil)
	last := got[len(got)-1]
	if last.kind != itemWaiting || last.text != "Wants to run go test ./internal/mdoc/. Approve it in the session above." {
		t.Fatalf("last %+v", last)
	}
	a.Steps[0].Tool = "Edit"
	if last := dockTimeline(a, nil); last[len(last)-1].text != "Waiting for your approval in the session above." {
		t.Fatalf("non-Bash wait %+v", last[len(last)-1])
	}
}

func TestDockTimelineDone(t *testing.T) {
	a := &subagents.Agent{State: subagents.Done, Started: tl0, Ended: tl0.Add(30 * time.Second),
		Steps: []subagents.Step{{At: tl0, Tool: "Read"}}}
	got := dockTimeline(a, nil)
	if k := timelineKinds(got); k != "step finished" {
		t.Fatalf("kinds %q", k)
	}
	if got[1].text != "Finished in <1m" {
		t.Fatalf("finished %q", got[1].text)
	}
}

func TestDockTimelineDropped(t *testing.T) {
	a := &subagents.Agent{State: subagents.Running, Dropped: 7, Steps: []subagents.Step{{At: tl0, Tool: "Read"}}}
	got := dockTimeline(a, &subagents.Transcript{Available: true, Dropped: true})
	if got[0].kind != itemDropped || got[0].text != "… 7 earlier steps" {
		t.Fatalf("first %+v", got[0])
	}
	if got[1].kind != itemDropped || got[1].text != "… earlier messages hidden" {
		t.Fatalf("second %+v", got[1])
	}
}

func TestDockTimelineCodexStepsOnly(t *testing.T) {
	a := &subagents.Agent{State: subagents.Running, Steps: []subagents.Step{{At: tl0, Tool: "Bash", Summary: "ls"}}}
	if k := timelineKinds(dockTimeline(a, nil)); k != "step live" {
		t.Fatalf("kinds %q", k)
	}
	if k := timelineKinds(dockTimeline(a, &subagents.Transcript{Messages: []subagents.TranscriptMessage{{Text: "x"}}})); k != "step live" {
		t.Fatalf("unavailable transcript used: %q", k)
	}
}
