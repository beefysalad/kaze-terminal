# Agents board redesign and sidebar follow-ups: verification

Branch `feat/agents-board`. Design: `docs/superpowers/specs/2026-11-03-kaze-agents-board-design.md`.
Plan: `docs/superpowers/plans/2026-11-03-kaze-agents-board.md`. Mockups: `docs/design/agents-board-mockup.html`
(the board) and `docs/design/sidebar-active-highlight-mockup.html` (option 3, "Session bar only").

## What changed

**Sidebar**

- **No project rail.** 0.25's 2 dp accent rail beside the active project is gone. The project's
  header keeps its bright, bold name. The selected session keeps its own accent bar and row fill,
  so there is one accent line, not two.
- **A click on an agent line opens its panel again**, and a second click on the same line closes
  it (0.24's behaviour). The `›` button, the double-click rule and safe click are gone.
- **"+N more" opens the board filtered to that session.** It replaces the in-place expand to 8
  lines and the "Show fewer" footer, which are removed. The tray itself (3 lines, the chip, the
  fold, the tooltip) is unchanged.

**The board**

- A list of **session cards** on the left and a **preview** of the selected agent on the right,
  in place of the three status columns.
- **Header:** "Agents", "N agents in M sessions", and "Back to <session>".
- **Filter:** segments All, Needs you, Running and Done with counts (the Needs you count is amber
  when it is not zero), a search field (type, task, session title and repo), and a removable
  "<session> ×" token when the board was opened from a session's "+N more". `/` focuses the
  search; Esc clears a search with text, otherwise it closes the board.
- **Needs you card** pinned on top under All, with an amber tint and border; each row names its
  session before the step.
- **One card per session**, in sidebar order: agent logo, title, repo and branch, "Clear" when
  it has finished agents, "Go to session ›", and the **state meter** (needs, running and done in
  proportion, counting all of the session's agents). Rows show the state mark, the type in
  magenta, the task, the elapsed time, and a quieter line with the current step (or the report's
  first line once done). Finished agents fold into "✓ N finished ›" and "Hide finished".
- **Preview:** state, type pill and time; the task as a title; "In <session> · <repo>"; **Open in
  session**, plus **Copy report** and **Open in reader** when done. The body is the docked panel's
  Activity timeline (with the approval box while waiting) or its rendered report. It reads the
  transcript only while it is drawn, at most once a second, and stops once a finished agent's
  transcript was read to the end.
- **Keys and clicks:** a click selects, a double-click or Enter opens the agent in its session,
  ↑/↓ move the selection through visible rows. Below 860 dp of board width there is no preview
  and a click opens the agent in its session.
- **This audit's fix:** a finished agent's second line (on the board and in the sidebar tooltip)
  showed the raw first line of its report, often `## Summary`. It now shows the first line of
  prose with headings, code blocks, tables, list and quote marks and inline `**` / `` ` `` marks
  dropped (`reportLine`, `TestSideAgentFromTracker`).

## Tests

All new or changed tests pass (`go test ./...` and `go vet ./...` on Windows).

- **Task 1 (board state):** `TestOpenBoardResetsViewAndSelectsWaiting`, `TestOpenBoardWithoutWaitingSelectsNothing`,
  `TestClearBoardSession`, `TestBoardCardOpensPane`, `TestBoardCardOpensDock`.
- **Sidebar (Tasks 2 to 4):** `TestActiveProjectNameStyle`, `TestSidebarTextContrast`,
  `TestActiveProjectFollowsSelectedSession` (the rail's RED was the compile failure of the removed
  `TestActiveProjectRail`), `TestAgentLineTogglesPanel`, `TestAgentLineOpensOtherSession`,
  `TestAgentLineQuickClicksToggleTwice`, `TestTrayChipOpensBoardForSession`, `TestTrayFoldControl`,
  `TestTrayFoldClearOfClose`, `TestTrayTooltipShowsStep`, `TestTrayTooltipFlipsAbove`,
  `TestTrayTooltipGoneOnLeave`, and `TestSidebarCache*` still allocate nothing.
- **Board model (Task 5):** `TestBoardViewNeedsCardAndCards`, `TestBoardViewSidebarOrder`,
  `TestBoardViewFilters`, `TestBoardViewSearch`, `TestBoardViewSessionToken`, `TestBoardViewShowDone`,
  `TestBoardViewSessionWait`, `TestBoardSummary`, `TestMeterSplit`.
- **Colours (Task 6):** `TestBoardContrast`.
- **List (Task 7):** `TestBoardDrawsHeaderAndSegments`, `TestBoardNeedsCardFirst`, `TestBoardCardHeaderAndMeter`,
  `TestBoardDoneFold`, `TestBoardSegmentPress`, `TestBoardTokenClears`, `TestBoardSearchPressFocuses`,
  `TestBoardGoToSession`, `TestBoardDismissDoneCard`, `TestBoardClearDone`, `TestBoardEmptyStates`.
- **Preview (Task 8):** `TestBoardPreviewPickAnAgent`, `TestBoardPreviewRunning`, `TestBoardPreviewWaiting`,
  `TestBoardPreviewDone`, `TestBoardPreviewParsesOnce`, `TestBoardPreviewStopsPollingWhenDone`,
  `TestBoardCopyReportUsesPreviewTranscript`.
- **Input (Task 9):** `TestBoardClickSelects`, `TestBoardDoubleClickOpensInSession`, `TestBoardOpenInSessionAndEnter`,
  `TestBoardArrowsStayInVisibleRows`, `TestBoardSlashFocusesSearch`, `TestBoardSlashTypesInSearch`,
  `TestBoardEscClearsSearchFirst`, `TestBoardSearchSubmitKeepsText`, `TestBoardNarrowClickOpensSession`,
  `TestBoardSelectionGone`, `TestBoardTokenDropsWithSession`, `TestBoardHoverRedrawsOnlyOnChange`,
  `TestBoardPressElsewhereLeavesSearch`.
- **Task 10:** `TestBoardFrame50Steady`, `BenchmarkBoardFrame50`, the opt-in `TestBoardRenders`,
  and `TestSideAgentFromTracker` (report line).

Known flaky test, older than this work: `TestDockFocusedPanelPassesTypingOn` fails now and then in a
full run and passes alone. It did not fail in this audit's runs.

`GOOS=darwin go vet ./internal/ui/` cannot run on Windows (Gio's darwin GL code needs cgo); the
macOS CI covers it.

## 50 agents

`boardFixture50`: 5 sessions with 10 agents each (1 waiting, 6 running, 3 done); one running agent
has a 20-message transcript and is previewed. `BenchmarkBoardFrame50` draws the wide board
(1400 × 860) once per op on a reset `op.Ops`, i7-1255U, `-count 5`, three alternating runs each.
"Before" is `main` (0.25, three columns, no preview) in a temporary worktree, with the same
fixture and no selection.

| | ns/op (median of runs) | B/op | allocs/op |
|---|---|---|---|
| Before (0.25 board) | about 470,000 to 495,000 | 30,688 | 636 |
| After (new board with preview) | about 390,000 to 510,000 | 47,144 | 244 |

Time per frame is the same within noise (both near half a millisecond), allocations drop by
about 60 percent, and the bytes go up by about 16 KB because the preview lays out the transcript
notes and timeline each frame. The board draws only while it is shown.

`TestBoardFrame50Steady`: after 5 warm-up frames, 20 frames inside one poll window read no
transcript (`trAt` unchanged) and parse no markdown (`cache.parses` unchanged). 244 allocations
per still frame (`testing.AllocsPerRun`).

## Contrast

`TestBoardContrast` checks every board text against each surface it sits on, amber on the Needs
you tint included. Lowest ratio per theme: Midnight 4.51, Sumi 4.50, Tide 4.54, Washi 4.50,
Sakura 4.51.

## Captures

Development captures from the real Gio frame (`TestBoardRenders`, `TestSidebarTrayRenders`, headless
GPU, no native window or input), not product screenshots. Saved in `docs/audits/agents-board/`:

- `board-sumi-all.png`, `board-midnight-all.png`: the Needs you card, two session cards with
  meters and folded finished rows, a running agent's preview.
- `board-sumi-done.png`, `board-midnight-done.png`: the Done filter and a report preview.
- `board-sumi-search.png`, `board-midnight-search.png`: a session token and the query "split".
- `board-sumi-empty.png`, `board-midnight-empty.png`: "No agents match", with the hooks hint.
- `board-sumi-narrow.png`, `board-midnight-narrow.png`: 1000 × 600, no preview.
- `tray-sumi.png`, `tray-midnight.png`, `tray-washi.png`: the tray (default, folded, tooltip,
  open line highlighted), with no project rail.

To regenerate: set `KAZE_BOARD_RENDER_DIR` and `KAZE_TRAY_RENDER_DIR` to the folder and run
`go test ./internal/ui/ -run 'TestBoardRenders|TestSidebarTrayRenders'`.

Compared with the mockup rendered in headless Edge, the captures match its layout: header and
summary, segments with counts, the search field with its `/` hint, the amber Needs you card,
session cards with meters and "Go to session ›", the folded "✓ N finished ›", and the preview's
eyebrow, title, "In <session> · <repo>", primary button and Activity timeline.

**Fixed:** the finished row's `## Summary` (see "What changed").

**Left on purpose:**

- `general-purpose` is cut to "general-pu…" in the fixed type column, as the spec's fixed width
  asks. The mockup's shorter "general" is sample data.
- When a filter or search hides the selected agent, the preview keeps showing it (see the empty
  capture). The spec does not say to drop it, and keeping it means a cleared search returns to
  the same agent.
- Cards carry "Clear" next to "Go to session ›" (Ruling 5); the mockup has no Clear.
- The test sessions have no git, so card headers show no branch; real sessions show it.
- In the 600 px narrow capture, the sidebar's tray runs under the "New session" section; that is
  the sidebar at a short height, older than this work.

## Rulings

From the plan, each cheap to overturn:

1. The preview shows when the board's own area is at least 860 dp, not the window width.
2. Segment counts follow the session token but ignore the search and the segment; the summary
   is global.
3. The Needs you card shows only under All; the Needs you segment shows session cards holding
   waiting rows.
4. A session wait (no subagent named) is a Needs you row that goes to its session; it can't be
   previewed or reached by arrows.
5. "Clear" sits in each card header (only with finished agents); the × shows on a done row when
   hovered or selected.
6. The preview body sits on the terminal background, like the docked panel.
7. Esc in the search with text clears it and gives the keyboard back to the board; Enter keeps
   the text and does the same.
8. The board drops a closed session's token or a gone selection when it draws.
9. The rail's removal has no pixel test; the captures check the look.
10. The chip keeps its text and `⌄` caret although it now opens the board.
11. "Back to <session>" loses 0.25's trailing ×.
12. The preview's dock helpers take a `texts *[]string` so board tests read the board's texts.

Lane B also made these calls: the preview's "Open in session" hit area is `b.openIn`; board text
colours are nudged against every surface they sit on; a `needsRaised` colour exists for amber on
raised surfaces; colours are worked out once per theme; `reveal` and `selY` scroll the selection
into view after ↑/↓.

## Owner checks

- Product screenshots (lander and README) need retaking with the new board and tray
  (`docs/site-assets/SCREENSHOTS.md`).
- Try the board with real Claude Code sessions: the preview's live notes, Copy report and Open in
  reader, and the narrow fallback.
- The macOS build is covered by CI only.
