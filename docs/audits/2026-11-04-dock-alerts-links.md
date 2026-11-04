# Dock tabs, alert cards and path links: verification

Branch `feat/dock-alerts-links`, for release 0.27 (the version bump and changelog are their own PR).
Plan: `docs/superpowers/plans/2026-11-04-kaze-dock-alerts-links.md`. Issues: #133 (dock tabs), #9 (alert
cards; the approved design is the 2026-11-04 comment) and #128 (path links). Mockups:
`docs/design/dock-tabs-mockup.html` (option A, "one dock with tabs") and `docs/design/alert-card-mockup.html`.

Built in three worktree lanes off the branch, merged in the planned order: L (links), then A (alerts),
then D (dock), with D merging the branch back in before it removed the scratch shell.

## What shipped

### #133: workspace members as dock tabs (Lane D)

- A workspace member opens as a **tab in the panel docked under the terminal**, the same panel
  subagents use. One panel per session, one tab shown at a time, one shared height
  (`agent_panel_height`).
- **Up to 6 tabs** per session, members and agents together. A seventh replaces the oldest tab
  that is not showing. Tabs shrink equally to 72 dp, then names are cut with "…".
- **×** closes a tab; the member keeps running. **Hide ⌄** (or a second click on the shown tab or
  its sidebar line, or Esc on an agent tab with the panel's keyboard) folds the panel and keeps
  its tabs. **Expand** makes a member the main view; picking a session goes back.
- **Keyboard:** opening a member, or clicking its tab, gives it the keyboard, with a 2 dp accent
  line on whichever terminal has the keyboard while a member tab shows. A click on the session's
  terminal takes the keyboard back. Kaze's shortcuts and Shift+Enter still work in a member.
- **Size:** a member gets the panel body's grid while its tab is open, even when hidden; a drag
  resizes on release; closing the tab, Expand or closing its session gives it the shared size back.
- An **exited member keeps its tab** with the down dot and its last output (no restart button).
- **Entry points:** sidebar Workspaces rows (highlighted while docked), status bar server dots
  (open the down member, else the first open one) and the palette all open members as tabs.
  Opening a member under another session moves its tab.
- **Removed:** the scratch shell (Ctrl+Shift+` / Ctrl+` on macOS, the title bar button, the
  `scratch_shell` key, `scratch.go`, `actScratch`, `Tab.Parent`, `hookTab`) and the floating panes
  (`panes*.go`, `win.fl`, `floatMember`). An old `scratch_shell` line under `[keys]` loads with the
  warning "scratch_shell under [keys] is no longer used: the scratch shell was removed; ignored".
  The two removal commits delete about 2,800 lines.

### #9: alert cards (Lane A)

- Cards sit at the **bottom right** above the status bar, or 8 dp above the dock panel when it
  shows. Newest nearest the corner, at most 3, one per session, 380 dp wide (narrower in a narrow
  window, none under 260 dp); the oldest are left out when the room runs out.
- Each card: the Claude, Codex or workspace mark with a `!` (Needs) or `✓` (Done) badge; a context
  line with `⎇ <branch>` and the slash command; the **sidebar row name** as title (no more "zsh
  needs you"); two body lines cut with "…" (needs: Claude's message; done: the first 160 runes of
  Claude's last reply, from `last_assistant_message`, for Claude and Codex); a status line ("Needs
  your permission", "Waiting for your answer", "Done · ran 6m 40s", "Run finished" for a workspace
  run); and "Open session" with the key caps on the newest card.
- A click anywhere on a card (except ×) opens the session; for a subagent alert, its tab.
  **Ctrl+Shift+J / Cmd+J** opens the newest card's session, and walks the marks as before when no
  card shows. × closes a card and keeps the sidebar dot. Done cards hide after `done_toast`.
- The body is wrapped once per text, width, metric and theme, not every frame.

### #128: path links (Lane L)

- A `.md` link stays whole across **single spaces** back to a rooted folder (`X:\`, `X:/`, `/`,
  `~`), at most 4 gaps, so `C:/Users/john/Documents/Patrick Obsidian/.../x.md` underlines whole.
  A path in quotes is taken whole (within 512 runes). Still text-only and allocation-free per frame.
- On a click, `Links.RefsAt` lists the candidates: the whole link, its shorter tails (longest
  first), each folder printed earlier on the line plus the name (nearest first, at most 4), then the
  session folder and its git top. The first that exists opens. The disk is touched only on a click,
  outside the session lock.
- A miss says "Can't find plan.md in <folder>", naming the last absolute folder tried.
- **OSC 8 is still out of scope** (#128 item 3 needs its own design).

## Tests

Every test was written first and seen failing (per the lane commits).

| Task | Commit | Tests added |
|---|---|---|
| A1 | `01ff818` | `TestNotifyDoneSendsReplyStart`, `TestNotifyDoneCodexSendsReplyStart`, `TestNotifyDoneReplyCappedTo160Runes`, `TestReplyStart` |
| A2 | `0849292` | `TestCardTitleIsSidebarRow`, `TestCardContextDropsMissingParts`, `TestCardStatus`, `TestDoneCardRanSincePrompt`, `TestRunCardUsesWorkspaceMark` |
| A3 | `895465d` | `TestCardContrast` |
| A4 | `d522c63` | `TestCardBodyWrapsOnceAndCuts`, `TestCardKeyCapsOnNewestOnly`, `TestDoneCardShowsReply`, `TestCardsStayRightOfTheSidebar`, `TestCardsSitBottomRightNewestLowest`, `TestCardsStackAboveDock`, `TestShortWindowDropsOldestCards`, `TestNarrowAreaDrawsNoCards`, `TestCardClickOpensSession` |
| A5 | `c002d6f` | `TestNextWaitingOpensNewestCard`, `TestNextWaitingWithoutCardsWalksMarks` |
| A6 | `0db9b0e` | `TestCardRenders` (opt-in captures) |
| L1 | `1a36e0e` | `TestFindSpacedDocPaths`, `TestLinkAtExtendsAcrossWrappedRows`, `TestVisibleWithSpacedPathsAllocatesNothing` |
| L2 | `240ed83` | `TestRefsForFolderEarlierOnLine`, `TestRefsForSpacedPathTails`, `TestRefsForTailsLongestFirstAndFoldersNearestFirst`, `TestRefsForWebLink`, `TestRefsAtNothingUnderPoint` |
| L3 | `d34c86b` | `TestClickOpensDocInFolderPrintedEarlier`, `TestClickOpensDocWithSpaceInPath`, `TestClickFallsBackToShorterTail`, `TestDocMissNamesTheLastFolderTried` |
| D1 | `e85e6a0` | `TestDeckOpensTabsAndSwitches`, `TestDeckToggleHidesKeepsTabs`, `TestDeckCapReplacesOldestNotShown`, `TestDeckCloseTabMovesLeft`, `TestDeckDropsGoneAgentOnly` |
| D2 | `0ffbcc8` | `TestDockDrawsTabRow`, `TestDockTabsShrinkThenCut`, `TestDockTabClickSwitches`, `TestDockTabCloseKeepsOthers`, `TestDockHideKeepsTabs`, `TestDockHoverRedrawsOnlyOnChange`, `TestDockHiddenAgentReadsNothing` |
| D3 | `5b0e607` | `TestDockMemberGetsPanelGrid`, `TestDockMemberDragResizesOnRelease`, `TestDockMemberCloseDocksSize`, `TestDockHiddenMemberDrawsNothing`, `TestDockMemberMovesBetweenSessions`, `TestDockExitedMemberKeepsTab` |
| D4 | `061058b` | `TestDockMemberClickTakesKeys`, `TestDockTerminalClickTakesKeysBackFromMember`, `TestDockMemberShortcutsWork`, `TestDockMemberFocusReports`, `TestDockKeysAndPaneKeysExclusive` (deleted with `win.fl` in D7), `TestDockMemberKeysLeaveOnTabSwitch`, `TestShiftEnterReachesDockMember`, `TestDockMemberSelectionAutoScrolls`, `TestDockFocusLineFollowsKeys` |
| D5 | `89619b2` | `TestMemberClickDocksATab`, `TestMemberClickAgainHidesPanel`, `TestMemberClickWhenItIsTheMainViewDoesNothing`, `TestMemberClickWhileAMemberIsShownShowsIt`, `TestExpandMakesTheMemberTheMainView`, `TestPickingASessionAfterExpandGoesBack`, `TestCloseTabKeepsMemberRunning`, `TestOpeningAMemberInAnotherSessionMovesIt`, `TestSeventhTabReplacesOldest`, `TestServerDotsDockTheDownMember`, `TestDockedMemberIsHighlighted`, `TestPaletteDocksAMember`, `TestTypingAfterDockingAMemberReachesIt`, `TestDockingAMemberMovesTheFocusReports`, `TestSidebarCacheDockedMemberAllocatesNothing` |
| D6 | `fb71684` | `TestScratchShellKeyWarns`, `TestNoScratchAction`, `TestClosingSessionDocksItsMembersBack`, `TestWorktreeOpenIncludesWorkspaceFolders` |
| D7 | `5a66542` | `TestActiveProjectKeepsMainSessionContextInDockedMember`, `TestAgentLineOpensAnAgentTab` |
| D8 | `00eaddf` | `TestDockTabsRender` (opt-in captures), `TestDockTabsFrameTiming` (opt-in, `KAZE_DOCK_BENCH=1`) |

**Full check on the merged branch (2026-11-04, Windows):** `go vet ./...` is clean and `go test ./...`
passes in every package. Seven packages were first blocked by Smart App Control (blocked, not failed)
and passed when rerun with a fresh build id. The race run, the Windows GUI build and the darwin vet
from F1 Step 1 were not repeated for this docs pass; CI runs macOS and Windows on the PR.

## Frame timing (D8)

Still 1000×600 frames, headless Gio, Windows, from `TestDockTabsFrameTiming`:

| Frame | Time per frame | Allocations per frame |
|---|---|---|
| No panel | about 114-128 µs | 56 |
| Agent tab | about 188-198 µs | 86 |
| Member tab | about 117-142 µs | 63 |
| Floating member pane on `main` (same test, before) | about 131 µs | 70 |

A member tab frame allocates less than the floating pane frame it replaces (63 against 70), which
the plan required. A hidden tab or a hidden panel draws nothing, owns no renderer and reads no
transcript.

## Link benchmark (L1)

`BenchmarkVisibleLinks240x80`, 5 runs each on a busy machine: before, median 408 µs/op; after,
median 362 µs/op. Noise dominates; the point is that extending links across spaces and quotes
costs nothing measurable. `TestVisibleWithSpacedPathsAllocatesNothing` keeps the underline pass at
zero allocations per frame.

## Contrast (A3, D2)

Lowest card text contrast per theme, from `TestCardContrast` (title, body, muted, branch, command,
"Open session", needs and done colours on the card, key cap text on its cap, badge glyph on its
badge):

| Theme | Lowest |
|---|---|
| Midnight | 5.16 |
| Sumi | 4.50 |
| Tide | 6.30 |
| Washi | 4.56 |
| Sakura | 5.83 |

`TestDockContrast` checks every dock tab text at 4.5:1 or more in all five themes and passes.

## Captures

Development captures, rendered headless by opt-in tests. They are evidence for this audit, not
product screenshots for the lander.

In `docs/audits/dock-alerts-links/`:

- **Alert cards** (`KAZE_CARD_RENDER_DIR`, `TestCardRenders`), Midnight, Sumi and Washi:
  `cards-<theme>-stack.png` (1400×860, three cards stacked), `cards-<theme>-dock.png` (1400×860,
  cards above an open dock panel), `cards-<theme>-narrow.png` (900×860, narrower cards).
- **Dock tabs** (`KAZE_DOCK_RENDER_DIR`, `TestDockTabsRender`), Midnight and Sumi at 1400×860:
  `dock-<theme>-member.png` (an API member shown with the keyboard, an agent tab beside it),
  `dock-<theme>-agent.png` (Explore running, Activity), `dock-<theme>-six.png` (6 tabs, cut names),
  `dock-<theme>-exited.png` (a stopped member), `dock-<theme>-hidden.png` (panel hidden, tabs kept).

Product screenshots to retake are listed in `docs/site-assets/SCREENSHOTS.md` ("0.27"):
`kaze-sessions.png` still shows a floating pane, and `kaze-scratch.png` was removed.

## Rulings

The 26 planning rulings are in the plan ("Rulings made while planning"). All were built as
written; none was overturned by the owner during the build. The ones users will notice:

1. Tab widths 72 to 200 dp; on overflow all shrink to 72 dp, then names are cut.
2. 6 tabs per session; a seventh replaces the oldest not showing.
3. An exited member keeps its tab, with no restart button (rerun the workspace from the sidebar).
4. Hiding is not closing: Hide, a second click and Esc on an agent tab keep the tabs; × closes one.
5. Opening a member gives it the keyboard; opening an agent does not.
6. A member keeps the panel's PTY size while its tab is open, even hidden.
9. An old `scratch_shell` key loads with a warning and binds nothing.
11. The 2 dp focus line shows only while a member tab shows.
12. Cards: 380 dp, at most 3, one per session, newest nearest the corner, 8 dp above the dock.
13. Done body: the first 160 runes of the last reply, markdown marks dropped; no reply, no body line.
14. Status line wording ("permission" or "approve" in the reason means "Needs your permission").
18. Ctrl+Shift+J / Cmd+J with cards showing opens the newest card's session.
20. Links extend back only across single spaces, at most 4 gaps, to a rooted run.
21. **Accepted false positive:** "cd /tmp then edit docs/notes.md" underlines from `/tmp`; the
    click still opens `docs/notes.md` through the shorter tails. Recorded in HANDOFF, Known gaps.
22. Click order: whole link, shorter tails, earlier folders, session folder and git top.
23. Miss note names the last absolute folder tried.
24. OSC 8 stays out. Recorded in HANDOFF, Known gaps.

Documentation calls made in F1:

- The README Roadmap line and the lander's "What's new" are left for the 0.27 release PR.
- `kaze-scratch.png` and its gallery tab were deleted rather than kept for a feature that is gone.
- The lander's "Servers in the panel." card links to the README section instead of the gallery,
  until a product shot shows a member tab. The hero alt text and caption were made neutral so they
  stay true for the current screenshot.

## Known limits

- OSC 8 hyperlinks are not read (#128 item 3).
- The Ruling 21 underline false positive above.
- Dock tabs are not kept across restarts; members reopen with their workspace, without tabs.
- No member restart button; a stopped member is restarted by running its workspace again.
- Product screenshots are out of date until retaken (see Captures).

## Owner checks

- A real workspace: open a server member as a tab, type into it, click the terminal to take the
  keyboard back, Hide, Expand, and open it under another session.
- Real Claude alerts: a permission prompt, a question and a finished turn with its reply on the
  card; Ctrl+Shift+J / Cmd+J with cards showing.
- Ctrl+click (Cmd+click) on a `.md` path with a space in a folder name, and on one printed after
  its folder, on Windows and the Mac.
- An old config with `scratch_shell` under `[keys]`: the warning shows in the first tab.
