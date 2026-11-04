# Dock Tabs, Alert Cards and Path Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship three approved issues as one 0.27 bundle: workspace members dock as tabs in the agent panel and the scratch shell and floating panes go away (#133), alert toasts become richer cards at the bottom right (#9), and Ctrl+click on `.md` paths works when the path starts earlier on the line or has a space in it (#128).

**Architecture:** Three lanes in separate worktrees off `feat/dock-alerts-links`, with no shared first task: the cross-lane contracts are functions that already exist and keep their signatures (listed below). **Lane L** (links) teaches `internal/render/links.go` to extend a `.md` link back across single spaces to a rooted path, adds click-time candidates (`Links.RefsAt`), and makes `openDocRef` try them in order with a clearer miss message. **Lane A** (alerts) sends the start of Claude's last reply on a done alert, builds a pure card model (title by the sidebar's rule, branch and command, status line), draws 380 dp cards in a new `alerts_card.go`, and points Ctrl+Shift+J / Cmd+J at the newest card. **Lane D** (dock) follows the issue's safe order: tabs in the docked panel first (agents only), then member tabs with their own PTY size and keyboard, then switch every member entry point to the dock, then remove the scratch shell and `Tab.Parent`, then delete the floating panes. A final task merges the lanes, runs the whole check and updates README, lander and HANDOFF.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`gpu/headless` for opt-in captures), existing `internal/render`, `internal/mdoc`, `internal/notify`, `internal/subagents`, `internal/tabs`, `render.Readable`.

**Inputs:** issues #133, #9 (the 2026-11-04 comment is the approved design and overrides the body), #128 (both comments); mockups `docs/design/dock-tabs-mockup.html` (option A, "one dock with tabs") and `docs/design/alert-card-mockup.html`. On behaviour the issue wins; on look the mockup wins.

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: fixed fixture sizes (at most 7 dock tabs, 4 cards, 16 wrapped rows, 512-rune lines), no unbounded loops, `time.After` on any wait, fake PTYs (`recStarter`) only.
- No global keyboard or mouse input. UI tests call the handlers (`dockHead`, `toastEvent`, `sidePress`) or queue `pointer.Event`/`key.Event`s on an `input.Router` (`clickAt`, `sideClick`, `typeKeys`, `dockKey`), never SendInput.
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`). Every `go test` in this plan means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`
  or in PowerShell `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags ("-B 0x{0:x16}" -f (Get-Random -Maximum ([long]::MaxValue))) <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off.
- Lean: no timers or animation. A hidden dock tab draws nothing, owns no renderer and is not `SetVisible`; only the shown agent tab reads its transcript (`dockTick`'s rule). Toast cards re-wrap their body only when the text, width, metric or theme changes. Link underlines stay text-only and allocation-free per frame (`TestVisibleWithDocPathsAllocatesNothing`); the disk is touched only on a click.
- The sidebar keeps zero allocations on unchanged frames: `TestSidebarCache*` keep passing in every lane.
- Colours come only from the theme (`Chrome, Active, Border, Muted, Accent, Needs, Done, ToastBG, ToastLine, ToastFG, ToastGo, ToastTitle`, plus `ActiveFG` and `memberColor("magenta")`) with local `render.Readable` nudges. Every new text is at least 4.5:1 on its surface in all five `theme.All()` themes.
- Copy fixed by the issues: dock buttons "Expand", "Open in reader", "Hide"; card "Open session", "Needs your permission", "Waiting for your answer", "Done · ran 6m 40s" form; miss note "Can't find <name> in <folder>".
- Lanes and file ownership: see "Lanes, shared files and merge order" below. Each lane works in its own worktree (`git worktree add ../kaze-<lane> -b feat/dock-alerts-links-<lane> feat/dock-alerts-links`) and merges back into `feat/dock-alerts-links`.
- Commits: `type(scope): Verb ...`, **no AI attribution lines of any kind** (CLAUDE.md overrides any default). Never push to `main`; the owner merges. Release 0.27 (version bump, changelog) is its own PR, not in this plan.

## Lanes, shared files and merge order

| Lane | Tasks | Owns |
|---|---|---|
| **L** links (#128) | L1 to L3 | `internal/render/links.go`, `internal/render/links_test.go`, `internal/ui/links.go`, `internal/ui/docs_open_test.go` |
| **A** alerts (#9) | A1 to A6 | `cmd/kaze/notify.go`, `cmd/kaze/notify_test.go`, `internal/ui/alerts.go`, `internal/ui/alerts_card.go` (new), `internal/ui/alerts_card_test.go` (new), `internal/ui/alerts_test.go`, `internal/ui/alerts_render_test.go` (new), `internal/ui/workspace_run.go` (run toast fields only), `internal/ui/tabbar.go` (`toastHits` type only) |
| **D** dock (#133) | D1 to D8 | `internal/ui/dock*.go` and tests, `internal/ui/panes*.go` and tests (deleted in D7), `internal/ui/scratch*.go` (deleted in D6), `internal/tabs/*`, `internal/config/*`, `internal/ui/commands.go`, `logic.go`, `titlebar*.go`, `settings_commands.go`, `sidebar.go`, `sidebar_cache.go`, `sidebar_model.go`, `status.go`, `palette_window.go`, `session_restore.go`, `resources.go`, `empty_window.go`, `subagents_ui.go`, and every test file that only exists for panes or scratch |
| Final | F1 | `README.md`, `docs/index.html`, `docs/HANDOFF.md`, `docs/audits/2026-11-04-dock-alerts-links.md` |

**Files two lanes touch** (each edit stays inside the named function, so git merges the hunks; the owner resolves any conflict):

| File | Lane L | Lane A | Lane D (owner of the rest) |
|---|---|---|---|
| `internal/ui/window.go` | `pointerView`'s Ctrl+click block only (the `linkUnder` / `openDocRef` lines) | none | everything else (`win.fl`, focus, `keyView`, `routeView`, frame order, `perform`, `captionEvent`) |
| `internal/ui/docs_open.go` | `openDocRef` (becomes `openDocRefs` plus a one-line wrapper) | none | `docParent` (drops `Parent` and `paneOwner`) |
| `internal/ui/alerts_ui.go` | none | deletes `drawToasts` (moved to `alerts_card.go`; `fitLabel` and `truncateRight` stay, other files use them), rewrites `toastEvent`, adds the turn start in `deliver` | `onNotify` only: drops `hookTab` and the `src` remap (D6) |
| `internal/ui/agent.go` | none | `agentState.since` field and its comment | `termAgents` doc comment (scratch wording, D6) |
| `internal/ui/sidebar_window.go` | none | `nextWaiting` body (A5) | `closeTab`, `afterClose`, `clickRow`, the draw key block (D5, D6) |

**Cross-lane contracts (unchanged signatures, nobody renames them):** `win.dockRect() image.Rectangle` (the whole docked panel as drawn last frame, tab row included; Lane A stacks cards above it), `win.toastGo(tab tabs.TabID)` (Lane A calls it for "Open session"; Lane D moves it to `dock_agent.go` in D7 and changes only its body), `win.sessionMeta(id) (title, repo, branch string)`, `win.agentOf(id) agentState`, `win.openDocRef(t, inPane bool, ref string)` (Lane L keeps it as a wrapper; `dock_input.go` keeps calling it), `formatShortcut(goos, sc)`.

**Merge order:** L first (smallest), then A, then D. Before D6, Lane D merges `feat/dock-alerts-links` (with L and A in) into its worktree, because D6 edits `alerts_ui.go`, `agent.go` and `docs_open.go` after the other lanes changed them. Then F1 runs on `feat/dock-alerts-links`.

## Review Focus

1. **Two keyboard models never run at once** (#133 risky spot). Between D4 and D7 both `win.fl.keys` and `win.dock.keys` exist: giving the dock member the keyboard clears `fl.keys` and the other way round, and after D7 only `dock.keys` is left. (D4 test `TestDockKeysAndPaneKeysExclusive`; D7 deletes it with `win.fl`.)
2. **A docked member's PTY size** (#133 risky spot): the member gets the panel body's grid, the session keeps the area above, a drag resizes neither until release, and closing the tab or Expand puts the member back on the shared size. (D3 tests `TestDockMemberGetsPanelGrid`, `TestDockMemberDragResizesOnRelease`, `TestDockMemberCloseDocksSize`.)
3. **Hidden tabs cost nothing:** a member tab that is not shown, or a hidden panel, has no renderer, is not visible to the manager and reads no transcript. (D3 `TestDockHiddenMemberDrawsNothing`, D2 `TestDockHiddenAgentReadsNothing`.)
4. **An old config with `scratch_shell`** loads with a warning, not an error, and binds nothing. (D6 `TestScratchShellKeyWarns`.)
5. **Card title is the sidebar row's**, not the shell title ("zsh needs you" bug). (A2 `TestCardTitleIsSidebarRow`.)
6. **Link underlines stay allocation-free** with spaces and quotes on the line, and a wrong long underline still opens the right file on click. (L1 `TestVisibleWithSpacedPathsAllocatesNothing`, L3 `TestClickFallsBackToShorterTail`.)

---

## Lane L: path links (#128), Tasks L1 to L3

### Task L1 (Lane L): Underline a path back to its root across single spaces, and quoted paths whole

**Files:**
- Modify: `internal/render/links.go` (`appendLinks`, new `extendBack`, `rootAt`, `quotedAt`)
- Test: `internal/render/links_test.go`

**Interfaces:**
- Produces (package-internal, allocation-free on `rs`):
  - `func rootAt(rs []rune, k int) bool`: `rs[k:]` starts with `X:\`, `X:/` (ASCII letter), `/`, `~/` or `~\`.
  - `func extendBack(rs []rune, i, floor int) int`: for a `.md` run starting at `i` that is not rooted, walks back across at most `maxGaps = 4` gaps, each exactly one space with a path rune on both sides (a drive's `:` counts inside `X:/`), never before `floor` (the end of the previous found link). It returns the start of the first rooted run it reaches whose last segment has no dot extension (a folder, not `x.go`), else `i`.
  - `func quotedAt(rs []rune, i int) (end int, ok bool)`: at a `"` or `'` in `rs[i]`, finds the same quote within 512 runes on the line; ok when the inside is non-empty, holds only path runes, spaces and a drive colon, and ends in `.md` by `pathAt`'s rule. The link is the inside, without the quotes.
  - `appendLinks`: tries `quotedAt` before `pathAt`; after `pathAt` finds a `.md` run, sets its start to `extendBack(rs, i, lastEnd)`. A failed quote just moves on one rune.
- `LinksIn` (the markdown reader's word splitter) is unchanged: it is fed single words, so nothing extends there.

- [ ] **Step 1: Write the failing tests** in `links_test.go`:
  - `TestFindSpacedDocPaths` (table, through `findLinks` like `TestFindDocPaths`):
    - `Write(C:/Users/john/Documents/Patrick Obsidian/Patrick/tickets/V3-3261/review-mine-2026-11-03.md)` → the whole path from `C:/`.
    - `/Users/pat/My Notes/Work Log/today.md` → whole (two gaps).
    - `see docs/a.md` → `docs/a.md` (no root behind it).
    - `Read /path/x.go and docs/README.md` → `docs/README.md` (the rooted run ends in an extension).
    - `C:\a\b  c\d.md` → `c\d.md` (two spaces stop it).
    - `edit "C:/My  Docs/x.md" now` → `C:/My  Docs/x.md` (quoted: taken whole, two spaces included).
    - `it's 'docs/a b.md'` → `docs/a b.md`; `don't open README.md` → `README.md`.
    - `/r/x y z w v u.md` → `u.md` (the root is 5 gaps back, past the 4-gap bound).
    - every case of `TestFindDocPaths` keeps its result.
  - `TestLinkAtExtendsAcrossWrappedRows`: a 30-column screen holding the Obsidian line wrapped over 3 rows → `At` on the last row returns the whole path.
  - `TestVisibleWithSpacedPathsAllocatesNothing`: 23 rows of the Obsidian line and the quoted line → `testing.AllocsPerRun(20, ...) == 0`.
- [ ] **Step 2: Run** `go test ./internal/render/ -run 'TestFind|TestLinkAt|TestVisible'` → FAIL (the Obsidian line gives `Obsidian/...`).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/render/` → PASS (`BenchmarkVisibleLinks240x80` within 10 % of before; note both numbers in the commit body).
- [ ] **Step 5: Commit** `fix(render): Keep a path link whole across single spaces and quotes`

### Task L2 (Lane L): Click-time candidates

**Files:**
- Modify: `internal/render/links.go` (new `RefsAt`, `refsFor`)
- Test: `internal/render/links_test.go`

**Interfaces:**
- Produces:
  - `func (l *Links) RefsAt(scr *screen.Screen, p Point) []string`: call with the session locked, only on a click (it allocates). Empty when no link is under `p`. For a web link: `[url]`. For a `.md` link: `refsFor(l.rs, f)`.
  - `func refsFor(rs []rune, f [2]int) []string`: (1) the link text; (2) each shorter tail that starts after one of the spaces inside the link, longest first; (3) for each rooted folder run earlier on the same logical line that ends in `\` or `/` (nearest first, at most 4), that folder joined with the link text when the link is not rooted; duplicates dropped.
  - `At` stays and returns `RefsAt(...)[0]` semantics (it keeps its own allocation-free path for tests and the cursor).
- [ ] **Step 1: Write the failing tests**:
  - `TestRefsForFolderEarlierOnLine`: `Full plan and design: C:\Users\me\.notes\tickets\T-1\ (plan.md, design.md, rulings.md)`, click on `design.md` → `["design.md", "C:\Users\me\.notes\tickets\T-1\design.md"]`.
  - `TestRefsForSpacedPathTails`: the Obsidian line → `[whole, "Obsidian/Patrick/tickets/V3-3261/review-mine-2026-11-03.md"]`.
  - `TestRefsForWebLink`: `see https://x.io/a.md` → `["https://x.io/a.md"]`.
  - `TestRefsAtNothingUnderPoint`: a blank cell → empty.
- [ ] **Step 2: Run** `go test ./internal/render/ -run TestRefs` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/render/` → PASS.
- [ ] **Step 5: Commit** `feat(render): List the paths a clicked .md link may mean`

### Task L3 (Lane L): Open the first candidate that exists, and say where kaze looked

**Files:**
- Modify: `internal/ui/links.go` (`linkUnder` returns `[]string`), `internal/ui/docs_open.go` (`openDocRefs`; `openDocRef` becomes `win.openDocRefs(t, inPane, []string{ref})`), `internal/ui/window.go` (`pointerView`'s Ctrl+click block: `refs[0]` decides web or doc)
- Test: `internal/ui/docs_open_test.go`

**Interfaces:**
- Produces:
  - `func (win *window) linkUnder(t *tabs.Tab, p image.Point, cw, ch int) ([]string, bool)` (locks the session for `RefsAt` only; no disk access under the lock).
  - `func (win *window) openDocRefs(t *tabs.Tab, inPane bool, refs []string)`: for each ref in order, `mdoc.Resolve(ref, roots)` (unchanged resolver: absolute refs are checked as they are, relative ones against `termFolder(t)` and its git top); the first hit opens. On a miss the note is `"Can't find " + filepath.Base(refs[0]) + " in " + where`, where `where` is the folder of the last absolute candidate tried, else `termFolder(t)`, shortened with `tildeHome` and passed through `cleanText`.
- [ ] **Step 1: Write the failing tests** (pattern of `TestShortcutClickOpensADocPath`, temp dirs only):
  - `TestClickOpensDocInFolderPrintedEarlier`: write `<tmp>\T-1\plan.md`; the shell prints `Full plan: <tmp>\T-1\ (plan.md, design.md)`; Ctrl+click `plan.md` → `shownDoc().path` is that file.
  - `TestClickOpensDocWithSpaceInPath`: write `<tmp>/Patrick Obsidian/Patrick/x.md`; print `Write(<tmp>/Patrick Obsidian/Patrick/x.md)` → opened.
  - `TestClickFallsBackToShorterTail`: `cd <tmp> then edit docs/notes.md` with only `<session dir>/docs/notes.md` on disk → opened (the long underline is wrong, the click still works; Ruling 21).
  - `TestDocPathFallsBackToTheRepoRoot`: the miss note becomes `"Can't find nope.md in " + tildeHome(dir)`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestClickOpens|TestClickFalls|TestDocPath|TestShortcutClick'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ ./internal/render/ ./internal/mdoc/` and `go vet ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `fix(ui): Open a .md path printed after its folder or with a space in it`

---

## Lane A: alert cards (#9), Tasks A1 to A6

### Task A1 (Lane A): A done alert carries the start of Claude's last reply

**Files:**
- Modify: `cmd/kaze/notify.go` (`runNotifyOutput`, new `replyStart`, `doneMaxRunes = 160`)
- Test: `cmd/kaze/notify_test.go`

**Interfaces:**
- Produces: `func replyStart(s string, n int) string`: drops markdown marks (`**`, `__`, backticks, a leading `#`, `>`, `-` or `*` and a space on any line), collapses every run of whitespace (newlines included) to one space, trims, cuts to `n` runes with `trimRunes`. In `runNotifyOutput`, for `kind == notify.KindDone` with an empty `hook.Message`, `msg.Reason = replyStart(hook.Last, doneMaxRunes)` (Claude and Codex both). `internal/notify` already passes a done reason through.
- `internal/ui/alerts.go`: `maxReason` 120 → 160 so the card, not the hook, does the cut.

- [ ] **Step 1: Write the failing tests**:
  - `TestNotifyDoneSendsReplyStart`: stdin `{"last_assistant_message":"## Done\n\nFixed **the link** split.\n- 214 tests pass"}` → `Reason == "Done Fixed the link split. 214 tests pass"`.
  - `TestNotifyDoneReplyCappedTo160Runes`: a 10 000-rune reply → `utf8.RuneCountInString(Reason) == 160`.
  - `TestNotifyDoneSends` (empty stdin) keeps `Reason == ""`.
  - `TestReplyStart` table, including an empty string and a reply of only marks.
  - `internal/ui`: `TestReasonIsCleanedAndCapped` now expects 160.
- [ ] **Step 2: Run** `go test ./cmd/kaze/ -run 'TestNotifyDone|TestReplyStart'` and `go test ./internal/ui/ -run TestReasonIsCleaned` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./cmd/kaze/ ./internal/notify/` → PASS.
- [ ] **Step 5: Commit** `feat(notify): Send the start of Claude's last reply with a done alert`

### Task A2 (Lane A): Card model

**Files:**
- Modify: `internal/ui/alerts.go` (`toast` gains `Ran time.Duration`, `Agent string`), `internal/ui/alerts_ui.go` (`deliver`: after `noteAgent`, a prompt sets `since`, a done sets the toast's `Ran`), `internal/ui/agent.go` (`agentState.since time.Time`: when the current turn's prompt arrived), `internal/ui/workspace_run.go` (the run toast sets `Agent: "workspace"`)
- Create: `internal/ui/alerts_card.go`, `internal/ui/alerts_card_test.go`

**Interfaces:**
- Produces:
  - `type alertCard struct { tab tabs.TabID; run uint64; kind notify.Kind; mark string /* "claude", "codex", "workspace", "" */; ctx, title, body, status string }`.
  - `func (win *window) cardOf(t toast) alertCard`: title = `win.sessionMeta(t.Tab)` title (the sidebar row rule) for a session, `t.Title` for a run; ctx = `"⎇ " + branch` and the command from `win.agentOf(t.Tab).command` joined by `" · "`, either part dropped when empty; a run's ctx is `"workspace"`; mark = `win.rowAgent(t.Tab).agentName()` for a session, `"workspace"` for a run; body = `t.Text`; status by Ruling 14.
  - `func cardStatus(kind notify.Kind, reason string, ran time.Duration, run bool) string`: needs → `"Needs your permission"` when `reason` contains "permission" or "approve" (case-insensitive), else `"Waiting for your answer"`; done → `"Done · ran " + ranText(ran)` (`"6m 40s"`, `"45s"`, `"1h 2m"`), plain `"Done"` when `ran == 0`; run → `"Run finished"`.
- [ ] **Step 1: Write the failing tests** in `alerts_card_test.go` (`liveWindow` fixtures):
  - `TestCardTitleIsSidebarRow`: a tab whose shell title is `zsh`, with a `/review-pr auth-tokens` prompt → `cardOf(...).title == "auth-tokens"` and `ctx` ends with `/review-pr`; never `"zsh"`.
  - `TestCardContextDropsMissingParts`: no branch → ctx is just the command; neither → `""`.
  - `TestCardStatus` (table: permission text, a question, done with 400 s, done with 0, run).
  - `TestDoneCardRanSincePrompt`: deliver a prompt at `t0`, a done at `t0+400s` → the toast's `Ran == 400*time.Second`.
  - `TestRunCardUsesWorkspaceMark`: `addRun(toast{Run: 7, Title: "web stack started", Agent: "workspace"})` → mark `"workspace"`, ctx `"workspace"`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestCard|TestDoneCard|TestRunCard'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Alert|Toast|Card|Notify|Deliver'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Name alert cards like their sidebar row and add their context`

### Task A3 (Lane A): Card colours and contrast

**Files:**
- Modify: `internal/ui/alerts_card.go` (`cardInk`)
- Test: `internal/ui/alerts_card_test.go`

**Interfaces:**
- Produces: `type cardInk struct { bg, line, tile, capBG, capLine color.NRGBA; title, body, muted, branch, cmd, go_, needs, done, badgeFG color.NRGBA }` from `func (c *chrome) cardInk() cardInk`: `bg = th.ToastBG`, `line = th.ToastLine`, `tile = mix(th.ToastBG, th.ToastLine, 0.6)` (the mark's square), `capBG = th.Active`, `capLine = th.ToastLine`; every text `render.Readable(x, bg)` with `title = ToastTitle`, `body = ToastFG`, `muted = Muted`, `branch = Done`, `cmd = go_ = ToastGo`, `needs = Needs`, `done = Done`; `badgeFG = render.Readable(th.ToastBG, needs)` (also checked on `done`). Cached per theme like `boardInk`.
- [ ] **Step 1: Write the failing test** `TestCardContrast`: for each `theme.All()`, every text field on `bg` ≥ 4.5, the key-cap text (`body`) on `capBG` ≥ 4.5, `badgeFG` on `needs` and on `done` ≥ 4.5.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestCardContrast` → FAIL (undefined).
- [ ] **Step 3: Implement.** If the badge glyph cannot reach 4.5 on a theme's Done, use `render.Readable(th.ActiveFG, done)` and recheck.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(ui): Add alert card colours with contrast checks`

### Task A4 (Lane A): Draw the cards bottom right

**Files:**
- Modify: `internal/ui/alerts_card.go` (`drawToasts` moves here and is rewritten), `internal/ui/alerts_ui.go` (delete the old `drawToasts`; `toastHit` loses `goRect` and `text`, keeps `id`, `run`, `rect`, `closeRect`; `toastEvent`: a press on `closeRect` dismisses, anywhere else on `rect` calls `toastGo`), `internal/ui/alerts.go` (`toast` gains the wrap cache `wrap cardWrap`)
- Test: `internal/ui/alerts_test.go` (rewrite the layout tests), `internal/ui/alerts_card_test.go`

**Interfaces:**
- Consumes: `cardOf`, `cardInk`, `agentLogo`, `agentLogoColor`, `c.icon(gtx, "workspaces", ...)`, `c.sans`, `c.fitSideText`, `formatShortcut`, `win.binds["next_waiting"]`, `win.dockRect()`.
- Produces:
  - `const cardWidthDp = 380`.
  - `type cardWrap struct { src string; width int; metric unit.Metric; th *theme.Theme; lines [2]string }` and `func (c *chrome) wrapTwo(gtx layout.Context, s string, size unit.Sp, room int, w *cardWrap) [2]string`: line 1 = the longest word-boundary prefix that fits `room` (at most 32 measures, binary search over word ends), line 2 = the rest cut with "…" by `fitSideText`; reused while `src`, `width`, `metric` and `th` are unchanged.
  - `drawToasts(gtx, area)`: card width `min(cardWidthDp, area.Dx() - 2*12dp)`; none drawn when that is under 260 dp. Bottom = `area.Max.Y - 12dp`, or `win.dockRect().Min.Y - 8dp` when the panel is drawn and higher. Newest card nearest the bottom (the stack grows up), 8 dp apart; the oldest are left out when the room runs out. Card (mockup `.card`): padding 12/14/11/12 dp, radius 10 dp, 1 px `line` over `bg`. Mark: 30 dp tile, radius 8 dp, `tile` fill, logo 18 dp (`agentLogo`) or the workspaces icon; badge 15 dp circle at its bottom right (2 dp `bg` ring), `!` on `needs` or `✓` on `done` in `badgeFG`, 9 sp ExtraBold. Text column from 34+11 dp: ctx 11 sp mono (`branch` for the `⎇` part, `cmd` for the command, `muted` for `·`), cut to fit, leaving 20 dp for ×; title 13.5 sp Bold `title`, one line with "…"; body 12.5 sp `body`, up to 2 lines from `wrapTwo`, omitted when empty; status 12 sp `muted` after a 6 dp dot in `needs`/`done`; footer after a 1 px `line` and 7 dp: "Open session" 12 sp SemiBold `go_`, and, on the newest card only, the shortcut's parts as key caps (10.5 sp mono, `capBG`, 1 px `capLine`, radius 4 dp, 5 dp side padding, 6 dp apart). × 14 sp `muted` in a 20 dp box at 7/8 dp from the top right.
- [ ] **Step 1: Write the failing tests**:
  - `TestCardsSitBottomRightNewestLowest` (replaces `TestToastsSitAboveStatusBarBottomRight`): 3 needs cards in a 1000×600 area → 3 hits, each inside the area, `Dx() == Dp(380)`, right edge at `area.Max.X - Dp(12)`, no overlap, the newest has the largest `Min.Y`.
  - `TestCardsStackAboveDock`: `win.dock.lastRect` set to the bottom 200 px → every card's `Max.Y <= lastRect.Min.Y - Dp(8)`.
  - `TestShortWindowDropsOldestCards` (rewrite of `TestShortWindowDropsCardsAboveTheBar`): a 300 px tall area → the newest card drawn, the oldest left out.
  - `TestNarrowAreaDrawsNoCards` (replaces `TestNarrowWindowKeepsTextOffTheButtons`): a 250 px wide area → no hits.
  - `TestCardsStayRightOfTheSidebar`: kept, retargeted.
  - `TestCardBodyWrapsOnceAndCuts`: a 160-rune body → 2 lines, the second ends in "…"; 10 more frames → `wrapTwo` measures 0 more times (count in `cardWrap`).
  - `TestCardKeyCapsOnNewestOnly`: texts drawn contain "Ctrl", "Shift", "J" once (Windows binding), on the newest card's rect; with `next_waiting` unbound, none.
  - `TestCardClickOpensSession` (replaces `TestGoSwitchesTab`): `toastEvent` press in the card's body → `mgr.Active().ID == tab`; a press on `closeRect` → dismissed, mark kept (`TestToastCloseDismisses` kept).
  - `TestDoneCardShowsReply`: a done alert with a reason → the texts include the body's first words and `"Done"`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestCard|TestShortWindow|TestNarrowArea|TestToast'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (`TestSidebarCache*` included).
- [ ] **Step 5: Commit** `feat(ui): Show alerts as cards with context, reply and status`

### Task A5 (Lane A): Ctrl+Shift+J / Cmd+J opens the newest card's session

**Files:**
- Modify: `internal/ui/sidebar_window.go` (`nextWaiting` body only)
- Test: `internal/ui/alerts_card_test.go`

**Interfaces:**
- Produces: `nextWaiting()`: when `win.alerts.toasts` is not empty, `win.toastGo(newest.Tab)` and `win.side.reveal = true`, then return; otherwise today's walk over `markNeeds`. The palette's next-waiting action calls the same function.
- [ ] **Step 1: Write the failing tests**:
  - `TestNextWaitingOpensNewestCard`: a needs card on tab 2, then a done card on tab 3 → `nextWaiting()` → active is 3, its card gone (visited on the next frame).
  - `TestNextWaitingWithoutCardsWalksMarks`: no cards, a needs mark on tab 2 → active 2 (today's behaviour).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestNextWaiting` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Jump to the newest alert card with the next-waiting key`

### Task A6 (Lane A): Card captures

**Files:**
- Create: `internal/ui/alerts_render_test.go`, `docs/audits/dock-alerts-links/cards-*.png`

- [ ] **Step 1:** `TestCardRenders`, gated by `KAZE_CARD_RENDER_DIR` (pattern of `TestDockRenders`: `headless.NewWindow`, `writeRender`), 1400×860, themes Midnight, Sumi and Washi: `stack` (a needs card with permission text, a done card with a reply and ran time, a run card), `dock` (two cards above an open panel), `narrow` (900 px window). Without the env var it is skipped.
- [ ] **Step 2:** Run it with the env var, open every PNG, compare with `docs/design/alert-card-mockup.html`, fix drift test-first.
- [ ] **Step 3: Commit** `test(ui): Capture the alert cards in three themes`

---

## Lane D: dock tabs (#133), Tasks D1 to D8, in the issue's safe order

### Task D1 (Lane D): A deck of tabs per session (agents only)

**Files:**
- Modify: `internal/ui/dock.go`
- Test: `internal/ui/dock_test.go`, new `internal/ui/dock_deck_test.go`

**Interfaces:**
- `dockSession` keeps its name and fields: it is one **agent view** (the board preview's `b.prev` keeps using it).
- Produces:
  - `const dockMaxTabs = 6`.
  - `type dockEntry struct { member tabs.TabID /* 0 for an agent */; view *dockSession /* agent tabs only */; opened uint64 }`.
  - `type dockDeck struct { entries []dockEntry; on int /* index shown, -1 none */; hidden bool }`.
  - `dockState.by map[tabs.TabID]*dockDeck`; `dockState.seq uint64`.
  - `func (win *window) deckOf(id tabs.TabID) *dockDeck`.
  - `func (win *window) dockOf(id tabs.TabID) *dockSession`: **meaning kept**: the agent view the panel shows now; nil when the deck is missing, hidden, or shows a member (Ruling 25).
  - `openDock(session, k)`: k's tab exists → shown; else appended (Ruling 2 cap: past 6, drop the oldest `opened` entry that is not shown); `hidden = false`.
  - `toggleDock(session, k)`: k is shown and not hidden → `hidden = true` (tabs kept, Ruling 4); else `openDock`.
  - `func (win *window) closeDockTab(session tabs.TabID, i int)`: removes entry i; the shown index moves to the left neighbour, else the right; the last one removes the deck. `closeDock(session)` hides the panel (keeps tabs) and drops panel keys.
  - `dropGoneAgents`: removes only the gone agents' entries.
- [ ] **Step 1: Write the failing tests** in `dock_deck_test.go`:
  - `TestDeckOpensTabsAndSwitches`: open k1, k2 → 2 entries, `on == 1`, `dockOf(a).agent == k2`; open k1 → `on == 0`, still 2.
  - `TestDeckToggleHidesKeepsTabs`: toggle the shown k2 → `dockOf(a) == nil`, `deckOf(a).hidden`, 2 entries; open k1 → shown again.
  - `TestDeckCapReplacesOldestNotShown`: 7 agents opened in order, the 1st re-shown before the 7th → 6 entries, the 2nd is the one gone.
  - `TestDeckCloseTabMovesLeft`: 3 tabs, close the middle shown one → the left one shows; close all → `deckOf(a) == nil`.
  - `TestDeckDropsGoneAgentOnly`: dismiss k1 from the tracker → k2's tab remains.
  - Existing `TestDockToggle`, `TestDockPerSession`, `TestDockPrunedWithSession`, `TestDockForgetsGoneAgent` and every `TestDock*` / `TestBoard*` keep passing unchanged.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDeck'` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Dock|Board|Sidebar|Toast'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Keep a deck of tabs in each session's docked panel`

### Task D2 (Lane D): Tab row, agent sub-header, Hide

**Files:**
- Modify: `internal/ui/dock_draw.go` (`drawDock` split into `drawDockTabs` and `drawDockAgent`; `drawShownDock`), `internal/ui/dock_input.go` (`dockHead`, tab hover), `internal/ui/dock.go` (`dockHits` gains tab hits)
- Test: `internal/ui/dock_draw_test.go`, `internal/ui/dock_input_test.go`, `internal/ui/dock_render_test.go`

**Interfaces:**
- Produces:
  - Sizes: `dockTabsDp = 32` (row), `dockSubDp = 26` (agent sub-header), `dockTabMinDp = 72`, `dockTabMaxDp = 200` (Ruling 1).
  - `type dockTabHit struct { i int; rect, close image.Rectangle }`; `dockHits` gains `tabs []dockTabHit; expand, hide image.Rectangle`; `dockState` gains `hover int` (-1 none) and `redraws int`.
  - Row (mockup `.tabs`, `.tab`, `.tbtn`): `th.Chrome` fill, 1 px `th.Border` under it. Each tab 12 dp left pad, 8 dp right pad, 7 dp gap, radius 7 dp on top, 4 dp from the top: a member shows its state dot (7 dp, `memberDot`) and name; an agent its type (11 sp mono `kind`) and task (12 sp). The shown tab: `win.pal.BG` fill, 2 dp `th.Accent` line on top, `text` colour; others `muted`. × (16 dp box) on the shown tab and on the hovered tab only. Widths: content width clamped to [72, 200] dp; when the row overflows, all shrink equally to 72 dp, then names are cut with "…" (`fitSide`). Right side, right to left: "Hide ⌄" always; "Open in reader" for a finished agent; "Expand" for a member (D3 draws members).
  - Agent sub-header (for the shown agent tab): today's header contents without × and the reader button: state dot, type, task, elapsed (or "waiting for you"), the Activity/Report switch, and "Copy report" when done. Prompt row and body unchanged below it.
  - Input: `dockHead` maps `tabs[i].close` → `closeDockTab`, `tabs[i].rect` → show tab i (`hidden = false`), `hide` → `closeDock` (hide), `reader`/`copy` as today. Pointer `Enter|Move|Leave` on the row set `hover` and invalidate only when it changed (`redraws++`).
  - Esc with panel keys on an agent tab hides the panel (Ruling 4); ↑/↓ still step agents (`dockStep` opens the neighbour as a tab).
- [ ] **Step 1: Write the failing tests**:
  - `TestDockDrawsTabRow`: two agents open → `len(win.dock.hits.tabs) == 2`, left to right, each `Dx()` in [Dp(72), Dp(200)]; texts contain both types and "Hide ⌄".
  - `TestDockTabsShrinkThenCut`: 6 agents with 80-rune tasks in a 700 px panel → every tab ≥ Dp(72), the last ends before `hide.Min.X`.
  - `TestDockTabClickSwitches` and `TestDockTabCloseKeepsOthers` through `dockHead`.
  - `TestDockHideKeepsTabs`: press `hide` → `dockRect().Empty()` next frame, deck kept; reopen by `showAgent(..., false)` → drawn.
  - `TestDockHoverRedrawsOnlyOnChange`: two Moves over tab 0 → `redraws == 1`; Move to tab 1 → 2.
  - `TestDockHiddenAgentReadsNothing`: hide the panel, 5 frames over 3 s of fake time → the agent view's `trAt` unchanged.
  - Update `TestDockTabsAndAutoReport`, `TestDockEscClosesWhenFocused` (now hides), `TestDockRenders` (adds a two-tab state).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDock'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Show docked agents as tabs with a Hide button`

### Task D3 (Lane D): Member tabs: live terminal at the panel's size

**Files:**
- Modify: `internal/ui/dock.go` (`openDockMember`, `memberDeck`, `dockFit` sizes the member, `pruneDock` docks members back), `internal/ui/dock_draw.go` (`drawDockMember`), new fields in `dockState`: `rend *render.Renderer; drawn tabs.TabID; term int /* the member view's event tag */`
- Test: new `internal/ui/dock_member_test.go`

**Interfaces:**
- Produces:
  - `func (win *window) openDockMember(session, m tabs.TabID)`: removes m from any other deck first (one deck at a time, `memberDeck(m) tabs.TabID`), then adds or shows it like `openDock`. Not yet reachable from the UI (D5).
  - `func (win *window) shownMember() tabs.TabID`: the dock owner's shown member, 0 when hidden or an agent shows.
  - `dockFit`: when a member shows and the panel is not empty and not being dragged, `win.mgr.ResizeTab(m, cols, rows)` for the body grid (panel minus handle and tab row, `pad` like the terminal); the member keeps that size while its tab is open (Ruling 6). Closing its tab, Expand, or pruning calls `win.mgr.Dock(m)`.
  - `drawDockMember(gtx, m, body)`: one renderer `win.dock.rend` (made on first draw with `render.NewRenderer(win.rendPx, win.pal, rendOptions(win.opt.Config))`, dropped with `rend = nil` when no member is drawn, and reset where `window.go` resets `win.fl.rends` today); `SetVisible(m, true)` for the drawn member and `false` for the previous one (`dock.drawn`); `linkCursors`; `event.Op(ops, &win.dock.term)` over the body. Member tab extras: "Expand" button, and the tab's dot from the `curRuns()` member status, coloured as the sidebar's member line (`sideMember.Down`).
  - Exited member (Ruling 3): the tab stays, the dot shows down, the last output stays, no restart prompt.
- [ ] **Step 1: Write the failing tests** (`paneWindow` fixture: sessions a, b, members m1, m2):
  - `TestDockMemberGetsPanelGrid`: `openDockMember(a, m1)`, frame → m1's PTY size equals `gridSize` of the panel body; a's PTY equals the area above.
  - `TestDockMemberDragResizesOnRelease`: drag the handle 60 px up → no new member size until Release, then one.
  - `TestDockMemberCloseDocksSize`: close m1's tab → m1 back on `mgr.Size()`; not visible.
  - `TestDockHiddenMemberDrawsNothing`: open m1 then an agent tab → `win.dock.rend == nil` after the frame (no member drawn), m1 not visible; hide the panel with m1 shown → same.
  - `TestDockMemberMovesBetweenSessions`: open m1 in a, then in b → only b's deck holds it.
  - `TestDockExitedMemberKeepsTab`: m1's shell exits → its tab remains, the dot reads down.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestDockMember` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Dock a workspace member's terminal as a panel tab`

### Task D4 (Lane D): The member tab takes the keyboard

**Files:**
- Modify: `internal/ui/dock_input.go` (`dockEvents`: when a member shows, its body events go through `termEvents` with `dockMemberView()`), `internal/ui/window.go` (`keyView`, `termEvents` press branch, `routeView`'s keyboard-moved check, `switchedTo`, `viewFocused`, `visit` of the typed-into view, `autoScrollSelection` caller), `internal/ui/dock.go` (`dockFocusMember(on bool)` with the focus-in/out reports, `dock.refocus`)
- Test: `internal/ui/dock_member_test.go`, `internal/ui/dock_review_test.go`

**Interfaces:**
- Produces:
  - `func (win *window) dockMemberView() (termView, bool)`: `termView{tab: m, tag: &win.dock.term, grid, cols, rows}` for the drawn member.
  - `keyView()`: `dock.keys && shownMember() != 0` → the member view; else the panes branch (until D7); else the main view.
  - `dockFocusMember(true)`: sets `dock.keys`, sends focus-out to the session and focus-in to the member (only while `win.focused`), clears `win.fl.keys` via `focusSession` first; `dropDockKeys` reverses it. A press in the member body takes the keyboard; a press in the main terminal gives it back (today's `dropDockKeys` call); opening a member gives it the keyboard (Ruling 5), opening an agent does not.
  - Focus line (Ruling 11): while a member tab shows, a 2 dp `th.Accent` line on top of whichever has the keyboard (member body or the terminal area).
  - Kaze shortcuts still run from the member (`onKey` sees them first, as from a pane today).
- [ ] **Step 1: Write the failing tests**:
  - `TestDockMemberClickTakesKeys`: click the member body → `focusTarget() == &win.dock.term`; `typeKeys("ls\r")` reaches m1's PTY, not a's.
  - `TestDockTerminalClickTakesKeysBackFromMember`: then click the terminal → typing reaches a.
  - `TestDockMemberShortcutsWork`: with the member focused, the palette shortcut opens the palette.
  - `TestDockMemberFocusReports`: with focus reporting on in both shells, the member gets `ESC[I` and the session `ESC[O`.
  - `TestDockKeysAndPaneKeysExclusive`: float m2 (old path) and focus it, then focus the dock member → `win.fl.keys == 0`; focus the pane again → `!win.dock.keys`.
  - `TestDockMemberKeysLeaveOnTabSwitch` and `TestShiftEnterReachesDockMember` (port of `TestShiftEnterReachesFocusedTerminal`).
  - `TestDockMemberSelectionAutoScrolls` (port of `TestPaneSelectionAutoScrollsOnlyItsOwnHistory`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDockMember|TestDockKeys|TestShiftEnterReachesDock'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS; `go test -race ./internal/ui/ -run 'TestDockMember'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Type into a docked member and take the keyboard back with a click`

### Task D5 (Lane D): Every member entry point docks a tab

**Files:**
- Modify: `internal/ui/dock.go` (new `dockMember(id tabs.TabID, toggle bool)`, `expandMember` moves here), `internal/ui/sidebar_window.go` (`clickRow` → `dockMember(id, true)`; the draw key's `floating` becomes `docked = win.shownMember()`), `internal/ui/sidebar.go` / `sidebar_cache.go` (`floating` → `docked`), `internal/ui/status.go` (server dots → `dockMember(down, false)`), `internal/ui/palette_window.go` (`dockMember(it.Tab, false)`), `internal/ui/panes_agent.go` (`showAgent` drops the `fl` branch), `internal/ui/panes_input.go` (delete `floatMember`)
- Test: `internal/ui/dock_member_test.go` (ports of `panes_member_test.go`), delete `internal/ui/panes_member_test.go`

**Interfaces:**
- Produces: `dockMember(id, toggle)`: with no session in front (`dockOwner()` is 0 or a member) → `selectTab(id)` (Ruling 8); the member already shown in front → nothing; shown in this session's visible panel and `toggle` → hide; else `openDockMember(owner, id)` and `dockFocusMember(true)`. `expandMember(id)`: drop its entry, `mgr.Dock(id)`, `selectTab(id)`, `side.reveal`.
- [ ] **Step 1: Write the failing tests** (ports, same fixture):
  - `TestMemberClickDocksATab`, `TestMemberClickAgainHidesPanel`, `TestMemberClickWhenItIsTheMainViewDoesNothing`, `TestMemberClickWhileAMemberIsShownShowsIt`, `TestExpandMakesTheMemberTheMainView`, `TestPickingASessionAfterExpandGoesBack`, `TestCloseTabKeepsMemberRunning`, `TestOpeningAMemberInAnotherSessionMovesIt`, `TestSeventhTabReplacesOldest`, `TestServerDotsDockTheDownMember`, `TestDockedMemberIsHighlighted` (sidebar `docked`), `TestPaletteDocksAMember`, `TestTypingAfterDockingAMemberReachesIt`.
  - `TestSidebarCache*` stay at 0 allocations with a docked member.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestMember|TestExpand|TestPicking|TestCloseTabKeeps|TestOpeningAMember|TestSeventh|TestServerDots|TestDockedMember|TestPaletteDocks|TestTypingAfterDocking|TestSidebarCache'` → FAIL.
- [ ] **Step 3: Implement.** `panes.go` keeps compiling: only the scratch shell still opens a pane.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Open workspace members as dock tabs from the sidebar, dots and palette`

### Task D6 (Lane D): Remove the scratch shell and Tab.Parent

**Before this task:** merge `feat/dock-alerts-links` (Lanes L and A merged) into the Lane D worktree and run `go test ./...`.

**Files:**
- Delete: `internal/ui/scratch.go`, `internal/ui/scratch_test.go`, `internal/ui/scratch_input_test.go`
- Modify: `internal/tabs/manager.go` (drop `Tab.Parent`, `StartOpts.Parent`, the `o.Parent == 0` active rule), `internal/tabs/state.go` (no Parent skip), `internal/tabs/float_test.go` → rename to `internal/tabs/sized_test.go` without `TestParentTabIsNotMadeActive`; `internal/config/config.go` (`KnownActions` loses `scratch_shell`; `loadKeys` warns `scratch_shell under [keys] is no longer used: the scratch shell was removed; ignored`), `internal/ui/logic.go` (`actScratch`, its bindings), `internal/ui/commands.go`, `internal/ui/settings_commands.go`, `internal/ui/titlebar.go` and `titlebar_draw.go` (`capScratch`, `Scratch` rect, the Windows button and the macOS sidebar/rail button), `internal/ui/window.go` (`case capScratch`, `case actScratch`, the scratch comments in `keyView` and `routeView`), `internal/ui/alerts_ui.go` (`onNotify`: no `hookTab`, `src := m.Tab` stays as the event source), `internal/ui/agent.go` (`termAgents` comment), `internal/ui/docs_open.go` (`docParent`: no `Parent`; `inPane` means the docked member, owner `dockOwner()`), `internal/ui/dock.go` (`dockOwner` without `Parent`), `internal/ui/empty_window.go`, `internal/ui/resources.go`, `internal/ui/sidebar_cache.go` (`sideTabInput.parent` goes), `internal/ui/sidebar_model.go`, `internal/ui/sidebar_window.go` (`closeTab` without `closeScratch`; it closes the session's dock deck and docks its members back; `afterClose`), `internal/ui/session_restore.go`, `internal/ui/panes*.go` (scratch kind, `scratchDir`, `paneScratch`)
- Test: `internal/config/config_test.go`, `internal/ui/logic_test.go`, `internal/ui/settings_commands_test.go`, `internal/ui/titlebar_draw_test.go`, `internal/ui/titlelayout_test.go`, `internal/ui/session_keys_test.go`, `internal/ui/links_test.go`, `internal/ui/agents_codex_test.go`, `internal/ui/session_restore_test.go`, `internal/ui/resources_test.go`, `internal/ui/subagents_ui_test.go`, `internal/ui/worktree_cleanup_test.go`, `internal/ui/panes_test.go` (scratch cases)

**Interfaces:**
- `subagents.Event.Src` stays; `onNotify` passes the tab the hook ran in, which now always equals `m.Tab` (Ruling 10).
- [ ] **Step 1: Write the failing tests**:
  - `TestScratchShellKeyWarns` (config): `[keys]\nscratch_shell = "ctrl+shift+\`"` → `err == nil`, one warning containing `scratch_shell`, `Keys` without it; `TestKnownActions` list without it.
  - `TestNoScratchAction` (ui): `actionByName["scratch_shell"]` missing; `defaultBindings` for both OSes have no backtick binding; the title bar layout has no scratch rect (`titleLayout` field gone: compile-level RED) and Ctrl+Shift+\` reaches the shell as input.
  - `TestClosingSessionDocksItsMembersBack`: a docked m1 in a, close a → m1 on the shared size, not visible, still open.
  - Every test that built a scratch tab is deleted or rewritten without `Parent`.
- [ ] **Step 2: Run** `go test ./internal/config/ ./internal/tabs/ ./internal/ui/` → FAIL (compile errors on `Parent`, `actScratch`, then the new assertions).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...` and `go vet ./...` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Remove the scratch shell`

### Task D7 (Lane D): Delete the floating panes

**Files:**
- Delete: `internal/ui/panes.go`, `internal/ui/panes_draw.go`, `internal/ui/panes_input.go`, `internal/ui/panes_test.go`, `internal/ui/panes_draw_test.go`, `internal/ui/panes_input_test.go`, `internal/ui/panes_autoscroll_test.go`
- Move: `internal/ui/panes_agent.go` → `internal/ui/dock_agent.go` (`git mv`, contents unchanged apart from removed pane mentions); `autoScrollSelection` → `internal/ui/dock_input.go`; `keyTab` → `window.go` (it now returns the dock member view's tab, else the shown tab).
- Create: `internal/ui/testkit_test.go` with the shared helpers moved verbatim from the deleted test files: `paneWindow` (renamed `memberWindow`; update callers), `floatFrames` (renamed `twoFrames`), `ptyOf`, `at`, `clickAt`, `dragFrom`, `typeKeys`, `emitTo`, `clickMember`, `historyOffset`.
- Modify: `internal/ui/window.go` (`fl floats` field, `win.fl.rends = nil` → `win.dock.rend = nil`, `paneEvents`, `paneStep` (its "the shown tab is never also docked" rule moves to `pruneDock`), `paneViews`, `drawPanes`, `syncVisible`, `paneKeyView`, `win.fl.refocus` → `win.dock.refocus`, `viewFocused`), every test referring to `win.fl`.

**Interfaces:**
- After this task `grep -rn "win\.fl\b\|paneKind\|floats\|paneOwner\|floatMember" internal/` is empty.
- [ ] **Step 1: RED by deletion:** delete `panes.go` first and run `go build ./internal/ui/` → FAIL listing every caller; that list is the task's checklist.
- [ ] **Step 2: Implement** the moves and replacements; `TestDockKeysAndPaneKeysExclusive` is deleted with `win.fl` (Review Focus 1 is now structural).
- [ ] **Step 3: Run** `go test ./...`, `go vet ./...`, `go test -race ./internal/ui/ -run 'Dock|Member|Sidebar'` → PASS; the grep above is empty; `GOOS=darwin go vet ./internal/ui/` where cgo allows.
- [ ] **Step 4: Commit** `refactor(ui): Delete the floating panes`

### Task D8 (Lane D): Dock captures and frame check

**Files:**
- Modify: `internal/ui/dock_render_test.go`
- Create: `docs/audits/dock-alerts-links/dock-*.png`

- [ ] **Step 1:** Extend `TestDockRenders` (gated by its env var): Sumi and Midnight, 1400×860: `member` (api member shown with the keyboard, an agent tab beside it), `agent` (Explore running, Activity), `six` (6 tabs, cut names), `exited` (worker down), `hidden` (panel hidden, tabs kept: plain terminal). Compare with `docs/design/dock-tabs-mockup.html`; fix drift test-first.
- [ ] **Step 2:** `TestDockFrameTiming` gains a member case; log ns per frame and allocations for: no panel, agent tab, member tab. A member tab frame must allocate no more than a floating pane frame did on `main` (measure `main` in a `git worktree add ../kaze-before main` copy with the same test).
- [ ] **Step 3: Commit** `test(ui): Capture the dock tabs and time a member tab frame`

---

### Task F1 (after all lanes): Merge check, audit, README, lander, HANDOFF

**Files:**
- Create: `docs/audits/2026-11-04-dock-alerts-links.md`
- Modify: `README.md`, `docs/index.html`, `docs/HANDOFF.md`

- [ ] **Step 1: Full check** on `feat/dock-alerts-links` after the three merges: `go test ./...`, `go vet ./...`, `go test -race ./internal/ui/ ./internal/render/`, Windows build `go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze` (in a temp folder, never over the owner's copy), `GOOS=darwin go vet ./...` where cgo allows.
- [ ] **Step 2: README:** replace "## Floating panes" and "### The scratch shell" with "## Workspace members in the panel" (open as tabs, one panel per session, × keeps the member running, Hide, Expand, 6 tabs, one session at a time, click to type, the accent line, status bar dots); shortcut table: delete the scratch shell row; the sentence on session switching ("skips workspace members and scratch shells ... floating terminal") and the `[keys]` list (no `scratch_shell`; note an old line is ignored with a warning); The sidebar, the title bar ("on the right the scratch shell"), The status bar ("Scratch shells count under ...", "moving a floating"), Workspaces ("float it over your session", "float the open member"), Session restoration ("scratch shells, floating layout"); Subagents ("Go" on the toast → "Open session" on the card; panel tabs); "## Alerts (Claude Code hooks)": cards at the bottom right, what each line shows, the done reply, Ctrl+Shift+J / Cmd+J opens the newest card; "## Reading docs": a path after its folder, a space in the path, quoted paths, the miss note; Roadmap: Plan 6d line notes the 0.27 replacement, add the three items.
- [ ] **Step 3: Lander `docs/index.html`:** hero `alt` and `#capture-description` no longer mention a floating server log (say the panel under the terminal); remove the "Scratch shell" `data-shot` button and its gallery entry; replace the "Floating panes." card with "Servers in the panel." (members as tabs under Claude's terminal) and delete "A scratch shell, per session."; the Workspaces example footnote ("Click one to float it over your session" → open it as a tab under your session); the "Jump to what's waiting." card mentions the alert card; the two installer tips drop "Scratch shell" (use "Next waiting: Ctrl Shift J" / "⌘ J"). Do not touch "What's new" (the release PR does). Record in `docs/site-assets/SCREENSHOTS.md` that the sessions shot (floating pane) and the scratch shot must be retaken or removed; do not fake them.
- [ ] **Step 4: HANDOFF:** a dated "Where things stand" entry at the top (built on `feat/dock-alerts-links`, release 0.27 next, screenshots to retake); in "Implementation history", a one-line note under the Plan 6d entry that 0.27 replaced floating panes and the scratch shell with dock tabs; Known gaps: OSC 8 still not read (#128 item 3), the underline false positive of Ruling 21.
- [ ] **Step 5: Audit** `docs/audits/2026-11-04-dock-alerts-links.md`: per lane what changed, the tests per task, the dock frame numbers (D8), the link benchmark before and after (L1), the contrast minimum per theme (A3, D2), the captures (A6, D8) labelled as development captures, and the rulings below with any the owner changed.
- [ ] **Step 6: Commit** `docs: Record the dock tabs, alert cards and path links`

## Rulings made while planning

Each is cheap to overturn in its own task.

1. **Tab widths:** content width clamped to 72 to 200 dp; on overflow all tabs shrink equally to 72 dp, then names are cut with "…". × shows on the shown tab and the hovered one; hover redraws only when the hovered tab changes. (D2)
2. **6-tab cap** per session, members and agents together; a seventh replaces the oldest-opened tab that is not showing. (D1)
3. **An exited member keeps its tab**, with the down dot and its last output; no "press Enter to restart" and no restart button, because Kaze has no member restart (rerun the workspace from the sidebar). (D3)
4. **Hiding is not closing:** a second click on the shown agent or member, "Hide", and Esc on an agent tab with the panel's keyboard all hide the panel and keep its tabs; × closes one tab. (D1, D2)
5. **Opening a member gives it the keyboard** (as floating did); opening an agent does not. (D4, D5)
6. **A member keeps the panel's PTY size while its tab is open**, even hidden, so hiding does not reflow its output; it returns to the shared size when its tab closes, on Expand, or when its session closes. Only a drawn member has a renderer and is `SetVisible`. (D3)
7. **Agent tabs keep a 26 dp sub-header** (state, type, task, elapsed, Activity/Report, Copy report); the tab row's right side carries "Open in reader" (finished agent), "Expand" (member) and "Hide ⌄", as the mockup. (D2)
8. **No session in front** (a member as the main view, Settings, the board, a document): a member click shows it as the main view, as `floatMember` did. (D5)
9. **Old `scratch_shell` key:** loads with the warning "scratch_shell under [keys] is no longer used: the scratch shell was removed; ignored"; the line stays in the file. (D6)
10. **`subagents.Event.Src` stays** (it now always equals `Tab`); removing it would churn `internal/subagents` for nothing users see. (D6)
11. **Focus line:** while a member tab shows, a 2 dp accent line sits on top of whichever terminal has the keyboard; with no member tab nothing changes. (D4)
12. **Cards:** 380 dp wide (less in a narrow window, none under 260 dp), at most 3, one per session, newest nearest the bottom right corner, 12 dp from the edges, 8 dp above the dock panel when it shows; the oldest are left out when the room runs out. (A4)
13. **Done body:** the first 160 runes of `last_assistant_message`, whitespace collapsed, markdown marks dropped, for Claude and Codex; no reply → no body line. (A1)
14. **Status line:** a needs reason that mentions "permission" or "approve" → "Needs your permission", any other → "Waiting for your answer"; done → "Done · ran 6m 40s" from the prompt that started the turn, plain "Done" when unknown; a run → "Run finished". (A2)
15. **Context line:** `⎇ <branch>` (full branch) and the session's slash command; a workspace run says "workspace". (A2)
16. **Key caps** come from the configured `next_waiting` binding (split on "+"), on the newest card only; none when unbound. (A4)
17. **A click anywhere on a card** (except ×) opens the session, like "Open session". (A4)
18. **Ctrl+Shift+J / Cmd+J** with cards showing opens the newest card's session (a run card: its tab); with none, it walks the marks as today. The palette action shares it. (A5)
19. **The card body is wrapped once** per text, width, metric and theme, not every frame. (A4)
20. **Underline rule:** a `.md` link extends back only across single spaces between path characters, at most 4 gaps, to a rooted run (`X:\`, `X:/`, `/`, `~`) whose last segment has no file extension; a quoted path is taken whole within 512 runes. (L1)
21. **Accepted false positive:** "cd /tmp then edit docs/notes.md" underlines from `/tmp`; the click still opens `docs/notes.md` through the shorter tails. Cheaper than an English-word filter. (L1, L3)
22. **Click order:** the whole link, its shorter tails (longest first), each folder printed earlier on the line plus the name (nearest first, at most 4), then today's session folder and git top. The disk is touched only on a click. (L2, L3)
23. **Miss note:** "Can't find plan.md in <folder>", naming the last absolute folder tried, else the session folder, home-shortened. (L3)
24. **OSC 8 stays out** (issue #128 item 3 needs its own design). (F1 records it.)
25. **`dockOf(id)` keeps its meaning** ("the agent the panel shows"), and the tab set is the new `deckOf(id)`, so the 0.24 to 0.26 dock and board tests stay valid. (D1)
26. **No shared first task:** the lanes meet only through functions that already exist (listed under Cross-lane contracts), and the merge order L, A, D puts the largest lane last so it resolves the shared hunks once.
