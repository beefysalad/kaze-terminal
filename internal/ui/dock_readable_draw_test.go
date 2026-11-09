package ui

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/codecolor"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/subagents"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
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
		for kind := codecolor.Plain; kind <= codecolor.Name; kind++ {
			check("code on the panel", ink.code[diffTonePlain][kind], bg)
			check("code on an added row", ink.code[diffToneAdd][kind], ink.addBG)
			check("code on a removed row", ink.code[diffToneDel][kind], ink.delBG)
		}
		check("number on the panel", ink.num[diffTonePlain], bg)
		check("number on an added row", ink.num[diffToneAdd], ink.addBG)
		check("number on a removed row", ink.num[diffToneDel], ink.delBG)
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
	win, r, _, s, _ := rdDock(t, rdTranscript())
	s.open = map[string]bool{"e1": false, "b1": false} // the open edit and failure would push rows out of the panel
	twoFrames(t, win, r)
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

// rdHit is where the readable step id was drawn in the dock.
func rdHit(t *testing.T, h dockHits, id string) image.Point {
	t.Helper()
	for _, x := range h.steps {
		if x.id == id {
			return centre(x.rect)
		}
	}
	t.Fatalf("step %q not drawn as clickable: %+v", id, h.steps)
	return image.Point{}
}

func rdCount(texts []string, want string) int {
	n := 0
	for _, s := range texts {
		if s == want {
			n++
		}
	}
	return n
}

func TestClickStepOpensDiff(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdTranscript())
	if rdCount(win.dock.texts, "+ ") == 0 || rdCount(win.dock.texts, "− ") != 5 || !rdHas(win.dock.texts, "n1") || !rdHas(win.dock.texts, "ctx1") {
		t.Fatalf("an applied edit is not open before any click: %q", win.dock.texts)
	}
	clickAt(t, win, r, rdHit(t, win.dock.hits, "e1"))
	if v, ok := s.open["e1"]; !ok || v || rdHas(win.dock.texts, "+ ") {
		t.Fatalf("diff still open: %q", win.dock.texts)
	}
	clickAt(t, win, r, rdHit(t, win.dock.hits, "e1"))
	if !s.open["e1"] || rdCount(win.dock.texts, "− ") != 5 {
		t.Fatalf("diff not open again: %q", win.dock.texts)
	}
}

func TestClickBashShowsCommandAndOutput(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdTranscript())
	s.open = map[string]bool{"e1": false} // a failed command is open by default; fold the edit above it into view
	twoFrames(t, win, r)
	if !rdHas(win.dock.texts, "$ "+rdBashCmd) || !rdHas(win.dock.texts, "FAIL    github.com/x/ui    4.8s") {
		t.Fatalf("texts %q", win.dock.texts)
	}
}

func TestClickFoldShowsFiles(t *testing.T) {
	win, r, _, _, _ := rdDock(t, rdTranscript())
	clickAt(t, win, r, rdHit(t, win.dock.hits, "r1"))
	for _, p := range []string{"internal/ui/agent.go", "internal/ui/window.go"} {
		if !rdHas(win.dock.texts, p) {
			t.Fatalf("no %q in %q", p, win.dock.texts)
		}
	}
	if rdCount(win.dock.texts, "internal/ui/scratch_gone_test.go") != 2 { // the fold list and the Edited row
		t.Fatalf("texts %q", win.dock.texts)
	}
}

func TestStepWithoutDetailIgnoresClick(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdUse("t1", "TodoWrite", map[string]any{"todos": []any{}}))
	if !rdHas(win.dock.texts, "Updated the to-do list") || len(win.dock.hits.steps) != 0 {
		t.Fatalf("texts %q steps %+v", win.dock.texts, win.dock.hits.steps)
	}
	h := win.dock.hits
	row := image.Pt(h.mode.Min.X+40, h.mode.Max.Y+gtxDp(win, 12))
	if win.readPress(s, &h, row) {
		t.Fatal("a step with no detail took the click")
	}
	clickAt(t, win, r, row)
	if len(s.open) != 0 {
		t.Fatalf("open %v", s.open)
	}
}

// gtxDp is n dp in px at the test frames scale.
func gtxDp(win *window, n int) int { return pageContext(10, 10).Dp(unit.Dp(n)) }

func TestModeToggleSwitchesBothViews(t *testing.T) {
	win, r, _, _, _ := rdDock(t, rdTranscript())
	clickAt(t, win, r, centre(win.dock.hits.modeEvery))
	if !win.actEvery || !rdHas(win.dock.texts, "Bash") {
		t.Fatalf("Every step not shown: %v %q", win.actEvery, win.dock.texts)
	}
	win.openBoard(0)
	win.boardSelect(subagents.Key{Session: "s", ID: "x"})
	drawPreview(win)
	if !boardHas(win, "Bash") || boardHas(win, "3 files") {
		t.Fatalf("board preview kept the readable view: %q", win.board.texts)
	}
	win.closeBoard()
	twoFrames(t, win, r)
	clickAt(t, win, r, centre(win.dock.hits.modeRead))
	if win.actEvery || !rdHas(win.dock.texts, "3 files") {
		t.Fatalf("Readable not back: %q", win.dock.texts)
	}
}

func TestBoardPreviewClickOpensStep(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k, _ := readScene(t, win, int(a.ID), rdTranscript())
	twoFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	win.board.prev.follow = false
	gtx := drawPreview(win)
	if !boardHas(win, "+ ") {
		t.Fatalf("an applied edit is not open in the preview: %q", win.board.texts)
	}
	win.boardPress(gtx, rdHit(t, win.board.prevHits, "e1"), 0)
	drawPreview(win)
	if v, ok := win.board.prev.open["e1"]; !ok || v || boardHas(win, "+ ") {
		t.Fatalf("preview step not folded: %q", win.board.texts)
	}
	win.boardPress(gtx, centre(win.board.prevHits.modeEvery), 0)
	drawPreview(win)
	if !win.actEvery || !boardHas(win, "Bash") {
		t.Fatalf("preview mode not switched: %q", win.board.texts)
	}
}

func TestOpenStepSurvivesNewRecords(t *testing.T) {
	win, r, _, s, path := rdDock(t, rdTranscript())
	clickAt(t, win, r, rdHit(t, win.dock.hits, "e1")) // folds the applied edit
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(rdUse("b2", "Bash", map[string]any{"command": "ls"}))
	f.Close()
	now := time.Now()
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second))
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second+10*time.Millisecond))
	if s.read.builds < 2 || s.open["e1"] || rdHas(win.dock.texts, "o1") || !rdHas(win.dock.texts, "go vet and the ui tests") {
		t.Fatalf("builds %d open %v texts %q", s.read.builds, s.open, win.dock.texts)
	}
}

func TestNoteLinksStillOpen(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	win.dock.heightDp = 440
	dir := t.TempDir()
	report(t, a, dir)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"), 0o644)
	k, _ := readScene(t, win, int(a.ID), rdLine("assistant", map[string]any{"type": "text", "text": "See notes.md now."})+rdUse("r1", "Read", map[string]any{"file_path": "/w/kaze/a.go"}))
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	if len(win.dock.hits.links) == 0 {
		t.Fatalf("no link drawn: %q", win.dock.texts)
	}
	p := centre(win.dock.hits.links[0].r)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Modifiers: key.ModShortcut, Position: at(p)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Modifiers: key.ModShortcut, Position: at(p)})
	twoFrames(t, win, r)
	if d := win.shownDoc(); d == nil || filepath.Base(d.path) != "notes.md" {
		t.Fatalf("doc not opened: %+v", d)
	}
}

// codexDock is a Codex helper with hook steps only, open in a tall panel.
func codexDock(t *testing.T, steps [][2]string) (*window, *dockSession) {
	t.Helper()
	win, _, r, a, _, _, _ := memberWindow(t)
	win.dock.heightDp = 440
	win.liveMgr.Store(win.mgr)
	send := func(m notify.Msg) { m.Tab, m.Session = int(a.ID), "s"; win.onNotify(m) }
	send(notify.Msg{Kind: notify.KindAgentStart, AgentID: "x", Agent: "codex", AgentType: "worker", Transcript: filepath.Join(t.TempDir(), "s.jsonl")})
	for _, s := range steps {
		send(notify.Msg{Kind: notify.KindTool, AgentID: "x", Agent: "codex", Tool: s[0], Summary: s[1]})
	}
	win.openDock(a.ID, subagents.Key{Session: "s", ID: "x"})
	s := win.dockOf(a.ID)
	s.follow = false
	twoFrames(t, win, r)
	return win, s
}

func TestCodexHelperReadableFallback(t *testing.T) {
	win, s := codexDock(t, [][2]string{{"apply_patch", "internal/ui/x.go"}, {"Bash", "go test ./..."}, {"Read", "a.go"}, {"Read", "b.go"}})
	if s.tr != nil {
		t.Fatal("a Codex helper read a transcript")
	}
	for _, w := range []string{"Edited", "internal/ui/x.go", "Ran", "go test ./...", "Read", "2 files", "4 steps · 0 messages"} {
		if !rdHas(win.dock.texts, w) {
			t.Errorf("no %q in %q", w, win.dock.texts)
		}
	}
	if rdHas(win.dock.texts, "passed") || rdHas(win.dock.texts, "done") || rdHas(win.dock.texts, "+0") {
		t.Errorf("hook steps claim results: %q", win.dock.texts)
	}
	// Only the fold has a detail to open.
	if len(win.dock.hits.steps) != 1 {
		t.Errorf("clickable steps %+v", win.dock.hits.steps)
	}
}

func TestNoTranscriptStillShowsNotes(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	win.dock.heightDp = 440
	k := dockAgent(t, win, int(a.ID), "") // no main transcript known
	win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindTool, Session: "s", AgentID: "x", AgentType: "Explore", Tool: "Grep", Summary: "auth"})
	win.openDock(a.ID, k)
	twoFrames(t, win, r)
	if !rdHas(win.dock.texts, "Searched the code") || win.dock.texts[len(win.dock.texts)-1] != "working" {
		t.Fatalf("texts %q", win.dock.texts)
	}
}

// rdPatchEdit is an Edit of path whose result carries a structuredPatch starting
// at line start of both files, with the given patch lines.
func rdPatchEdit(id, path string, start int, lines ...string) string {
	b, _ := json.Marshal(map[string]any{"type": "user",
		"toolUseResult": map[string]any{"structuredPatch": []map[string]any{{"oldStart": start, "oldLines": len(lines), "newStart": start, "newLines": len(lines), "lines": lines}}},
		"message":       map[string]any{"content": []map[string]any{{"type": "tool_result", "tool_use_id": id, "is_error": false, "content": "ok"}}}})
	return rdUse(id, "Edit", map[string]any{"file_path": path, "old_string": "x", "new_string": "y"}) + string(b) + "\n"
}

// rdDiffEdit: context at 70, one removal at 71, two additions at 71 and 72.
func rdDiffEdit(id string) string {
	return rdPatchEdit(id, "/w/kaze/a.go", 70, " package main", "-func old() {}", "+func fresh() {}", "+var keep = 1")
}

// diffShot renders one frame at size and returns its pixels. Skips where the
// machine has no GPU-backed Gio context.
func diffShot(t *testing.T, win *window, r *input.Router, w, h int) *image.RGBA {
	t.Helper()
	gpu, err := headless.NewWindow(w, h)
	if err != nil {
		t.Skipf("no headless GPU here: %v", err)
	}
	defer gpu.Release()
	gtx := pageContext(w, h)
	gtx.Source = r.Source()
	gtx.Now = time.Unix(1000, 0)
	if err := win.frame(gtx); err != nil {
		t.Fatal(err)
	}
	r.Frame(gtx.Ops)
	if err := gpu.Frame(gtx.Ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := gpu.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

// nearColour is whether p is c, give or take rounding.
func nearColour(p color.RGBA, c color.NRGBA) bool {
	d := func(a, b uint8) int { return max(int(a), int(b)) - min(int(a), int(b)) }
	return d(p.R, c.R) <= 3 && d(p.G, c.G) <= 3 && d(p.B, c.B) <= 3
}

// colourDist is how far apart p and c are, per channel summed.
func colourDist(p color.RGBA, c color.NRGBA) int {
	d := func(a, b uint8) int { return max(int(a), int(b)) - min(int(a), int(b)) }
	return d(p.R, c.R) + d(p.G, c.G) + d(p.B, c.B)
}

// diffRowAt is the pane's right edge and the middle of row i below the step
// drawn at hit.
func diffRowAt(win *window, hit image.Point, i int) (x, y int) {
	var r image.Rectangle
	for _, st := range win.dock.hits.steps {
		if hit.In(st.rect) {
			r = st.rect
		}
	}
	return r.Max.X - 2, r.Max.Y + gtxDp(win, 2) + i*gtxDp(win, readLineDp) + gtxDp(win, readLineDp)/2
}

func TestDiffDrawsNumbersAndNoBox(t *testing.T) {
	win, r, _, _, _ := rdDock(t, rdDiffEdit("e1"))
	for _, w := range []string{"70", "71", "72", "func fresh() {}", "package main", "+ ", "− "} {
		if !rdHas(win.dock.texts, w) {
			t.Fatalf("no %q in %q", w, win.dock.texts)
		}
	}
	x, y := diffRowAt(win, rdHit(t, win.dock.hits, "e1"), 0)
	th := win.ui.th
	img := diffShot(t, win, r, 1000, 600)
	if p := img.RGBAAt(x, y); !nearColour(p, win.pal.BG) || nearColour(p, th.Chrome) {
		t.Fatalf("context row sits on %v, want the pane's %v and no box %v", p, win.pal.BG, th.Chrome)
	}
	if p := img.RGBAAt(x, y+gtxDp(win, readLineDp)); !nearColour(p, th.DiffDelBG) {
		t.Fatalf("removed row is %v, want %v across the whole row", p, th.DiffDelBG)
	}
	if p := img.RGBAAt(x, y+2*gtxDp(win, readLineDp)); !nearColour(p, th.DiffAddBG) {
		t.Fatalf("added row is %v, want %v across the whole row", p, th.DiffAddBG)
	}
}

// A diff without numbers (its result has not arrived) shows none.
func TestDiffWithoutNumbersShowsNone(t *testing.T) {
	win, _, _, _, _ := rdDock(t, rdUse("e1", "Edit", map[string]any{"file_path": "/w/kaze/a.go", "old_string": "aa", "new_string": "bb"}))
	if !rdHas(win.dock.texts, "bb") {
		t.Fatalf("no diff: %q", win.dock.texts)
	}
	for _, s := range win.dock.texts {
		if s == "0" || s == "1" {
			t.Fatalf("a made-up number %q in %q", s, win.dock.texts)
		}
	}
}

func TestDiffGutterFitsTheWidestNumber(t *testing.T) {
	lines := []subagents.DiffLine{{Kind: '+', New: 7}, {Kind: ' ', Text: "no number"}, {Kind: '-', Old: 1204}}
	if got := diffDigits(lines); got != 4 {
		t.Fatalf("digits %d, want 4 for the whole diff", got)
	}
	if got := diffDigits([]subagents.DiffLine{{Kind: '+'}}); got != 0 {
		t.Fatalf("digits %d for a diff with no numbers", got)
	}
}

func TestDiffRowsColoured(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdDiffEdit("e1"))
	x, y := diffRowAt(win, rdHit(t, win.dock.hits, "e1"), 2)
	ink := s.inkFor(win.ui, win.pal.BG)
	want := ink.code[diffToneAdd][codecolor.Keyword]
	if want == (color.NRGBA{}) || want == ink.addFG {
		t.Fatalf("keyword ink %v against plain %v", want, ink.addFG)
	}
	img := diffShot(t, win, r, 1000, 600)
	found := false
	for px := max(x-800, 0); px < x && !found; px++ {
		for py := y - gtxDp(win, readLineDp)/2; py < y+gtxDp(win, readLineDp)/2; py++ {
			// Thin glyphs are antialiased: a keyword pixel leans to the keyword
			// colour, away from the row's own background and plain text.
			if p := img.RGBAAt(px, py); !nearColour(p, ink.addBG) && colourDist(p, want) < colourDist(p, ink.addFG) {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("no pixel of the keyword colour %v in the added row", want)
	}
}

func TestDiffMoreRow(t *testing.T) {
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = "+line" + strconv.Itoa(i)
	}
	win, r, _, s, _ := rdDock(t, rdPatchEdit("e1", "/w/kaze/a.txt", 1, lines...))
	s.scroll = win.dock.hits.scrollMax
	twoFrames(t, win, r)
	if !rdHas(win.dock.texts, "20 more lines") {
		t.Fatalf("no more row in %q", win.dock.texts)
	}
	want := gtxDp(win, readRowDp) + 41*gtxDp(win, readLineDp) + gtxDp(win, 10)
	found := false
	for _, h := range s.read.heights {
		found = found || h == want
	}
	if !found {
		t.Fatalf("heights %v, none holds 40 rows and the more row (%d)", s.read.heights, want)
	}
}

func TestDiffRecordedOnce(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdDiffEdit("e1"))
	first := s.read.diffRecords
	if first != 1 {
		t.Fatalf("recorded %d times, want 1", first)
	}
	now := time.Now()
	for i := range 50 {
		frameAt(t, win, r, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	if s.read.diffRecords != first {
		t.Fatalf("recorded %d times over still frames", s.read.diffRecords-first)
	}
	if !rdHas(win.dock.texts, "func fresh() {}") {
		t.Fatalf("texts lost on replay: %q", win.dock.texts)
	}
}

func TestDiffRedrawnOnThemeAndWidth(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdDiffEdit("e1"))
	n := s.read.diffRecords
	again := func(what string) {
		t.Helper()
		if s.read.diffRecords <= n {
			t.Fatalf("%s: not recorded again", what)
		}
		n = s.read.diffRecords
	}
	var other theme.Theme
	for _, th := range theme.All() {
		if th.UI.DiffAddBG != win.ui.th.DiffAddBG {
			other = th
			break
		}
	}
	win.applyTheme(other)
	twoFrames(t, win, r)
	again("theme")
	x, y := diffRowAt(win, rdHit(t, win.dock.hits, "e1"), 2)
	if p := diffShot(t, win, r, 1000, 600).RGBAAt(x, y); !nearColour(p, other.UI.DiffAddBG) {
		t.Fatalf("added row %v, want the new theme's %v", p, other.UI.DiffAddBG)
	}
	sizedFrame(t, win, r, 1200, 600)
	sizedFrame(t, win, r, 1200, 600)
	again("width")
	gtx := pageContext(1200, 600)
	gtx.Source, gtx.Now = r.Source(), time.Unix(1000, 0)
	gtx.Metric = unit.Metric{PxPerDp: 2, PxPerSp: 2}
	if err := win.frame(gtx); err != nil {
		t.Fatal(err)
	}
	again("scale")
}

func TestDiffRecordingDroppedWithItsStep(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdDiffEdit("e1"))
	if len(s.read.diffs) != 1 {
		t.Fatalf("recordings %d", len(s.read.diffs))
	}
	s.tr.Steps = nil // the step is gone from the list
	s.tr.Version++
	twoFrames(t, win, r)
	if len(s.read.diffs) != 0 {
		t.Fatalf("recording kept for a step no longer listed: %d", len(s.read.diffs))
	}
}

func TestFollowStaysAtBottomWithOpenDiffs(t *testing.T) {
	win, r, _, s, path := rdDock(t, rdDiffEdit("e1"))
	s.follow = true
	twoFrames(t, win, r)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(rdPatchEdit("e2", "/w/kaze/b.go", 300, "+func tail() {}"))
	f.Close()
	now := time.Now()
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second))
	frameAt(t, win, r, now.Add(agentTranscriptPoll+time.Second+10*time.Millisecond))
	if !s.follow || s.scroll != win.dock.hits.scrollMax {
		t.Fatalf("follow %v scroll %d of %d", s.follow, s.scroll, win.dock.hits.scrollMax)
	}
	if !rdHas(win.dock.texts, "func tail() {}") || win.dock.texts[len(win.dock.texts)-1] != "working" {
		t.Fatalf("last row not shown: %q", win.dock.texts)
	}
}

func TestFoldingMiddleRowStopsFollow(t *testing.T) {
	win, r, _, s, _ := rdDock(t, rdDiffEdit("e1")+rdDiffEdit("e2"))
	s.follow = true
	twoFrames(t, win, r)
	clickAt(t, win, r, rdHit(t, win.dock.hits, "e1"))
	if s.follow {
		t.Fatal("folding a middle row kept following")
	}
	if s.open["e1"] {
		t.Fatalf("e1 not folded: %v", s.open)
	}
}

func TestBoardPreviewShowsDiff(t *testing.T) {
	win, _, r, a, _, _, _ := memberWindow(t)
	k, _ := readScene(t, win, int(a.ID), rdDiffEdit("e1"))
	twoFrames(t, win, r)
	win.openBoard(0)
	win.boardSelect(k)
	drawPreview(win)
	win.board.prev.follow = false
	drawPreview(win)
	for _, w := range []string{"72", "func fresh() {}", "+ ", "− "} {
		if !boardHas(win, w) {
			t.Errorf("no %q in %q", w, win.board.texts)
		}
	}
}

// A failed command draws in place: no box behind its output either.
func TestCommandOutputHasNoBox(t *testing.T) {
	win, r, _, _, _ := rdDock(t, rdUse("b1", "Bash", map[string]any{"command": "go test"})+rdResult("b1", true, "boom"))
	x, y := diffRowAt(win, rdHit(t, win.dock.hits, "b1"), 0)
	th := win.ui.th
	if p := diffShot(t, win, r, 1000, 600).RGBAAt(x, y); !nearColour(p, win.pal.BG) || nearColour(p, th.Chrome) {
		t.Fatalf("output sits on %v, want the pane's %v", p, win.pal.BG)
	}
}
