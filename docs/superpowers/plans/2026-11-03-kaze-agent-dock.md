# Docked Agent Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the floating subagent cards with one docked panel under the terminal (Activity and Report tabs, markdown rendered), per issues #106 and #103.

**Architecture:** A new `dock` state on the window holds one remembered agent per session plus a global height. `frame` splits the terminal area into terminal and panel before sizing the grid, and the docked session gets its own PTY size through `tabs.Manager.ResizeTab`. The doc reader's layout is split into a reusable markdown layout/draw pair at a scale, used by the reader, the panel's notes, prompt and report. The floating `paneAgent` kind and its code are removed.

**Tech Stack:** Go 1.27, Gio, `internal/mdoc` (goldmark, already a dependency).

**Spec:** `docs/superpowers/specs/2026-11-03-kaze-agent-dock-design.md`

## Global Constraints

- Strict TDD: each test is seen failing before the code exists. RED tests are bounded (no unbounded sizes or loops).
- No global keyboard or mouse input in tests. Drive input through `input.Router` queues as existing UI tests do.
- Idle cost: a closed panel, or an open panel on a finished agent with no new data, asks for no frames and parses nothing.
- Transcript reads only while the panel is visible: once a second, ≤ 256 KiB per read, 128 entries / 128 KiB kept (existing `subagents.Transcript` caps).
- Panel min height 120 dp; terminal keeps at least 8 rows; default height 40% of the terminal area; saved as `agent_panel_height` (dp, int) in `config.toml`, written on drag release only.
- `Esc`, `↑`, `↓` go to the terminal unless the panel has the keyboard (the user clicked into the panel body).
- Copy report copies the original markdown.
- All five themes: panel text ≥ 4.5:1 contrast.
- Commits: `type(scope): Verb ...`, no AI attribution lines. Work on branch `feat/agent-dock`; never push to `main`.
- Smart App Control: if a test binary is blocked, rerun with `go test -ldflags "-X main.r=<random>" ./internal/<pkg>/`.

## Review Focus

1. A window too short for both: the terminal's 8-row minimum wins, the panel shrinks or hides, and the saved height is untouched. (Task 4 test `TestDockSplitSmallWindow`.)
2. The open agent disappears from the tracker (next prompt, session cleared): the panel closes without a crash and the session forgets it. (Task 9 test `TestDockForgetsGoneAgent`.)
3. Closing a session with a remembered panel: its state is pruned and its PTY size is not left shrunk for a reused id. (Task 5 test `TestDockPrunedWithSession`.)
4. A long, growing transcript: only new or changed messages are parsed; an unchanged frame parses nothing. (Task 7 test `TestDockNotesParseOnce`.)
5. Esc typed in the terminal while the panel is open reaches the PTY and leaves the panel open. (Task 8 test `TestDockEscStaysWithTerminal`.)

---

### Task 1: Keep the prompt's leading whitespace

**Files:**
- Modify: `cmd/kaze/notify.go:261`
- Test: `cmd/kaze/notify_test.go`

**Interfaces:** none new.

- [ ] **Step 1: Write the failing test** `TestAgentToolKeepsPromptIndent`: feed the PreToolUse hook input for an `Agent` call with `prompt: "    indented code\nnext"` through the same function the existing Agent-call tests use, assert `m.TaskPrompt == "    indented code\nnext"`.
- [ ] **Step 2: Run** `go test ./cmd/kaze/ -run TestAgentToolKeepsPromptIndent` → FAIL (indent trimmed).
- [ ] **Step 3: Change** to `notify.FitReport(prompt, notify.MaxPrompt)` (no TrimSpace). The UI already applies `trimBlock`.
- [ ] **Step 4: Run** `go test ./cmd/kaze/` → PASS.
- [ ] **Step 5: Commit** `fix(notify): Keep a subagent prompt's leading indent`

### Task 2: Timestamps on transcript messages

**Files:**
- Modify: `internal/subagents/transcript.go` (`TranscriptMessage`, `record`)
- Test: `internal/subagents/transcript_test.go`

**Interfaces:**
- Produces: `TranscriptMessage.At time.Time` (zero when the record has no parsable `timestamp`).

- [ ] **Step 1: Write the failing test** `TestTranscriptMessageTime`: a JSONL line `{"type":"assistant","timestamp":"2026-11-03T05:02:03.123Z","message":{"content":[{"type":"text","text":"hi"}]}}` gives `Messages[0].At.Equal(time.Date(2026,11,3,5,2,3,123e6,time.UTC))`; a line without `timestamp` gives zero `At`. When text and tools merge into one message, `At` is the first record's time.
- [ ] **Step 2: Run** `go test ./internal/subagents/ -run TestTranscriptMessageTime` → FAIL (no field).
- [ ] **Step 3: Implement**: add `Timestamp string \`json:"timestamp"\`` to the record struct, parse with `time.Parse(time.RFC3339Nano, …)`, set on the appended message.
- [ ] **Step 4: Run** `go test ./internal/subagents/` → PASS.
- [ ] **Step 5: Commit** `feat(subagents): Record when each transcript message was written`

### Task 3: Reusable markdown layout and drawing

**Files:**
- Modify: `internal/ui/doc_layout.go`, `internal/ui/doc_draw.go`
- Test: `internal/ui/doc_layout_test.go` (existing reader tests must keep passing unchanged)

**Interfaces:**
- Produces:
  - `type mdStyle struct{ scale float32 }`; `var readerStyle = mdStyle{1}`, `var panelStyle = mdStyle{12.0 / 14}` (body 12 sp instead of 14). Every sp and dp constant in the layout is multiplied by `scale`.
  - `func (win *window) layoutMarkdown(gtx layout.Context, d *mdoc.Doc, width int, st mdStyle) docLayout` — the body of today's `layoutDoc` without caching.
  - `layoutDoc` keeps its signature and cache and calls `layoutMarkdown(..., readerStyle)`.
  - `func (win *window) drawMarkdown(gtx layout.Context, l *docLayout, view image.Rectangle, origin image.Point, scroll int, links *[]docLinkHit, texts *[]string)` — the boxes-and-lines loop from `drawDoc`; `drawDoc` calls it with `&win.docs.hits.links, &win.docs.texts`.
  - Tables in `layoutMarkdown` never exceed `width` (they already shrink columns; assert it).

- [ ] **Step 1: Write the failing tests** `TestLayoutMarkdownPanelScaleIsSmaller` (same doc, `panelStyle` height < `readerStyle` height, and a body line's run `size == 12`) and `TestLayoutMarkdownTableFitsWidth` (a 6-column table of long words at width 300: every run's `x+w <= 300`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestLayoutMarkdown'` → FAIL (undefined).
- [ ] **Step 3: Implement** the split above. No behaviour change for the reader.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Doc|Markdown'` → PASS (including all existing reader tests).
- [ ] **Step 5: Commit** `refactor(ui): Share the reader's markdown layout at any scale`

### Task 4: Dock state, height split and config key

**Files:**
- Create: `internal/ui/dock.go`, `internal/ui/dock_test.go`
- Modify: `internal/config/config.go` (+ its test), `internal/ui/window.go` (field `dock dockState` on `window`)

**Interfaces:**
- Produces:
  - config: `AgentPanelHeight int \`toml:"agent_panel_height"\`` (dp; 0 = unset; negative or non-int warns and is ignored).
  - `type dockTab uint8` with `dockActivity`, `dockReport`.
  - `type dockSession struct { agent subagents.Key; tab dockTab; picked bool; scroll int; follow bool; promptOpen bool; tr *subagents.Transcript; trPath string; trAt, trStarted time.Time; cache dockCache }` (cache defined in Task 7; start with an empty struct type).
  - `type dockState struct { by map[tabs.TabID]*dockSession; heightDp int; keys bool; tag, handle, head, body int; drag dockDrag; hits dockHits; lastRect image.Rectangle }`
  - `func dockSplit(term image.Rectangle, heightDp int, pxPerDp float32, cellH, pad int) (top, panel image.Rectangle)` — pure. `heightDp<=0` → 40% of `term`. Clamp: panel ≥ 120 dp; `top` keeps ≥ `8*cellH+pad` px. If both cannot fit, `top` keeps its minimum and `panel` gets the rest; panel shorter than 40 dp → empty (no panel drawn).
  - `func (win *window) dockOwner() tabs.TabID` — the shown tab's ID if it is not a scratch shell and no Settings/board/doc covers it; else 0. Members count (unlike `paneOwner`).
  - `func (win *window) dockOf(id tabs.TabID) *dockSession` — nil when none.
  - `func (win *window) openDock(session tabs.TabID, k subagents.Key)`, `func (win *window) closeDock(session tabs.TabID)`, `func (win *window) toggleDock(session tabs.TabID, k subagents.Key)` (same key open → close; else open/switch; switching agent resets tab to Activity, `picked=false`, `follow=true`, scroll 0, transcript nil).
  - `win.dock.heightDp` loads from `Config.AgentPanelHeight` at window start.

- [ ] **Step 1: Write the failing tests**: `TestDockSplitDefault` (term 1000x600 at 1x, height 0 → panel 240 tall at the bottom, top+panel == term), `TestDockSplitClamps` (height 50 → 120; height 900 → top keeps `8*cellH+pad`), `TestDockSplitSmallWindow` (term 150 tall, cellH 16 → top gets its minimum or all, panel ≤ remaining, empty if < 40), `TestDockToggle` (open k, toggle k closes, open k then j switches and resets tab), `TestDockPerSession` (open X on A, open Y on B; `dockOf(A).agent == X`), config `TestAgentPanelHeight` (load 300; `-5` warns, stays 0).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDock' ; go test ./internal/config/ -run TestAgentPanelHeight` → FAIL.
- [ ] **Step 3: Implement** the interfaces above.
- [ ] **Step 4: Run** the same commands → PASS.
- [ ] **Step 5: Commit** `feat(ui): Add the agent dock model and height split`

### Task 5: Fit the dock into the frame and size the PTY

**Files:**
- Modify: `internal/ui/window.go` (`frame`), `internal/ui/panes_draw.go` (`paneStep`), `internal/ui/dock.go`
- Test: `internal/ui/dock_test.go`

**Interfaces:**
- Consumes: `dockSplit`, `dockOwner`, `dockOf` (Task 4).
- Produces: `func (win *window) dockRect() image.Rectangle` (last frame's panel rect, empty when none); `func (win *window) pruneDock(list []*tabs.Tab)`.

Behaviour: in `frame`, after `termRect` is known and before `grid`, if `dockOwner()` has a `dockSession`, split `termRect` into `termRect` (top) and `win.dock.lastRect` (panel). The grid and `ResizeAll` use the full size for every tab; the docked session then gets `win.mgr.ResizeTab(id, cols, rows)` for the smaller grid. When a session's dock closes, or its session is not shown with a dock, call `win.mgr.Dock(id)` only if it was sized by the dock (track `sized map[tabs.TabID]bool` on `dockState`). `paneStep` must call `win.mgr.Dock(active.ID)` only when `win.fl.owner(active.ID) != 0` (it floated); otherwise it would undo the dock's size every frame. During a handle drag, the split follows the pointer but `ResizeTab` runs only on release (`win.dock.drag.on` → keep the last sized rows). Floating member panes use the shrunk `termRect`, so `fitPane` keeps them above the panel.

- [ ] **Step 1: Write the failing tests**: `TestDockShrinksOnlyItsSession` (paneWindow; open dock on `a`; two frames; `a`'s recPTY last size has fewer rows than `b`'s), `TestDockNoResizeChurn` (with dock open, 5 more frames add no resize calls on `a`'s PTY), `TestDockCloseRestoresSize` (close → `a` back to shared rows), `TestDockPrunedWithSession` (close tab `a` → `win.dock.by` has no `a`, `sized` has no `a`), `TestDockMemberPaneStaysAbove` (float member over `a` with dock open: the pane's `hits.outer.Max.Y <= dockRect().Min.Y`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDock'` → FAIL.
- [ ] **Step 3: Implement** as described; call `pruneDock(list)` next to `pruneDocs(list)`.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Dock the agent panel under the terminal`

### Task 6: Activity timeline model

**Files:**
- Create: `internal/ui/dock_timeline.go`, `internal/ui/dock_timeline_test.go`

**Interfaces:**
- Consumes: `subagents.Agent`, `subagents.Transcript` (with `TranscriptMessage.At`, Task 2).
- Produces:
  - `type dockItemKind uint8`: `itemDropped`, `itemStep`, `itemNote`, `itemWaiting`, `itemLive`, `itemFinished`.
  - `type dockItem struct { kind dockItemKind; at time.Time; tool, target, text string; msg int }` (`msg` indexes `Transcript.Messages` for notes, -1 otherwise).
  - `func dockTimeline(a *subagents.Agent, tr *subagents.Transcript) []dockItem`.

Rules: `itemDropped` first when `a.Dropped > 0` ("… N earlier steps") or `tr.Dropped` ("… earlier messages hidden"). Steps from `a.Steps` (tool, summary as target). Notes from transcript messages with non-empty `Text`; ordered by `At` among steps (stable; a note with zero `At` goes right after the previous item in file order). Tool-only transcript messages are not shown (the steps cover them). Waiting: `itemWaiting` with text `Wants to run <summary>. Approve it in the session above.` when the last step is `Bash`, else `Waiting for your approval in the session above.`. Running: `itemLive` last. Done: `itemFinished` with text `Finished in <minutes(a.Ended-a.Started)>` (reuse `minutes`). `tr == nil` or `!tr.Available` → steps only.

- [ ] **Step 1: Write the failing tests** `TestDockTimelineInterleaves` (steps at t0, t0+2s; note at t0+1s → step, note, step, live), `TestDockTimelineUntimedNote`, `TestDockTimelineWaitingBash` (exact text `Wants to run go test ./internal/mdoc/. Approve it in the session above.`), `TestDockTimelineDone` (last item `Finished in <1m`, no live row), `TestDockTimelineDropped`, `TestDockTimelineCodexStepsOnly` (tr nil).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestDockTimeline` → FAIL.
- [ ] **Step 3: Implement** `dockTimeline`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(ui): Build the agent activity timeline`

### Task 7: Draw the panel

**Files:**
- Create: `internal/ui/dock_draw.go`, `internal/ui/dock_draw_test.go`
- Modify: `internal/ui/window.go` (call `drawDock` after `drawPanes` in the terminal branch), `internal/ui/panes_transcript.go` (move the poll to the dock)

**Interfaces:**
- Consumes: Tasks 3–6.
- Produces:
  - `type dockCache struct { notes map[int]*mdCache; prompt, report mdCache }`, `type mdCache struct { src string; width int; metric unit.Metric; shaper *text.Shaper; doc *mdoc.Doc; lay docLayout; parses int }`.
  - `func (win *window) mdLayout(gtx layout.Context, c *mdCache, src string, width int) *docLayout` — parses and lays out only when `src`, `width`, metric or shaper changed; increments `c.parses` on parse.
  - `func (win *window) drawDock(gtx layout.Context, s *dockSession, a *subagents.Agent, r image.Rectangle)`.
  - `func (win *window) dockTick(s *dockSession, a *subagents.Agent, now time.Time) (time.Time, bool)` — the existing `agentTranscriptTick` logic on `dockSession` fields; called only from `drawDock`.
  - `func (win *window) dockReport(a *subagents.Agent) string` — replaces `agentReport`: the docked transcript's `Report(a.Report)` when available, else `new(subagents.Transcript).Report(a.Report)`.
  - `dockHits { handle, header, actTab, repTab, copy, reader, close, prompt, finishLink, body image.Rectangle; links []docLinkHit; scrollMax int }`.

Look (follow `docs/design/agent-pane-dock-mockup.html`): handle 6 dp with `CursorRowResize`; header 32 dp on `th.Chrome` with state mark (`stateDot`, running ring), type in magenta (`memberColor("magenta")`), task in `ActiveFG`, elapsed muted (`waiting for you` in `th.Needs` when Waiting), a two-button segmented switch, then Copy report, Open in reader (Done only) and ×. Prompt row: `▸ Prompt: <first line>` muted 11 sp; expanded `▾` shows the prompt via `mdLayout(&cache.prompt, trimBlock(a.Prompt), …)` at `panelStyle`. Activity rows 20 dp: clock `15:04` muted mono, tool in accent mono, target `ActiveFG` mono, clipped with ellipsis (`fitSide`). Notes: 2 dp accent bar, markdown at `panelStyle`, cached per message index in `cache.notes`. Waiting box: `tint(th.Needs,0x22)` rounded, text `th.Needs`. Finished row: `✓ ` in `th.Done`, "read the report" in accent (its rect is `finishLink`). Report tab: "No report yet" muted while not Done, else `mdLayout(&cache.report, win.dockReport(a), …)` drawn with `drawMarkdown`. Body scroll follows the bottom on Activity while `follow`; Report starts at top. Poll: `gtx.Execute(op.InvalidateCmd{At: at})` from `dockTick` only while running and visible.

- [ ] **Step 1: Write the failing tests**: `TestDockDrawsReportMarkdown` (Done agent with report `**No binding** and `+"`x.go:3`"+`; open Report; frame; the drawn texts contain `No binding` and do not contain `**`), `TestDockNotesParseOnce` (transcript with 3 notes; 3 frames; total `parses` == 3; append one note + tick; frame; total == 4), `TestDockIdleNoWake` (Done agent, open dock, frame: no `InvalidateCmd` queued by the dock; use the existing frame-wake test helper pattern from `panes_transcript_test.go`), `TestDockReportCopyIsMarkdown` (copy gives the raw `**…**` text), `TestDockContrast` (for each of the five themes, the muted, accent, needs and done text colours on `th.Chrome` and `pal.BG` are ≥ 4.5:1 via the existing contrast helper used by the sidebar tests).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDock(Draws|Notes|Idle|ReportCopy|Contrast)'` → FAIL.
- [ ] **Step 3: Implement** the drawing, the caches and the moved poll.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw the agent panel with Activity and Report`

### Task 8: Panel input: buttons, tabs, drag, scroll, links, keys

**Files:**
- Create: `internal/ui/dock_input.go`, `internal/ui/dock_input_test.go`
- Modify: `internal/ui/window.go` (`dockEvents` before `paneEvents` in the terminal branch; `focusTarget` returns `&win.dock.tag` when `win.dock.keys` and a panel is shown), `internal/ui/settings_appearance.go` not needed (`saveSetting` is reused)

**Interfaces:**
- Consumes: Task 7 hits.
- Produces: `func (win *window) dockEvents(gtx layout.Context)`; `func (win *window) dockStep(d int)` (next/prev agent of the same session in `win.subs.Tree` order, wrapping off: stops at ends).

Behaviour: Activity/Report buttons set `tab` and `picked=true`. Finishing (state turns Done while `!picked`) switches to Report once. `finishLink` → Report + `picked=true`. Copy → `copyReport` using `dockReport`. Open in reader → Task 10's `openReportDoc`. × → `closeDock`. Prompt row toggles `promptOpen`. Handle: press starts `drag` (start height, press Y); drag sets `heightDp` from the pointer (clamped through `dockSplit` limits) and invalidates; release saves `win.saveSetting("agent_panel_height", heightDp)` once and resizes the PTY. Scroll in the body moves `scroll` within `scrollMax` and sets `follow = scroll >= scrollMax`. Modifier-click (`key.ModShortcut`) on a link: web → `win.open`, else `win.openDocRef(sessionTab, false, ref)`. A press in the body sets `win.dock.keys = true` and `wantFocus`. While `keys`: `Esc` closes the panel and returns keys to the terminal; `↑`/`↓` call `dockStep(-1/+1)`; other keys are ignored. Any press in the terminal (main view) sets `keys = false`.

- [ ] **Step 1: Write the failing tests**: `TestDockTabsAndAutoReport` (running → Activity; stop → Report; if user pressed Activity before stop, stays Activity), `TestDockHandleDragSavesOnce` (drag over 5 moves: `heightDp` changes, config file written once with `agent_panel_height`), `TestDockEscStaysWithTerminal` (dock open, keys on terminal, queue `Esc`: the recPTY got `\x1b`, dock still open), `TestDockEscClosesWhenFocused` (click body, queue `Esc`: dock closed, PTY got nothing), `TestDockArrowsStepAgents` (3 agents, click body, `↓` → second agent), `TestDockLinkClickOpensDoc` (Ctrl/Cmd-click a `notes.md` link in the report → a doc opened under the session).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDock(Tabs|Handle|Esc|Arrows|Link)'` → FAIL.
- [ ] **Step 3: Implement** `dockEvents`, `dockStep` and the focus rule.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Make the agent panel interactive`

### Task 9: Entry points, highlight, and removing the floating agent pane

**Files:**
- Modify: `internal/ui/sidebar_window.go:598`, `internal/ui/sidebar_agents.go` (highlight), `internal/ui/board.go:401`, `internal/ui/alerts_ui.go` (`toastEvent`), `internal/ui/panes.go`, `panes_draw.go`, `panes_input.go`, `panes_agent.go`, `panes_agent_links.go`, `panes_transcript.go`
- Delete or migrate tests: `panes_agent_test.go`, `panes_agent_links_test.go`, `panes_transcript_test.go`, `panes_autoscroll_test.go` (keep what still applies, e.g. `activityText`, `trimBlock`, `wrapSans` tests; rewrite transcript tests against the dock)
- Test: `internal/ui/dock_entry_test.go`

**Interfaces:**
- Consumes: `toggleDock`, `openDock`, `closeDock`.
- Produces: `func (win *window) showAgent(tab tabs.TabID, k subagents.Key)` — the single entry: if `tab` is a workspace member, `win.selectTab(tab)` (it becomes the main view, like Expand: `fl.remove` + `mgr.Dock` first if it floated); else if it is not the shown session, `win.selectTab(tab)`; then `toggleDock(tab, k)` (from the sidebar) / `openDock` (board, toast). `func (win *window) waitingAgent(tab tabs.TabID) (subagents.Key, bool)` — the session's Waiting subagent with the earliest `Started`.

Behaviour: sidebar agent line click → `showAgent` with toggle; the line whose key is `dockOf(shown).agent` gets the selected-row background (`th.Selection` or the colour selected session rows use). Board card → `showAgent` (open, not toggle). Toast Go → `selectTab(h.id)`, then `if k, ok := waitingAgent(h.id); ok { openDock(h.id, k) }`. `dropGoneAgents` becomes: for every `dockSession` whose agent `win.subs.Get` returns nil, delete it (and un-size). Remove `paneAgent`, `agentID(s)`, `agentIDs`, `lastAgent`, the agent branches in `open`/`prune`/`paneViews`/`drawPanes`/`paneEvents`/`paneFrame`/`focusPane`/`closePane`/`syncVisible`, `drawAgentPane`, `agentWrapped`, `wrapCached`, `paneWrapKey`, `wrapCache`, the pane's live-only fields, `copyBtn` in `paneLook`, and `agentPaneLink`/`agentPanePointer`. Keep `wrapSans`/`wrapBlock` only if something still uses them; otherwise delete with their tests.

- [ ] **Step 1: Write the failing tests**: `TestSidebarAgentTogglesDock` (click line → dock open; click again → closed), `TestSidebarAgentHighlight` (open line's hit band is drawn with the selection colour; check via the sidebar draw-op recorder used by existing sidebar tests, or a `side.hits.agents[i].selected` flag set during drawing), `TestSidebarAgentOtherSession` (dock X on A, click agent under B → B shown with Y; back to A → X), `TestSidebarMemberAgentShowsMember` (member m1 with agent → m1 is the shown tab, dock owned by m1), `TestToastGoOpensWaitingAgent` (two Waiting, earliest opens; none waiting → just switches), `TestBoardCardOpensDock`, `TestDockForgetsGoneAgent` (tracker forgets → `dockOf(a)==nil`, no panic over 2 frames), `TestNoFloatingAgentPane` (after any entry, `win.fl.sets[a]` has no pane with a negative id).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestSidebarAgent|TestToastGo|TestBoardCard|TestDockForgets|TestNoFloating'` → FAIL.
- [ ] **Step 3: Implement** the entry points, then delete the floating agent pane code and migrate tests.
- [ ] **Step 4: Run** `go test ./internal/ui/ && go vet ./...` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Open subagents in the dock and retire floating agent cards`

### Task 10: Open the report in the reader

**Files:**
- Modify: `internal/ui/docs.go` (in-memory docs), `internal/ui/doc_reload.go` (skip reload when no path), `internal/ui/doc_draw.go` (header label for in-memory docs), `internal/ui/sidebar_docs.go` if it shows the path
- Test: `internal/ui/docs_test.go`

**Interfaces:**
- Produces: `func (win *window) openReportDoc(session tabs.TabID, title, md string) *docView` — a `docView` with `path == ""`, `key = "report:" + title`, `name = title`, `doc = mdoc.Parse([]byte(md))`; reopening the same key shows the existing one with refreshed text. Header shows `Agent report` instead of path and `Loaded …`. Relative `.md` links in it resolve against the session's folder.

- [ ] **Step 1: Write the failing tests** `TestOpenReportDoc` (opens, nested under the session, titled `Explore · Find auth`, renders the markdown), `TestReportDocNoReload` (a reload tick with no path makes no file reads and does not mark `removed`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestOpenReportDoc|TestReportDocNoReload'` → FAIL.
- [ ] **Step 3: Implement**, wire the panel's Open in reader button (Task 8) to `openReportDoc(session, a.Type+" · "+a.Task, win.dockReport(a))`.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Open an agent's report in the reader`

### Task 11: Bench, audit note, docs

**Files:**
- Create: `internal/ui/dock_bench_test.go`, `docs/audits/2026-11-03-agent-dock.md`
- Modify: `docs/HANDOFF.md` (status entry), `README.md` (subagents section: docked panel, tabs, drag height, Esc rule; Roadmap)

- [ ] **Step 1: Write** `BenchmarkDockLongReport` (≈100 KB generated markdown report with headings, lists, a table and code: first layout, then 100 scrolled frames) and `BenchmarkDockIdleFrame` (open panel on a Done agent, steady frames). Bounded input sizes.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run '^$' -bench 'BenchmarkDock' -benchmem` and record ns/op and allocs in the audit note. Target: a steady frame does no parse (0 parses) and a scrolled frame stays well under 16.7 ms.
- [ ] **Step 3: Headless captures**: render the panel (running, waiting, report) in Midnight and Washi using the existing headless capture tooling (`tools/capture`), inspect them, and reference them in the audit note as development captures (not product screenshots).
- [ ] **Step 4: Full check**: `go test ./...`, `go vet ./...`, `GOOS=darwin go vet ./internal/ui/` (cgo permitting) or a Windows amd64 build plus the CI on the PR for macOS.
- [ ] **Step 5: Update** HANDOFF and README, then commit `docs(agents): Record the docked agent panel`.
