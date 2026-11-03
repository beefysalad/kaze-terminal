# Agents Board Redesign and Sidebar Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop 0.25's project rail and safe click (a click on an agent line opens its panel again, "+N more" opens the board for that session) and rebuild the agents board as a filterable, searchable list of per-session cards with a live preview pane.

**Architecture:** One shared task first fixes the board's new state and the cross-lane contract (`openBoard(tab)` sets a session token). Then two lanes run in parallel worktrees. **Lane S** (sidebar files only) deletes the rail, the `›` button, the double-click rule, the expanded tray and "Show fewer", and points the chip at `openBoard(tab)`. **Lane B** (board files, `dock_draw.go`, `window.go`) adds a pure board view model (`board_model.go`), draws the list (`board_list.go`) and a preview (`board_preview.go`) that reuses the docked panel's timeline, report layout and polling through a board-owned `dockSession`, then wires clicks, keys and the search editor. A final task adds captures, the 50-agent frame measurement, the audit, README and HANDOFF.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`widget.Editor`, `gpu/headless` for opt-in captures), existing `internal/subagents`, `internal/render.Readable`.

**Spec:** `docs/superpowers/specs/2026-11-03-kaze-agents-board-design.md` (look: `docs/design/agents-board-mockup.html`; sidebar highlight: `docs/design/sidebar-active-highlight-mockup.html`, option 3 "Session bar only"). On behaviour the spec wins; on look the mockup wins.

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: fixed fixture sizes (at most 50 agents), no unbounded loops, `time.After` on any wait.
- No global keyboard or mouse input. UI tests call `boardPress`/`boardKey`/`sidePress` or queue `pointer.Event`s on an `input.Router` (`sideClick`, `sideClickAt`, `clickAt`), never SendInput.
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`). Every `go test` in this plan means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`
  or in PowerShell `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags ("-B 0x{0:x16}" -f (Get-Random -Maximum ([long]::MaxValue))) <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off.
- Lean: the board draws only while shown; the preview reads a transcript only while it is drawn, at most once per `agentTranscriptPoll` (1 s), with `subagents.Transcript`'s caps, and stops once a finished agent's transcript was read to the end (`dockTick`'s rule). No timers or animation anywhere.
- The sidebar keeps zero allocations on unchanged frames: `TestSidebarCache*` keep passing.
- Colours come only from the theme (`Chrome, Active, Border, Muted, Accent, Needs, Done`, plus `ActiveFG` and `memberColor("magenta")`) with local `render.Readable` nudges. Every board text is ≥ 4.5:1 on its surface in all five `theme.All()` themes, amber on the Needs you tint included.
- Type: `c.sans` (Manrope) for the title, labels, buttons, filter and empty states; `c.fitSide`/`c.label` (JetBrains Mono) for agent data (type, task, step, elapsed, repo/branch). Section labels in caps, 11 sp, letterspaced as the mockup's `.sect`.
- Copy fixed by the spec: "Agents", "N agents in M sessions", "Back to <session>", segments "All", "Needs you", "Running", "Done", "NEEDS YOU", "Go to session ›", "✓ N finished ›", "Hide finished", "No agents match" / "Try another filter, or clear the search.", "No subagents yet", "Pick an agent" / "Its steps, messages and report show here.", "Open in session", "Copy report", "Open in reader", eyebrow labels "NEEDS YOU" / "RUNNING" / "DONE", "In <session> · <repo>".
- Keep `doubleClick` (500 ms) in `sidebar_window.go`: rename still uses it and the board reuses it.
- Lanes: after Task 1, Lane S edits only `internal/ui/sidebar*.go`, `internal/ui/sidebar*_test.go`, `internal/ui/active_project_test.go`, `internal/ui/dock_entry_test.go`; Lane B edits only `internal/ui/board*.go`, `internal/ui/board*_test.go`, `internal/ui/dock_draw.go`, `internal/ui/window.go`. Each lane works in its own worktree off the Task 1 commit; merge both into `feat/agents-board` before Task 10.
- Commits: `type(scope): Verb ...`, **no AI attribution lines of any kind** (CLAUDE.md overrides any default). Branch `feat/agents-board`; never push to `main`; the owner merges; release 0.26 follows as its own PR (not in this plan).

## Review Focus

1. **The previewed agent disappears** (its × , Clear, the session's next prompt, the session closing): the preview falls back to "Pick an agent", drops its transcript and nothing panics. (Task 9 test `TestBoardSelectionGone`.)
2. **Esc with text in the search**, focused or not, clears the text and keeps the board open; only an Esc on an empty search closes it. A `/` typed inside the field is text, not a refocus. (Task 9 tests `TestBoardEscClearsSearchFirst`, `TestBoardSlashTypesInSearch`.)
3. **The session token's session closes while the board is open**: the token goes away and the board shows all sessions, never an empty "session ×" view. (Task 9 test `TestBoardTokenDropsWithSession`.)
4. **Arrow keys at the ends, on an empty list, and past folded finished rows**: the selection stays on a visible row, never wraps, never lands on a folded one. (Task 9 test `TestBoardArrowsStayInVisibleRows`.)
5. **A narrow window**: no preview is drawn, a click opens the agent in its session, and no transcript is read for a preview nobody sees. (Task 9 test `TestBoardNarrowClickOpensSession`.)

---

### Task 1 (shared, first): Board state and the session-filter contract

**Files:**
- Modify: `internal/ui/board.go` (`boardState`, `openBoard`, new helpers)
- Modify: `internal/ui/dock_entry_test.go` (move `TestBoardCardOpensDock` out, verbatim), `internal/ui/sidebar_tray_input_test.go` (receive `TestTrayBoardLinkOpensFilteredBoard`, verbatim)
- Test: `internal/ui/board_test.go` (receives `TestBoardCardOpensDock`; loses `TestTrayBoardLinkOpensFilteredBoard`)

**Interfaces:**
- Produces (the cross-lane contract):
  - `func (win *window) openBoard(tab tabs.TabID)`: signature unchanged. `tab != 0` sets the session token (`b.tab = tab`); `0` shows all. Every open resets `filter = boardAll`, empties the search (`b.ed.SetText("")`), `searching = false`, `scroll = 0`, drops `prev` and `lastRow`, prunes `showDone` entries whose tab is gone (`win.mgr.Lookup == nil`), and selects `win.firstWaiting(tab)`. **Lane S calls only this.**
  - `type boardFilter uint8` with `boardAll, boardNeeds, boardRunning, boardDone` (in that order; used as an index into `[4]` arrays).
  - New `boardState` fields: `filter boardFilter; ed widget.Editor; searching bool; edKeys []event.Filter; sel, hover subagents.Key; showDone map[tabs.TabID]bool; lastRow boardClick; prev dockSession; prevHits dockHits; wide bool; redraws int`. `type boardClick struct{ key subagents.Key; at time.Duration }`.
  - `func (win *window) clearBoardSession()`: `b.tab = 0`, keeps the board open, invalidates.
  - `func (win *window) boardSelect(k subagents.Key)`: sets `sel`; when `k` differs from `prev.agent`, `b.prev = dockSession{agent: k, follow: true}`.
  - `func (win *window) firstWaiting(tab tabs.TabID) subagents.Key`: the first agent in `win.subs.Board()`'s waiting list (start order) whose tab is `tab` (any tab when `tab == 0`); zero when none.
- The old fields `cards`, `all`, `clear` stay until Task 7 replaces them, so this task compiles alone.

- [ ] **Step 1: Write the failing tests** in `board_test.go`:
  - `TestOpenBoardResetsViewAndSelectsWaiting`: `boardWindow`; set `b.filter = boardDone`, `b.ed.SetText("x")`, `b.searching = true`; `openBoard(id)` → `b.tab == id`, `b.filter == boardAll`, `b.ed.Text() == ""`, `!b.searching`, `b.sel == subagents.Key{Session: "s", ID: "wait"}`, `b.prev.agent == b.sel`; `closeBoard(); openBoard(0)` → `b.tab == 0`.
  - `TestOpenBoardWithoutWaitingSelectsNothing`: `liveWindow(t, 2)` with one running agent via `startAgent` → after `openBoard(0)`, `b.sel == (subagents.Key{})`.
  - `TestClearBoardSession`: `openBoard(id); clearBoardSession()` → `b.open && b.tab == 0`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestOpenBoard|TestClearBoardSession'` → FAIL (undefined fields and functions).
- [ ] **Step 3: Implement** the interfaces above. Move the two tests between files without changing their bodies.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Board|Tray|Dock'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Give the agents board a filter, search and selection state`

---

## Lane S: sidebar (Tasks 2 to 4, in order)

### Task 2 (Lane S): Remove the project rail

**Files:**
- Modify: `internal/ui/sidebar.go` (delete the rail block in `sideList` after the group rows, the `rail` field of `sideHits`; fix the `groupNameStyle` comment: "the project you are in is bold")
- Test: `internal/ui/active_project_test.go` (delete `TestActiveProjectRail` and `railFixture`), `internal/ui/sidebar_tray_render_test.go` (comment: no rail)

**Interfaces:** none new. The selected session's own 2 dp accent bar and `th.Active` row fill (`sidebar.go`, `on` branch of `row`) stay as they are.

- [ ] **Step 1: RED by deletion:** remove `sideHits.rail` first; run `go test ./internal/ui/ -run TestActiveProject` → FAIL to compile (`h.rail undefined` in `TestActiveProjectRail`). No pixel test exists for a fill; the captures in Task 10 are the visual check (Ruling 9).
- [ ] **Step 2: Implement:** delete the rail drawing and `TestActiveProjectRail`/`railFixture`; keep `TestActiveProjectNameStyle`, `TestSidebarTextContrast`, `TestActiveProjectFollowsSelectedSession`.
- [ ] **Step 3: Run** `go test ./internal/ui/ -run 'TestActiveProject|TestSidebar'` → PASS (`TestSidebarCache*` included).
- [ ] **Step 4: Commit** `fix(ui): Drop the project rail beside the selected session`

### Task 3 (Lane S): A click on an agent line opens its panel

**Files:**
- Modify: `internal/ui/sidebar_agents.go` (`trayLines`: no `›`; `sideAgentHit.btn` removed; `hitAgentLine` comment), `internal/ui/sidebar_window.go` (`sidePress`, delete `agentClick`, `sideUI.lastAgent` and its resets; `sidePointer` comment "› included" goes)
- Test: `internal/ui/sidebar_tray_input_test.go`, `internal/ui/sidebar_agents_test.go` (`TestSidebarDrawsTray`), `internal/ui/dock_entry_test.go` (`TestSidebarAgentOtherSession`, `TestNoFloatingAgentPane`)

**Interfaces:**
- Consumes: `showAgent(tab tabs.TabID, k subagents.Key, toggle bool)` (`panes_agent.go`, unchanged).
- Produces: `sideAgentHit{key; tab; kind; open bool; rect image.Rectangle; tip string}` (no `btn`). In `sidePress`, `hitAgentLine` → `win.showAgent(a.tab, a.key, true)`. Elapsed is right-aligned at `x1 - 6dp`; the task text gets the room the `›` box had.

- [ ] **Step 1: Write the failing tests** in `sidebar_tray_input_test.go` (`lineSpot` becomes `centre(agentHit(t, win, k).rect)`):
  - `TestAgentLineTogglesPanel` (replaces `TestAgentLineShowsSession` and `TestAgentButtonTogglesPanel`): `paneWindow`, agent k on a; click the line at 0 → `dockOf(a.ID).agent == k` and `agentHit(t, win, k).open`; click again at `2*doubleClick` → `dockOf(a.ID) == nil`.
  - `TestAgentLineOpensOtherSession`: b shown, click a's line → `mgr.Active().ID == a.ID` and `dockOf(a.ID).agent == k`.
  - `TestAgentLineQuickClicksToggleTwice`: presses at 0 and 100 ms on one line → panel closed (no double-click rule left).
  - Delete `TestAgentLineDoubleClickOpensPanel`, `TestAgentLineDoubleClickNeedsSameLine`, `TestAgentLineMatchesRowClick` (behaviour reversed by spec 2).
  - `TestTrayTooltipShowsStep`: move to `centre(agentHit(...).rect)`.
  - `TestSidebarDrawsTray`: replace the `btn` checks with `a.rect.Max.X == h.rows[0].rect.Max.X`.
  - `dock_entry_test.go`: `sideClick(win, r, agentHit(t, win, y).rect, ...)` in `TestSidebarAgentOtherSession` and `TestNoFloatingAgentPane`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestAgentLine|TestTrayTooltip|TestSidebarDrawsTray|TestSidebarAgent|TestNoFloating'` → FAIL (line click only selects; `btn` still referenced).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS, `TestSidebarCache*` included.
- [ ] **Step 5: Commit** `feat(ui): Open an agent's panel with a click on its line again`

### Task 4 (Lane S): "+N more" opens the board for that session

**Files:**
- Modify: `internal/ui/sidebar_agents.go` (delete `trayMax`, `hitAgentFewer`, `hitAgentBoard`, `trayFooter`, `narrowFooter`, `sideTray.Expanded`; `sideAgents` always uses `trayCap`; `trayChipLine` is `!r.TrayFolded && r.Hidden.Total() > 0`; `trayChip` draws only the chip), `internal/ui/sidebar_model.go` (delete `sideRow.TrayExpanded`), `internal/ui/sidebar_window.go` (delete `sideUI.trayOpen`; `pruneTray` prunes `trayFolded` only; `hitAgentChip` → `win.openBoard(a.tab)`), `internal/ui/sidebar_cache.go` (delete `trayOpen` compare and clone)
- Test: `internal/ui/sidebar_tray_input_test.go`, `internal/ui/sidebar_agents_test.go`, `internal/ui/sidebar_cache_test.go`, `internal/ui/sidebar_tray_render_test.go`

**Interfaces:**
- Consumes: `openBoard(tab tabs.TabID)` (Task 1 contract: sets the session token).
- Produces: `agentHitKind` is `hitAgentLine, hitAgentChip` only. Chip text and colours are unchanged (`trayChip`, `narrowChip`, `⌄` kept; Ruling 10).

- [ ] **Step 1: Write the failing tests**:
  - `TestTrayChipOpensBoardForSession` (replaces `TestTrayChipExpandsAndFewerCollapses` and the moved `TestTrayBoardLinkOpensFilteredBoard`): 12 agents on a; click the chip → `win.board.open && win.board.tab == a.ID`, `dockOf(a.ID) == nil`; `closeBoard()`, two frames → still `trayCap` line hits and one chip.
  - `TestTrayStateLimits`: default 3 lines and `Hidden.Total() == 9`; folded → 0 lines, `TrayTotal == 12`.
  - `TestTrayStateRebuildsModel`: folding rebuilds once (`builds+1`).
  - `TestTrayStatePrunedWithSession`: only `trayFolded` is set and pruned.
  - Delete `TestSidebarDrawsTrayFooter`, `TestTrayFooterKeepsBoardLinkAtDefaultWidth`.
  - `TestSidebarCacheTrayAllocatesNothing`: drop the `trayOpen` line; hit floor `trayCap+1+trayCap+1`; still `AllocsPerRun == 0`, `builds`/`draws` unchanged.
  - `TestSidebarTrayRenders`: states `default`, `folded`, `tooltip`, `open` (an open agent line, `st.openAgent = tray.Agents[1].Key`); themes `sumi`, `midnight`, `washi`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestTray|TestSidebarCache|TestSidebarDraws'` → FAIL (the chip expands in place).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` and `go vet ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Send a tray's "+N more" to the board for that session`

---

## Lane B: board (Tasks 5 to 9, in order)

### Task 5 (Lane B): Board view model

**Files:**
- Create: `internal/ui/board_model.go`, `internal/ui/board_model_test.go`
- Modify: `internal/ui/board.go` (add `sessionMeta`; `boardColumns`, the old `boardCard` and `drawBoard` stay untouched until Task 7)

**Interfaces:**
- Consumes: Task 1 state; `win.subs.Board()`, `win.subs.SessionWaits()`, `win.sessionIDs()` (sidebar order), `sideAgentOf(a, now)` (`Elapsed`, `Action`), `win.rowAgent(id).agentName()`, `win.locOf(t)`.
- Produces:
  - `type boardRow struct { key subagents.Key; tab tabs.TabID; session bool; state subagents.State; typ, task, step, time string }` (`step` = `sideAgentOf(...).Action`; in the Needs you card it is `"<session title> · " + step`; a session wait is `typ "Session"`, `task` = title, `step "a subagent may be asking"`).
  - `type boardGroup struct { tab tabs.TabID; title, meta, agent string; meter subagents.TrayCounts; rows []boardRow; done int; folded bool }` (one session card; named so it does not clash with 0.25's `boardCard` until Task 7 deletes that) (`meta` = `"repo · branch"`, either part dropped when empty; `meter` counts **all** that session's agents; `done` = matching finished rows; `folded` true when they are hidden).
  - `type boardView struct { needs []boardRow; cards []boardGroup; counts [4]int; agents, sessions int; order []boardRow }` (`counts` indexed by `boardFilter`, session token applied, search and segment not; `agents`/`sessions` global, for the summary; `order` = selectable rows top to bottom, session waits and folded rows excluded).
  - `func (win *window) boardView() boardView` — reads `b.filter`, `b.tab`, `b.ed.Text()`, `b.showDone`.
  - `func boardMatch(r boardRow, title, repo, lowerQuery string) bool` — `lowerQuery == ""` or a case-insensitive substring of type, task, title or repo.
  - `func boardSummary(agents, sessions int) string` — `"11 agents in 3 sessions"`, `"1 agent in 1 session"`, `""` for 0 agents.
  - `func meterSplit(c subagents.TrayCounts, width, gap int) (needs, run, done int)` — proportional widths; a zero count gets 0 and no gap; a non-zero count gets ≥ 1 px; widths plus gaps between non-zero parts equal `width`.
  - `func (win *window) sessionMeta(id tabs.TabID) (title, repo, branch string)`; `sessionName` stays and calls it.
- Rules: cards follow `sessionIDs()` order, only sessions with matching rows. Under `boardAll`, waiting rows (and session waits) go to `needs` only; card rows are running (start order) then done (newest first). Under `boardNeeds`/`boardRunning`/`boardDone`, `needs` is empty and cards hold only that state's rows (Ruling 3). Done rows show when `filter == boardDone` or `showDone[tab]`; otherwise `folded = done > 0` and they are left out of `rows` and `order`.

- [ ] **Step 1: Write the failing tests** (fixture `boardWindow` plus a second session where noted):
  - `TestBoardViewNeedsCardAndCards`: under All, `needs[0].key.ID == "wait"` with `step` starting `title + " · "`; one card for `id`, rows `[run]`, `done == 1`, `folded`, `meter == {Waiting:1, Running:1, Done:1}`; `order` keys `[wait, run]`.
  - `TestBoardViewSidebarOrder`: agents in both tabs → `cards` tab order equals `sessionIDs()` filtered to those tabs.
  - `TestBoardViewFilters`: Needs → `needs` empty, card rows `[wait]`; Running → `[run]`; Done → `[done]`, `!folded`.
  - `TestBoardViewSearch`: `"PLAN"` matches type Plan only; the session title and the repo match all three; `"zzz"` → no needs, no cards; `counts == [3,1,1,1]` in every case.
  - `TestBoardViewSessionToken`: second session with agent `x`; `b.tab = id` → one card; `counts[boardAll] == 3`; `agents == 4`, `sessions == 2`.
  - `TestBoardViewShowDone`: `showDone[id] = true` → rows `[run, done]`, `!folded`, `order` ends with `done`.
  - `TestBoardViewSessionWait`: `KindNeeds` with no `AgentID` → a `needs` row with `session`, `typ == "Session"`, not in `order`.
  - `TestBoardSummary`, `TestMeterSplit` (`{1,2,1}`, 100, 2 → sums to 100, `run ≈ 2*needs ± 1`; `{0,5,0}` → `(0,100,0)`; `{1,1000,0}` → `needs >= 1`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBoardView|TestBoardSummary|TestMeterSplit'` → FAIL.
- [ ] **Step 3: Implement** in `board_model.go` only; the 0.25 drawing and its tests keep passing.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run Board` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Model the agents board as session cards with filters and search`

### Task 6 (Lane B): Board colours and contrast

**Files:**
- Create: `internal/ui/board_ink.go`
- Test: `internal/ui/board_model_test.go` (or `board_ink_test.go`)

**Interfaces:**
- Consumes: `c.dockColors(bg) dockPalette` (`dock_draw.go`), `mix`, `tint`.
- Produces: `type boardInk struct { bg, card, raised, field, line, needsBG, needsLine, meterDone, priBG color.NRGBA; page, onCard, onNeeds dockPalette; priFG color.NRGBA }` from `func (c *chrome) boardInk() boardInk`:
  `bg = th.Active`; `card = mix(th.Active, th.ActiveFG, 0.035)`; `raised = mix(th.Active, th.ActiveFG, 0.07)` (hover, selection, buttons); `field = th.Chrome` (segment and search); `line = th.Border`; `needsBG = mix(card, th.Needs, 0.07)`; `needsLine = mix(th.Border, th.Needs, 0.4)`; `meterDone = mix(th.Border, th.Muted, 0.3)`; `priBG = th.Accent`; `priFG = render.Readable(th.ActiveFG, th.Accent)`; `page = c.dockColors(bg)`, `onCard = c.dockColors(card)`, `onNeeds = c.dockColors(needsBG)`.

- [ ] **Step 1: Write the failing test** `TestBoardContrast`: for each `theme.All()`, every field of `page` on `bg`, of `onCard` on `card` and on `raised`, of `onNeeds` on `needsBG`, and `priFG` on `priBG` is ≥ 4.5 (`contrast` from `statusbar_test.go`); also `onCard.text` on `field`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestBoardContrast` → FAIL (undefined).
- [ ] **Step 3: Implement.** If `onCard` fails on `raised` in a theme, compute it on `raised` (the darker-contrast case) and recheck.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(ui): Add the agents board's theme colours with contrast checks`

### Task 7 (Lane B): Draw the list pane

**Files:**
- Create: `internal/ui/board_list.go`
- Modify: `internal/ui/board.go` (`drawBoard` splits the area; hit types; `boardPress` list hits)
- Test: `internal/ui/board_test.go` (rewrite the 0.25 board tests), `internal/ui/board_setup_test.go` (new `boardPress` signature)

**Interfaces:**
- Consumes: `boardView`, `boardInk`, `meterSplit`, `boardSummary`, `agentLogo`, `c.stateDot`, `c.lineEditor`, `stroke`, `place`.
- Produces:
  - `const boardWideDp = 860` (Ruling 1). `drawBoard(gtx, r)` sets `b.wide = r.Dx() >= gtx.Dp(boardWideDp)`; wide: list = left `r.Dx()*105/205`, a 1 px `ink.line` divider, preview = the rest (Task 8 draws it; until then it stays blank); narrow: list = `r`.
  - `func (win *window) drawBoardList(gtx layout.Context, r image.Rectangle, v *boardView, ink boardInk)`.
  - `type boardRowHit struct { row boardRow; rect, close image.Rectangle }`; `type boardCardHit struct { tab tabs.TabID; rect, goTo, clear, fold, meter image.Rectangle }`.
  - `boardState` hit fields: `back, setup, search, token, list, preview image.Rectangle; seg [4]image.Rectangle; rows []boardRowHit; cardHits []boardCardHit; texts []string` (old `cards`, `all`, `clear` deleted).
  - `func (win *window) boardPress(gtx layout.Context, p image.Point, at time.Duration)` (this task: back, setup, segments → `b.filter`, token → `clearBoardSession`, search → `b.searching = true` and `wantFocus`, goTo → `closeBoard(); selectTab(tab)`, clear → `subs.ClearDone(int(tab))` + `dropGoneAgents`, fold → flip `showDone[tab]`, row `close` → `subs.Dismiss` + `dropGoneAgents`; a row press still opens the agent in its session, as in 0.25, until Task 9 makes it select). Delete `boardColumns`, the old `boardCard`, `boardHit`, `TestBoardColumns` and `TestBoardFilteredToTab` (covered by Task 5).
- Look (mockup `.bhead`, `.seg`, `.search`, `.card`, `.chead`, `.meter`, `.brow`, `.donefold`, `.empty`): header pad 20/22 dp; "Agents" 22 sp SemiBold; summary 12.5 sp `page.muted`; "Back to <title>" (no ×, Ruling 11) as a `raised` pill with a 1 px line at the right. Tools row 14 dp below: segmented control on `field` with `raised` "on" segment, each label plus its count in mono 11 sp (`page.muted`; `page.needs` for Needs you when > 0); search field (28 dp, radius 7 dp, `field`, 1 px line, `ink.priBG` line while `searching`) drawn with `c.lineEditor(gtx, &b.ed, ..., "Search agents", page.muted, 28dp)`; the token "<title> ×" as a `raised` pill right of the search when `b.tab != 0`. Cards: 12 dp apart, radius 10 dp, `card` fill and 1 px `line`; head 10/14/8 dp: logo 12 dp, title 12.5 sp SemiBold, meta 11 sp mono `onCard.muted`, then right-aligned "Clear" (only when the session has finished agents, Ruling 5) and "Go to session ›" 11.5 sp Medium `onCard.muted`. Meter: 3 dp tall, 14 dp side margins, 6 dp below, 2 dp gaps, `needs`/`th.Accent`/`meterDone`. Rows: 44 dp, 1 px top line except the first, columns mark 16 dp · type 84 dp (mono 12 sp `onCard.kind`) · task (12.5 sp `onCard.text`; `onCard.muted` when done) · elapsed 54 dp right-aligned (11 sp `onCard.muted`), step line under the task spanning task and elapsed (11 sp `onCard.muted`). Hovered or selected row: `raised` fill; selected also a 2 dp `th.Accent` bar on its left; a done row that is hovered or selected shows "×" at its right with a `close` hit. Fold line 30 dp: "✓ N finished ›" (✓ in `onCard.done`) or "Hide finished". Needs you card: `needsBG` fill, `needsLine` border, head "NEEDS YOU" 11 sp letterspaced `onNeeds.needs`, rows in `onNeeds` colours. Scroll: today's `b.scroll` clamp over the list body only. Empty states centred, 56 dp from the top: no agents at all → "No subagents yet" / "They show here when Claude Code or Codex starts helpers."; otherwise "No agents match" / "Try another filter, or clear the search.". The hooks hint keeps its text and `b.setup`.

- [ ] **Step 1: Write the failing tests** (`wide := image.Rect(260, 0, 1400, 860)`, `gtx := pageContext(1400, 860)`):
  - `TestBoardDrawsHeaderAndSegments`: texts contain "Agents", `boardSummary(3, 1)`, "Back to " + title, "All", "Needs you", "Running", "Done"; `seg[0..3]` non-empty, left to right; `b.search.Min.X > b.seg[3].Max.X`.
  - `TestBoardNeedsCardFirst`: `b.rows[0].row.key.ID == "wait"`, above `b.cardHits[0].rect`; texts contain "NEEDS YOU".
  - `TestBoardCardHeaderAndMeter`: texts contain the title and "Go to session ›"; `cardHits[0].meter.Dy() == gtx.Dp(3)`; `goTo` non-empty.
  - `TestBoardDoneFold`: texts contain "✓ 1 finished ›", no row hit for "done"; press `fold` → row hit for "done" and "Hide finished" drawn; press again → folded.
  - `TestBoardSegmentPress`: press `seg[boardDone]` → `b.filter == boardDone`, only the "done" row hit.
  - `TestBoardTokenClears`: `openBoard(id)` → texts contain title + " ×"; press `b.token` → `b.tab == 0`.
  - `TestBoardGoToSession`: press `goTo` → `!b.open`, `mgr.Active().ID == id`, no dock opened.
  - `TestBoardDismissDoneCard` (rewrite): `showDone[id] = true`, `b.hover = done key`, draw → the "done" row has `close`, "run" has none; press it → `subs.Get(done) == nil`, board open.
  - `TestBoardClearDone` (rewrite): press `cardHits[0].clear` → no done rows, run and wait remain; next draw → `clear.Empty()`.
  - `TestBoardEmptyStates`: search `"zzz"` → "No agents match", "Try another filter, or clear the search."; a `liveWindow` with no agents → "No subagents yet".
  - Keep and adapt: `TestBoardEscAndKeyClose`, `TestBoardHooksMissingHint`, `TestBoardSetupHintOnUnreadableSettings` (call `boardPress(gtx, p, 0)`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run Board` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw the agents board as filtered session cards`

### Task 8 (Lane B): Preview pane

**Files:**
- Create: `internal/ui/board_preview.go`, `internal/ui/board_preview_test.go`
- Modify: `internal/ui/dock_draw.go` (`drawPanelMarkdown`, `drawDockActivity`, `drawDockReport` take `texts *[]string` as their last parameter; `drawDock` passes `&win.dock.texts`; `dockReport` also uses `win.board.prev.tr` when `win.board.prev.agent == a.Key`), `internal/ui/board.go` (`drawBoard` calls the preview when `b.wide`)

**Interfaces:**
- Consumes: `dockTick(s *dockSession, a *subagents.Agent, now time.Time) (time.Time, bool)`, `drawDockActivity`, `drawDockReport`, `mdLayout`, `dockTimeline`, `agentElapsed`, `boardInk`, `copyReport`, `openReportDoc`.
- Produces:
  - `func (win *window) drawBoardPreview(gtx layout.Context, r image.Rectangle, ink boardInk)`; preview hits in `boardState`: `open, copy, reader image.Rectangle` and `b.prevHits` (body, links, scrollMax).
  - Behaviour: only when `b.wide` and `b.sel` names a live agent: `dockTick(&b.prev, a, gtx.Now)` and `gtx.Execute(op.InvalidateCmd{At: at})` when it says so. Header pad 20/24 dp: eyebrow (11 sp) state label in `page.needs`/`page.accent`/`page.done`, "·", the type in a `raised` pill (`page.kind`), elapsed; task 17 sp SemiBold; "In <title> · <repo>" 12 sp `page.muted`; actions 28 dp, radius 7 dp, 8 dp apart: "Open in session" (`priBG`/`priFG`), and when done "Copy report", "Open in reader" (`raised`). Body on `win.pal.BG` (Ruling 6), section label "ACTIVITY" or "REPORT", then `drawDockActivity` (running, waiting: its `itemWaiting` row is the approval box) or `drawDockReport` (done) with `c.dockColors(win.pal.BG)`. Nothing selected: centred "Pick an agent" / "Its steps, messages and report show here.".

- [ ] **Step 1: Write the failing tests** (reuse `dockScene` from `dock_render_test.go` for a transcript-backed agent "x"):
  - `TestBoardPreviewPickAnAgent`: no waiting agent, wide draw → texts contain both "Pick an agent" lines; `b.prev.tr == nil`.
  - `TestBoardPreviewRunning`: `dockScene(..., "running")`, `boardSelect(k)`, draw → texts contain "RUNNING", "Explore", "Split pane keys", "In ", "Open in session", "ACTIVITY", "Looking at panes.go"; not "Copy report"; `b.open` non-empty.
  - `TestBoardPreviewWaiting`: `"waiting"` → "NEEDS YOU" and a text starting "⏸ ".
  - `TestBoardPreviewDone`: `"report"` → "DONE", "REPORT", "Copy report", "Open in reader", report text without `**`.
  - `TestBoardPreviewParsesOnce`: 10 unchanged draws → `b.prev.cache.parses` unchanged after the first.
  - `TestBoardPreviewStopsPollingWhenDone`: finished agent read after its end → `dockTick(&b.prev, a, a.Ended.Add(time.Minute))` returns `false`.
  - `TestBoardCopyReportUsesPreviewTranscript`: preview on a done agent whose transcript holds a longer report than the hook's → `toClipboard` receives the transcript's report.
  - All `TestDock*` keep passing unchanged.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBoardPreview|TestBoardCopy|TestDock'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Preview the selected agent beside the board`

### Task 9 (Lane B): Selection, keys, search editor and narrow fallback

**Files:**
- Modify: `internal/ui/board.go` (`boardEvents`, `boardKey`, `boardPress` rows and preview actions, `boardSearchEvents`), `internal/ui/window.go` (`focusTarget`: `if win.board.open && win.board.searching { return &win.board.ed }` before `&win.board.tag`)
- Test: `internal/ui/board_input_test.go` (new), `internal/ui/board_test.go` (`TestBoardCardOpensPane`, moved `TestBoardCardOpensDock`)

**Interfaces:**
- Consumes: `boardView().order`, `boardSelect`, `showAgent(tab, k, false)`, `edKeys`, `edShortcuts`, `doubleClick`.
- Produces:
  - `func (win *window) boardOpenAgent(r boardRow)`: `closeBoard()`, then `selectTab(r.tab)` for a session wait, else `showAgent(r.tab, r.key, false)`.
  - `func (win *window) boardMove(d int)`: next/previous in `order`, clamped (no wrap); from no selection, Down picks the first; sets `b.reveal` so the next draw scrolls the row into view.
  - `func (win *window) boardSearchEsc()`: non-empty text → clear it, `searching = false`; empty → `closeBoard()`.
  - `func (win *window) boardSearchEvents(gtx layout.Context)`: called from `boardEvents`; `SingleLine`, `Submit`; `edShortcuts(gtx, &b.ed, b.edKeys, win.boardSearchEsc)`; `ChangeEvent` → `b.scroll = 0`, invalidate; `SubmitEvent` → `searching = false`, keeps the text (Ruling 7).
  - Behaviour: row press, wide: session wait → `boardOpenAgent`; same key as `lastRow` within `doubleClick` → `boardOpenAgent`; else `boardSelect` and remember `lastRow`. Narrow: any row press → `boardOpenAgent`. Preview `open` → `boardOpenAgent`; `copy` → `copyReport(gtx, a)`; `reader` → `closeBoard(); openReportDoc(tab, a.Type+" · "+a.Task, dockReport(a))`. `boardKey`: `/` → `searching = true`, `wantFocus`; Esc → `boardSearchEsc()`; Up/Down → `boardMove(∓1)`; Enter/Return → open the selected row. Pointer: `Move|Enter|Leave` set `b.hover` and invalidate only when it changed (`b.redraws++`); Scroll over the preview body scrolls `b.prev` (`follow = false`), elsewhere `b.scroll`. At draw: `b.tab` whose session is gone → 0; `b.sel` whose agent is gone → zero and `b.prev = dockSession{}`.

- [ ] **Step 1: Write the failing tests**:
  - `TestBoardClickSelects`: wide, press the "run" row → `b.sel.ID == "run"`, board open, `dockOf(id) == nil`.
  - `TestBoardDoubleClickOpensInSession`: presses at 0 and 100 ms → `!b.open`, `dockOf(id).agent.ID == "run"`; presses on two rows within 100 ms → no open.
  - `TestBoardOpenInSessionAndEnter`: press `b.open` → opened; reopen, `boardKey(Enter)` → opened.
  - `TestBoardArrowsStayInVisibleRows`: order `[wait, run]`; Down from `wait` → `run`; Down again stays `run`; Up twice → `wait`; `done` never selected while folded; empty search result → no panic, `sel` unchanged.
  - `TestBoardSlashFocusesSearch`: `boardKey("/")` → `b.searching`, `focusTarget() == &win.board.ed`.
  - `TestBoardSlashTypesInSearch`: with `searching`, queue `key.EditEvent{Text: "/"}` to the editor via an `input.Router` frame → `b.ed.Text() == "/"`, still `searching`.
  - `TestBoardEscClearsSearchFirst`: text "plan" → `boardSearchEsc()` → text "", board open; again → closed. Same through `boardKey(Esc)` with the field unfocused.
  - `TestBoardNarrowClickOpensSession`: draw at `image.Rect(260, 0, 1000, 600)` → `!b.wide`, `b.prev.tr == nil`; press "run" → `!b.open`, panel open on run.
  - `TestBoardSelectionGone`: select "done", `subs.Dismiss` it, draw → `b.sel` zero, texts contain "Pick an agent".
  - `TestBoardTokenDropsWithSession`: `openBoard(id)`, `closeTab(id)`, draw → `b.tab == 0`.
  - `TestBoardHoverRedrawsOnlyOnChange`: two Moves over one row → `b.redraws == 1`; a Move to another row → 2.
  - `TestBoardCardOpensPane` (rewrite: uses a narrow rect) and `TestBoardCardOpensDock` (narrow at `paneWindow`'s 1000 px) pass.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBoard'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` and `go vet ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Select, search and open agents from the board by mouse and keys`

---

### Task 10 (after both lanes): Captures, 50-agent frames, audit, README and HANDOFF

**Files:**
- Create: `internal/ui/board_render_test.go`, `internal/ui/board_bench_test.go`, `docs/audits/2026-11-03-agents-board.md`, `docs/audits/agents-board/*.png`
- Modify: `README.md` (`## Subagents`: the tray bullets and "The board"; `## Shortcuts` if needed; `## Roadmap`), `docs/HANDOFF.md` ("Where things stand" entry at the top)

**Interfaces:**
- Produces: `func boardFixture50(t testing.TB, win *window) []subagents.Key` (5 sessions × 10 agents: 1 waiting, 6 running, 3 done; one running agent with a 20-message transcript); `BenchmarkBoardFrame50` (wide 1400×860, `drawBoard` per op on a reset `op.Ops`, `b.ReportAllocs()`, preview on the transcript agent); `TestBoardFrame50Steady` (after 5 warm-up frames, 20 frames within one poll window: `b.prev.cache.parses` and `b.prev.trAt` unchanged; logs `testing.AllocsPerRun(20, frame)`).

- [ ] **Step 1: Write the failing test** `TestBoardFrame50Steady` → FAIL until `boardFixture50` exists; implement the fixture → PASS. Run `go test ./internal/ui/ -run '^$' -bench BenchmarkBoardFrame50 -benchmem -count 5`. For "before", copy `board_bench_test.go` (only `drawBoard`, `startAgent`, `onNotify`) into a `main` worktree (`git worktree add ../kaze-before main`) and run the same command there.
- [ ] **Step 2: Headless captures:** `TestBoardRenders`, gated by `KAZE_BOARD_RENDER_DIR` (pattern of `TestDockRenders`: `paneWindow`, `applyTheme`, `win.frame`, `headless.NewWindow`, `writeRender`), Sumi and Midnight, 1400×860: `all` (Needs you card, two cards, running preview), `done` (Done filter, report preview), `search` (session token and a query), `empty` ("No agents match"), `narrow` (1000×600, no preview); plus `TestSidebarTrayRenders` output from Task 4 (no rail, open line highlight). Save to `docs/audits/agents-board/`, compare with the mockup, label them development captures.
- [ ] **Step 3: Full check:** `go test ./...`, `go vet ./...`, and `GOOS=darwin go vet ./internal/ui/` where cgo allows.
- [ ] **Step 4: Write the audit** `docs/audits/2026-11-03-agents-board.md`: what changed (sidebar and board), the test list per task, the 50-agent frame table (ns/op, B/op, allocs/op, before and after), the contrast minimum per theme from `TestBoardContrast`, the captures, and the rulings below. README: tray bullets (a click opens the panel, a second closes it; "+N more" opens the board for that session; no `›`, safe click, expand or "Show fewer"), "The board" rewritten (filters, search, `/`, Esc, session token, Needs you card, cards with meter and folds, preview, ↑/↓/Enter, double-click, narrow fallback), Roadmap. HANDOFF: a dated "Where things stand" entry (built on `feat/agents-board`, release 0.26 next, screenshots need retaking per `docs/site-assets/SCREENSHOTS.md`).
- [ ] **Step 5: Commit** `docs(agents): Record the agents board audit, captures and frame numbers`

## Rulings made while planning

Each is cheap to overturn in its own task.

1. The preview shows when the board's own area is ≥ 860 dp (mockup's `minmax(460px)` + `minmax(400px)`), not the window width. Cost if wrong: one constant (Task 7).
2. Segment counts follow the session token but ignore the search and segment (mockup counts ignore the query); the "N agents in M sessions" summary is global. (Task 5)
3. The Needs you card appears only under All; the Needs you segment shows session cards holding waiting rows, as the mockup does. (Task 5)
4. A session wait (no subagent named) is a Needs you row that goes to its session; it cannot be previewed or reached by arrows. (Tasks 5, 9)
5. "Clear" sits in each card header (only with finished agents) and clears that session; the × shows on a done row when hovered or selected. (Task 7)
6. The preview body sits on `win.pal.BG`, the docked panel's background, so the reused timeline and report keep the contrast `TestDockContrast` already proves. (Task 8)
7. Esc in the search with text clears it and gives the keyboard back to the board; Enter keeps the text and does the same. (Task 9)
8. The board drops a gone session token or selection at draw time instead of adding a prune hook to `window.go`'s tab-list path. (Task 9)
9. The rail's removal has no pixel RED test (fills are not inspectable without a GPU); the RED is the compile failure of `TestActiveProjectRail`, and the captures check the look. (Task 2)
10. The chip keeps its text and `⌄` caret ("unchanged" in spec 2; the mockup keeps `⌄`) although it now opens the board. (Task 4)
11. "Back to <session>" loses 0.25's trailing ×, as in the mockup. (Task 7)
12. The preview's dock helpers take a `texts *[]string` so board tests read `win.board.texts`, not the panel's. (Task 8)
