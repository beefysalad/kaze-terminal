# Reading a subagent's work: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Readable activity list show edits as open inline diffs with real line numbers and code colours, keep quiet commands folded, and open failures by themselves.

**Architecture:** The transcript reader keeps numbered diff lines from `toolUseResult.structuredPatch`. A new small package colours one line at a time. The readable list decides each step's default open state, and drawing replays one retained recording per open diff, so a still frame does no shaping for diff rows.

**Tech Stack:** Go 1.27, Gio. No new dependency.

**Spec:** `docs/superpowers/specs/2026-11-09-kaze-subagent-pane-reading-design.md`

## Global Constraints

- No new dependency for colouring. No new polling or timers.
- At most 40 lines per edit, then a muted `N more lines` row.
- Memory stays bounded: 200 steps and 256 KiB of detail in total, the oldest steps losing detail first.
- Colour a line once, never while drawing.
- A still frame with edits open: at most 120 allocations and 400 microseconds. Everything folded: within 5% of today.
- Colours and added/removed rows are not to be traded away to meet those limits.
- Every token colour at least 4.5:1 on `DiffAddBG`, `DiffDelBG` and the panel background, in all ten themes.
- One font: JetBrains Mono, Regular and Bold only.
- A diff never shows wrong line numbers: no patch means no numbers.
- Every step mode, Codex helpers, the main terminal, the Report tab and the markdown reader are unchanged. No new settings.
- Strict test first, and a failing test must be bounded. Never inject real input. No AI attribution in commits. Never push to `main`.

Windows test command used below (a new build id each run, because of Smart App Control):
`go test -timeout=120s -ldflags "-B 0x<random hex>" <package>`

## Review Focus

1. `toolUseResult` is a plain string, as on tool errors: the record must still be read and the step marked failed. Test in Task 1.
2. An edit whose result has not arrived: open, with the fallback diff and no numbers, then numbers once the result lands. Tests in Tasks 1 and 4.
3. Theme change, pane resize or display scale change while a diff is open: redrawn correctly, not a stale recording. Test in Task 5.
4. Following the bottom while new open diffs arrive: the list stays at the bottom, and folding a row that is not the last stops following, as today. Test in Task 5.
5. A step that lost its detail to the memory cap: one plain line, no empty diff. Test in Task 4.

---

### Task 1: Numbered diffs in the transcript

**Files:**
- Modify: `internal/subagents/transcript.go`
- Test: `internal/subagents/transcript_steps_test.go`

**Interfaces:**
- Produces: `DiffLine{Kind byte; Text string; Old, New int}` (0 means no number; `Kind` is `' '`, `'+'`, `'-'` or the existing `'l'`). `TranscriptStep.DiffMore int` (lines left out).

- [ ] **Step 1: Write the failing tests**

Fixtures are real-shaped user records: `{"type":"user","toolUseResult":{"structuredPatch":[{"oldStart":69,"oldLines":2,"newStart":69,"newLines":3,"lines":[" a","-b","+c","+d"]}]},"message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}`.

- `TestPatchGivesRealLineNumbers`: the Diff is `{' ',"a",69,69}`, `{'-',"b",70,0}`, `{'+',"c",0,70}`, `{'+',"d",0,71}`; `Added==2`, `Removed==1`.
- `TestNoPatchKeepsFallbackWithoutNumbers`: an Edit with a result and no patch keeps its `old_string` diff, and every `Old` and `New` is 0.
- `TestPendingEditHasFallbackThenNumbers`: before its result there are no numbers; after a result with a patch there are.
- `TestStringToolUseResultStillMarksFailure`: `"toolUseResult":"Error: String to replace not found"` with `is_error:true` sets `Failed`.
- `TestFailedEditDropsDiffAndKeepsError`: a failed Edit has `Diff==nil`, `Added==0`, `Removed==0`, and `Out` holds the error's first lines.
- `TestPatchCappedAt40`: 60 patch lines keep 40, with `DiffMore==20`.
- `TestPatchLinesAreCleaned`: a control character in a patch line is removed, as `detailLine` does.
- `TestPatchRespectsTotalDetailBudget`: 200 large edits stay within `maxStepsDetail`.

- [ ] **Step 2: Run to verify they fail**

Run the test command on `./internal/subagents/`. Expected: FAIL on the new tests only.

- [ ] **Step 3: Implement**

Add `ToolUseResult json.RawMessage` with tag `toolUseResult` to the record struct. Decode the patch only when it starts with `{`. Change the signature to `result(id string, isErr bool, raw json.RawMessage, patch []patchHunk)`, where `patchHunk` is `{OldStart, NewStart int; Lines []string}`. Set `maxDiffLines = 40` and `maxStepDetail = 8 << 10`. Keep `stepBytes` accounting correct when a diff is replaced or dropped. A failed Edit, Write or MultiEdit keeps its first `maxOutLines` lines of error text. Existing tests that pin the 24-line cap change to 40.

- [ ] **Step 4: Run to verify they pass**, along with the rest of the package.

- [ ] **Step 5: Commit**: `feat(subagents): Keep real line numbers for edits from the transcript`

### Task 2: One-line code colouring

**Files:**
- Create: `internal/codecolor/codecolor.go`, `internal/codecolor/codecolor_test.go`

**Interfaces:**
- Produces: `type Lang uint8`; `None` and one value per language; `func LangOf(path string) Lang`; `type Kind uint8` with `Plain, Keyword, String, Comment, Number, Name`; `type Span struct{ Start, End int; Kind Kind }` (byte offsets); `func Line(l Lang, s string, dst []Span) []Span`.

- [ ] **Step 1: Write the failing tests**

- `TestLangOf`: `.go`, `.py`, `.ts`, `.tsx`, `.js`, `.jsx`, `.sh`, `.bash`, `.json` map to a language; `.rb`, `README`, `""` give `None`; matching ignores case.
- `TestLineGo`: `func main() { return "x" // y` has `func` and `return` as Keyword, `main` as Name, `"x"` as String, `// y` as Comment.
- `TestLinePython`: `async def f(a): # c` has `async`, `def` as Keyword, `f` as Name, `# c` as Comment.
- `TestLineTS`, `TestLineShell`, `TestLineJSON`: one line each, with a keyword or key, a string and a number.
- `TestLineSpansCoverInOrder`: for a table of lines, spans are sorted, do not overlap, and fall on rune boundaries, including a Japanese string.
- `TestLineUnterminatedString`: `"abc` is String to the end of the line.
- `TestLineNoneIsNil`: `None` returns `dst` unchanged.
- `TestLineDoesNotAllocateWithRoom`: `testing.AllocsPerRun` is 0 when `dst` has capacity.

- [ ] **Step 2: Run to verify they fail** on `./internal/codecolor/`.

- [ ] **Step 3: Implement** a single hand-written scanner driven by a per-language table (keywords in a sorted slice, comment markers, string quotes). A word directly before `(` is a Name, as is a capitalised word in Go, TypeScript and JavaScript.

- [ ] **Step 4: Run to verify they pass.**

- [ ] **Step 5: Commit**: `feat(codecolor): Colour single lines of common languages`

### Task 3: Token colours for every theme

**Files:**
- Modify: `internal/theme/theme.go`
- Test: `internal/theme/theme_test.go`

**Interfaces:**
- Produces: `Chrome` fields `CodeKeyword, CodeString, CodeComment, CodeNumber, CodeName color.NRGBA`.

- [ ] **Step 1: Write the failing test** `TestCodeColoursReadable`: for all ten themes, each of the five colours has at least 4.5:1 contrast on `DiffAddBG`, `DiffDelBG` and the theme's terminal background, using a WCAG contrast helper (reuse the one the theme or UI tests already have if it can be reached, otherwise add one in the test file). Zero-valued colours fail.
- [ ] **Step 2: Run to verify it fails** on `./internal/theme/`.
- [ ] **Step 3: Add the five colours** to each theme, keeping that theme's character. Comments must stay distinct from plain text.
- [ ] **Step 4: Run to verify it passes.**
- [ ] **Step 5: Commit**: `feat(theme): Add code colours to every theme`

### Task 4: What is open, and what the list shows

**Files:**
- Modify: `internal/ui/dock_readable.go`, `internal/ui/dock_readable_draw.go`, `internal/ui/dock.go`
- Test: `internal/ui/dock_readable_test.go`

**Interfaces:**
- Consumes: Task 1's `DiffLine`, `DiffMore`; Task 2's `LangOf`, `Line`, `Span`.
- Produces: `readDetail` gains `diff bool`, `more int`, `spans [][]codecolor.Span`, `path string`. `readItem.openByDefault bool`. `func (s *dockSession) isOpen(it *readItem) bool`. `readHit` gains `open bool`. The `s.open` map now means a per-step override of the default.

- [ ] **Step 1: Write the failing tests**

- `TestAppliedEditOpensByDefault`: an Edit with a Diff is `isOpen`, and has `detail.diff`.
- `TestAppliedEditsAreNotFolded`: three edits in a row give three items.
- `TestSuccessfulCommandStaysFolded`, `TestFailedCommandOpensByDefault`.
- `TestFailedEditShowsErrorNoDiff`: `failed` meta, no `detail.diff`, the error in `detail.lines`, open.
- `TestNotebookEditIsHeaderOnly`: no `+0`, no detail.
- `TestReadsAndSearchesShareALine`: a neighbouring search and read run is one item whose words read `Searched the code · Read 2 files`, and a click lists what is in it.
- `TestFoldOverrideSticks`: clicking an open-by-default item closes it, and it stays closed after the list rebuilds.
- `TestStepWithoutDetailIsOneLine`: an edit whose Diff was trimmed has no chevron.
- `TestSpansComputedOncePerStep`: rebuilding the list with an unchanged edit does not call the colourer again (use a counter in a test seam).
- `TestHookStepsUnchanged`: without a transcript, edits use today's folded rows.

- [ ] **Step 2: Run to verify they fail** with `-run` on those names in `./internal/ui/`.

- [ ] **Step 3: Implement**

`foldOf` no longer folds Edit, Write or MultiEdit when the step comes from a transcript. Keep spans in `readCache`, keyed by step id plus line count, and prune them where `s.open` is pruned. `readPress` writes `s.open[id] = !hit.open`. `readItemHeight` and `drawReadStep` use `isOpen`.

- [ ] **Step 4: Run to verify they pass**, along with `./internal/ui/`.

- [ ] **Step 5: Commit**: `feat(dock): Open edits and failures by default in Readable`

### Task 5: Draw diffs inline, once

**Files:**
- Create: `internal/ui/dock_diff_draw.go`
- Modify: `internal/ui/dock_readable_draw.go`
- Test: `internal/ui/dock_render_test.go`, `internal/ui/dock_readable_draw_test.go`

**Interfaces:**
- Consumes: Task 3's colours; Task 4's `readDetail`.
- Produces: `func (win *window) drawDiffBody(gtx layout.Context, s *dockSession, it *readItem, at image.Point, width int, ink *readInk, h *dockHits) int` returning the height.

- [ ] **Step 0: Record today's numbers.** Run `KAZE_DOCK_BENCH=1` with `-run TestReadableFrameTiming -v` on this commit, and write the folded allocation count and time into the task report. The folded limit below is that count times 1.05.

- [ ] **Step 1: Write the failing tests**

- `TestDiffDrawsNumbersAndNoBox`: `win.dock.texts` has `72` and the code text. No fill of `detailBG` is issued for a diff (assert through a pixel sample of the area, as the dock render tests do).
- `TestDiffRowsColoured`: a keyword pixel in an added row differs from the plain `DiffAddFG` colour.
- `TestDiffMoreRow`: `20 more lines` appears for `more: 20`.
- `TestDiffRecordedOnce`: over 50 still frames, the shaping counter for diff rows does not rise after the first.
- `TestDiffRedrawnOnThemeAndWidth`: changing theme, width or scale records again, and the pixels match the new theme.
- `TestFollowStaysAtBottomWithOpenDiffs`: `s.follow` holds, and the last row is on screen as open diffs arrive.
- `TestFoldingMiddleRowStopsFollow`.
- `TestBoardPreviewShowsDiff`: the same diff text appears in the board preview.
- `TestOpenDiffsStayWithinAllocations`: a scene filling the pane with about 30 diff rows, `testing.AllocsPerRun` at most 120 on a still frame. Folded scene: at most the Step 0 count times 1.05.
- `TestOpenDiffsFrameTime`, gated by `KAZE_DOCK_BENCH`: at most 400 microseconds per still frame.

- [ ] **Step 2: Run to verify they fail** in `./internal/ui/`.

- [ ] **Step 3: Implement**

Keep one retained `op.Ops` recording per open diff, in the dock session, keyed by step id, line count, width, theme and metric. Record rows once: full-width add or delete background, right-aligned line number, marker, then one label per `Span`; rows clip at the pane's edge. Replay it each frame with an offset, inside the list's existing clip. Keep the recording's texts beside it and append them to `win.dock.texts` on replay. Drop recordings for steps no longer in the list. The non-diff detail (a command's output, a failed step's error) drops the box too and draws in place.

- [ ] **Step 4: Run to verify they pass**, then all of `./internal/ui/`.

- [ ] **Step 5: Commit**: `feat(dock): Draw edits inline with line numbers and code colours`

### Task 6: Verify and document

**Files:**
- Create: `docs/audits/2026-11-09-subagent-pane-reading.md`
- Modify: `README.md`, `docs/HANDOFF.md`, `docs/site-assets/SCREENSHOTS.md`

- [ ] **Step 1:** Run `go test ./...` and `go vet ./...`. Expected: pass.
- [ ] **Step 2:** Run the frame timing with `KAZE_DOCK_BENCH=1`, and write the before and after numbers, the folded and open scenes, and the app size change into the audit.
- [ ] **Step 3:** Render headless captures in Midnight and Washi and inspect them against `docs/design/pane-reading-mockup.html` option B. Record what was inspected. Real sessions remain owner checks.
- [ ] **Step 4:** Update the README's agent activity section, the handoff notes, and which screenshots need retaking.
- [ ] **Step 5: Commit**: `docs(dock): Record the pane reading work and its measurements`
