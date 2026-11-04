package ui

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/subagents"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/theme"
)

// Every text of the readable Activity reaches 4.5:1 on the surface it is
// drawn on, in every theme, the diff lines on their diff backgrounds too.
func TestReadInkContrast(t *testing.T) {
	for _, th := range theme.All() {
		c := newChrome()
		c.setTheme(th)
		bg := th.Term.BG
		ink := c.readInk(bg)
		low := 99.0
		check := func(label string, fg, on color.NRGBA) {
			got := contrast(fg, on)
			low = min(low, got)
			if got < 4.5 {
				t.Errorf("%s: %s contrast %.2f", th.Key, label, got)
			}
		}
		for label, fg := range map[string]color.NRGBA{"say": ink.say, "step": ink.step, "strong": ink.strong, "mono": ink.mono,
			"meta": ink.meta, "add": ink.add, "del": ink.del, "ok": ink.ok, "bad": ink.bad} {
			check(label+" on the panel", fg, bg)
		}
		for label, fg := range map[string]color.NRGBA{"dim": ink.dim, "mono": ink.detailMono, "list": ink.list} {
			check(label+" on the detail", fg, ink.detailBG)
		}
		check("added line", ink.addFG, ink.addBG)
		check("removed line", ink.delFG, ink.delBG)
		if ink.sayBar != claudeCoral {
			t.Errorf("%s: note bar %v, want the Claude coral", th.Key, ink.sayBar)
		}
		t.Logf("%s: lowest readable contrast %.2f", th.Key, low)
	}
}

func rdLine(kind string, blocks ...map[string]any) string {
	b, _ := json.Marshal(map[string]any{"type": kind, "cwd": "/w/kaze", "message": map[string]any{"content": blocks}})
	return string(b) + "\n"
}

func rdUse(id, name string, input map[string]any) string {
	return rdLine("assistant", map[string]any{"type": "tool_use", "id": id, "name": name, "input": input})
}

func rdResult(id string, isErr bool, out string) string {
	return rdLine("user", map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isErr, "content": out})
}

const rdBashCmd = "cd kaze-lane-d && go vet ./... && go test ./internal/ui/"

// rdTranscript is the readable fixture: a note, three reads, an edit
// (+8 −5) and a failed Bash with a description. 5 steps, 1 message.
func rdTranscript() string {
	old := "ctx1\nctx0\no1\no2\no3\no4\no5\nend0\nend1"
	nw := "ctx1\nctx0\nn1\nn2\nn3\nn4\nn5\nn6\nn7\nn8\nend0\nend1"
	return rdLine("assistant", map[string]any{"type": "text", "text": "Starting D6: removing the scratch shell."}) +
		rdUse("r1", "Read", map[string]any{"file_path": "/w/kaze/internal/ui/agent.go"}) +
		rdUse("r2", "Read", map[string]any{"file_path": "/w/kaze/internal/ui/window.go"}) +
		rdUse("r3", "Read", map[string]any{"file_path": "/w/kaze/internal/ui/scratch_gone_test.go"}) +
		rdUse("e1", "Edit", map[string]any{"file_path": "/w/kaze/internal/ui/scratch_gone_test.go", "old_string": old, "new_string": nw}) +
		rdResult("e1", false, "ok") +
		rdUse("b1", "Bash", map[string]any{"command": rdBashCmd, "description": "Run go vet and the ui tests"}) +
		rdResult("b1", true, "Exit code 1\n--- FAIL: TestOldScratchKeyIsTheShells (0.01s)\n    scratch_gone_test.go:26: boom\nFAIL\tgithub.com/x/ui\t4.8s")
}

// readScene starts agent "x" in tab a's session with transcript as its
// transcript and the fixture's hook steps, and returns its key and the
// transcript's path.
func readScene(t testing.TB, win *window, a int, transcript string) (subagents.Key, string) {
	main := filepath.Join(t.TempDir(), "s.jsonl")
	path := subagents.TranscriptPath(main, "x")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(transcript), 0o644)
	win.liveMgr.Store(win.mgr)
	send := func(m notify.Msg) { m.Tab, m.Session = a, "s"; win.onNotify(m) }
	send(notify.Msg{Kind: notify.KindTool, Tool: "Agent", AgentType: "general-purpose", Description: "Lane D: dock tabs #133", TaskPrompt: "Remove the scratch shell."})
	send(notify.Msg{Kind: notify.KindAgentStart, AgentID: "x", AgentType: "general-purpose", Transcript: main})
	for _, s := range [][2]string{{"Read", "internal/ui/agent.go"}, {"Read", "internal/ui/window.go"}, {"Read", "internal/ui/scratch_gone_test.go"},
		{"Edit", "internal/ui/scratch_gone_test.go"}, {"Bash", rdBashCmd}} {
		send(notify.Msg{Kind: notify.KindTool, AgentID: "x", AgentType: "general-purpose", Tool: s[0], Summary: s[1]})
	}
	return subagents.Key{Session: "s", ID: "x"}, path
}

// rdDock is a window with the fixture's agent open in a tall panel.
func rdDock(t *testing.T, transcript string) (*window, *input.Router, *tabs.Tab, *dockSession, string) {
	t.Helper()
	win, _, r, a, _, _, _ := memberWindow(t)
	win.dock.heightDp = 440
	k, path := readScene(t, win, int(a.ID), transcript)
	win.openDock(a.ID, k)
	s := win.dockOf(a.ID)
	s.follow = false
	twoFrames(t, win, r)
	return win, r, a, s, path
}

func rdHas(texts []string, want string) bool {
	for _, s := range texts {
		if s == want {
			return true
		}
	}
	return false
}

func TestDockShowsReadableSteps(t *testing.T) {
	win, _, _, _, _ := rdDock(t, rdTranscript())
	all := strings.Join(win.dock.texts, " ")
	if !strings.Contains(all, "Starting") || !strings.Contains(all, "scratch") {
		t.Fatalf("note missing: %q", win.dock.texts)
	}
	for _, w := range []string{"Read", "3 files", "Edited", "internal/ui/scratch_gone_test.go", "+8", "−5", "Ran", "go vet and the ui tests", "failed · 1 test", "Readable", "Every step"} {
		if !rdHas(win.dock.texts, w) {
			t.Errorf("no %q in %q", w, win.dock.texts)
		}
	}
	if rdHas(win.dock.texts, "Bash") {
		t.Errorf("raw tool name shown: %q", win.dock.texts)
	}
}

func TestDockEveryStepShowsTodaysList(t *testing.T) {
	win, r, _, _, _ := rdDock(t, rdTranscript())
	win.actEvery = true
	twoFrames(t, win, r)
	if !rdHas(win.dock.texts, "Bash") || !rdHas(win.dock.texts, rdBashCmd) || rdHas(win.dock.texts, "3 files") {
		t.Fatalf("texts %q", win.dock.texts)
	}
}

func TestBoardPreviewShowsReadableSteps(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k, _ := readScene(t, win, int(a.ID), rdTranscript())
	twoFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	drawPreview(win)
	win.board.prev.follow = false
	drawPreview(win)
	for _, w := range []string{"3 files", "Edited", "+8", "−5", "go vet and the ui tests", "failed · 1 test", "Every step"} {
		if !boardHas(win, w) {
			t.Errorf("no %q in %q", w, win.board.texts)
		}
	}
	if boardHas(win, "Bash") {
		t.Errorf("raw tool name in the preview: %q", win.board.texts)
	}
}

func TestReadableBuiltOncePerChange(t *testing.T) {
	win, r, _, s, path := rdDock(t, rdTranscript())
	now := time.Now()
	for i := range 20 {
		frameAt(t, win, r, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	if s.read.builds != 1 {
		t.Fatalf("built %d times over unchanged frames", s.read.builds)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(rdLine("assistant", map[string]any{"type": "text", "text": "Next part."}))
	f.Close()
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second))
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second+10*time.Millisecond))
	if s.read.builds != 2 {
		t.Fatalf("built %d times after one new record, want 2", s.read.builds)
	}
}

func TestReadableHeaderCounts(t *testing.T) {
	win, _, _, _, _ := rdDock(t, rdTranscript())
	if !rdHas(win.dock.texts, "5 steps · 1 message") {
		t.Fatalf("texts %q", win.dock.texts)
	}
}

func TestHiddenPanelBuildsNothing(t *testing.T) {
	win, r, a, s, _ := rdDock(t, rdTranscript())
	builds, read := s.read.builds, s.trAt
	win.closeDock(a.ID)
	for i := range 5 {
		frameAt(t, win, r, time.Now().Add(time.Duration(i+1)*600*time.Millisecond))
	}
	if s.read.builds != builds || s.trAt != read {
		t.Fatalf("hidden panel: builds %d→%d, read %v→%v", builds, s.read.builds, read, s.trAt)
	}
}

func TestReadableLongDiffStaysCapped(t *testing.T) {
	var ob, nb strings.Builder
	for i := range 20000 {
		ob.WriteString("o" + strconv.Itoa(i%10) + "\n")
		nb.WriteString("n" + strconv.Itoa(i%10) + "\n")
	}
	tr := rdUse("big", "Edit", map[string]any{"file_path": "/w/kaze/a.go", "old_string": ob.String(), "new_string": nb.String()})
	win, r, _, s, _ := rdDock(t, tr)
	s.open = map[string]bool{"big": true}
	twoFrames(t, win, r)
	n := 0
	for _, x := range win.dock.texts {
		if x == "+ " || x == "− " {
			n++
		}
	}
	if n == 0 || n > 24 {
		t.Fatalf("%d diff lines drawn", n)
	}
	if !rdHas(win.dock.texts, "+20000") {
		t.Fatalf("counts lost: %q", win.dock.texts)
	}
}
