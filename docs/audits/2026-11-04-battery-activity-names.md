# Battery, readable agent activity and agent names: verification

Branch `feat/battery-activity-names`, for release 0.28 (the version bump and changelog are their own PR).
Plan: `docs/superpowers/plans/2026-11-04-kaze-battery-activity-names.md`. Issues: #111 (battery, look
"option A" from `docs/design/battery-status-mockup.html`), #134 (readable agent activity, mockup
`docs/design/readable-activity-mockup.html`, approved 2026-11-04) and #135 (subagents started together
swap names). Follow-up: #139 (battery on macOS).

Built in three worktree lanes off the branch, merged in the planned order: N (names), then B
(battery), then R (readable activity), with R merging the branch back in before its captures and
timing so they ran on the merged code. About 4,560 lines of Go added across 35 files (most of it tests).

## What shipped

### #111: battery in the status bar (Lane B), Windows only

- A small drawn battery that fills to the level, then the percent, at the **far right** of the
  status bar after the tab count, with a hairline like the other groups.
- **Charging:** a green bolt inside the icon while Windows says charging, or on AC below 100%. A full
  battery shows no bolt, even when Windows still reports charging.
- **Low:** amber (`Needs`) at 20% or less; **very low:** red (`UsageHigh`) at 10% or less. Percent
  only, no decimals, clamped at 100.
- **Narrow bars:** the battery is the first group dropped, before the tab count.
- **Desktops:** the first read that finds no battery (or fails) turns the battery off for the life of
  the window: no notifications, no more reads.
- **No polling timer:** Windows `PowerSettingRegisterNotification` for battery percentage and AC/DC
  source wakes the window once per change. If registering fails, a read at most every 60 s, only on
  frames that are drawn anyway.
- **Minimised:** a notification only marks the value dirty; the first frame after restoring reads once.
- **Bench mode** (`KAZE_BENCH`): no battery, so bench frames are unchanged.
- Not clickable, no hover, no tooltip.
- `internal/battery` builds everywhere; every non-Windows OS, macOS included, reports "no battery"
  from pure Go (`source_other.go`, `//go:build !windows`). No cgo.

### #134: readable agent activity (Lane R)

- In the dock panel's Activity and the board preview, the **agent's messages lead**, with Claude's
  coral bar and their time.
- **Each step is one sentence:**
  - Bash: Claude's `description`, a leading imperative turned past tense from a fixed table ("Ran go
    vet and the ui tests"), with **passed** / **failed · N tests** / **failed** / **done**, or
    **started** for a background run; no description gives "Ran" and the command's first line.
  - Edit / Write / MultiEdit: "Edited `<path>` +N −M".
  - Read / Grep / Glob: "Read `<file>`", "Searched the code".
  - A `git commit` whose output has `[<branch> <hash>] <subject>`: "Committed "<subject>"" with the
    short hash.
- **Folds:** consecutive reads ("Read 3 files"), searches ("Searched the code N times"), edits across
  2 or more files and to-do updates fold. Plumbing tools (`ToolSearch`, `Monitor`, `TaskStop`,
  `TaskOutput`, `BashOutput`, `KillShell`) are hidden in Readable and don't break a run; a note ends
  a run; Bash, commits and web steps never fold.
- **Click a step** to open its detail: the diff for an edit, the file list for a fold, or the command
  and the last lines of its output.
- **Readable | Every step:** one window-wide switch for the panel and the board preview, default
  Readable, not saved in `config.toml`. Every step is the old list.
- **Codex helpers** (no transcript) get sentences from the hook summaries, without counts, results or
  diffs.
- **Caps:** 200 transcript steps; per step 24 diff lines, 8 output lines, 200 runes a line and 4 KiB;
  256 KiB of detail in all (the oldest steps lose their detail first).
- The readable list is rebuilt only when the transcript's version, the hook step count, the dropped
  count, the agent's state or the transcript's availability changes; never per frame. A hidden panel
  builds nothing.
- Diff colours are new theme fields in all five themes.

### #135: agent names (Lane N)

- Agents of one type started together are matched to **their own prompt**: the first user record of
  their transcript (`FirstPrompt`, at most 64 KiB read) against the queued Agent call's prompt.
- Only contested pairings are checked (two or more candidate calls of the type, or untyped, waiting;
  or the type already contested since the owner's last prompt). The common one-call case opens no file.
- The prompt is read on the agent's tool or stop events, at most 3 times. No transcript or no match
  keeps today's pairing.
- A wrong early guess **swaps** with another unsettled agent of the same tab and session, or with a
  still-queued call. A settled name never moves.
- Matching lives in the tracker (`internal/subagents`, clock-free, reads no file); the file read is a
  separate bounded helper the UI calls.

## Tests

Every behaviour task was test-first (RED, then GREEN). All RED tests are bounded (fixed sizes,
capped reads, no unbounded loops).

| Task | Commit | Tests |
|---|---|---|
| B1 Windows read | `fa77394` | TestFromPowerStatus, TestNoBatteryOffWindows, TestNativeReadOnWindows, BenchmarkRead |
| N1 settle by prompt | `d9f5ac8` | TestReverseStartsSettleToTheirOwnPrompts, TestSettleTakesAStillQueuedCall, TestSettleNeverTakesASettledName, TestSingleCallIsNotContested, TestEmptyPromptGivesUpAfterThreeTries, TestSettleIsOnce, TestSettleStaysInItsSession, TestPromptKeyNormalizes, TestUntypedFallbackContested, TestOwnerPromptClearsContested, TestSettleMovesTheParent |
| B2 change notifications | `dae9616` | TestWatchRegistersBothSettings, TestWatchNotRegisteredWithoutBattery, TestWatchFailureUnregistersAndFallsBack, TestCallbackCallsChanged, TestWatchRealSmoke |
| N2 first prompt | `30749a1` | TestFirstPromptString, TestFirstPromptTextBlocks, TestFirstPromptSkipsLeadingRecords, TestFirstPromptSkipsToolResults, TestFirstPromptIncompleteLine, TestFirstPromptOversizedFirstLine, TestFirstPromptMissingOrDirectory |
| B3 read only on change | `d17aa28` | TestBatteryDesktopReadsOnce, TestBatteryNotifyWakesOnce, TestBatteryPollsAtMostOncePerMinute, TestBatteryPausedReadsNothing, TestBatteryCloseStopsWatching, TestBatteryTickAllocatesNothing, TestNoBatteryInBenchMode |
| N3 names in the UI | `fc8136d` | TestThreeAgentsStartedTogetherKeepTheirNames, TestNoTranscriptFallsBackWithoutStealing, TestSingleCallOpensNoFile, TestCodexHelpersNeverRead, TestSettleReadsAtMostThreeTimes |
| R1 transcript steps | `74b7e32` | TestTranscriptStepsInRecordOrder, TestTranscriptBashResult, TestTranscriptEditCounts, TestTranscriptCommitSubject, TestTranscriptResultContentBlocks, TestTranscriptPathRelativeToCwd, TestTranscriptStepCaps, TestTranscriptUnmatchedResultIgnored, TestTranscriptHandbackIsNoStep, TestTranscriptOddToolInputKeepsText, BenchmarkTranscriptRead |
| B4 status bar group | `5494914` | TestStatusBatteryColors, TestStatusBatteryReadableInEveryTheme, TestStatusBatteryCacheHits, TestStatusBatteryAddsNoAllocations, TestStatusBatteryFarRight, TestStatusDropsBatteryFirst, TestStatusNoBatteryNoGroup |
| R2 sentences and folds | `4bc155b` | TestReadableBashSentence, TestReadableCommit, TestReadableEditSentence, TestReadableFoldsReads, TestReadableFoldsSearches, TestReadableFoldsEdits, TestReadableNoteBreaksFold, TestReadablePlumbingHidden, TestReadableOtherTools, TestReadableBashDetail, TestReadableHookFallback, TestReadableMessagesLead, TestReadableIDsStable, TestReadableEndRowsAndDropped |
| R3 diff colours | `8c2c927` | TestDiffTokensInEveryTheme, TestReadInkContrast |
| B5 captures, idle | `bd34884` | TestBatteryIdleObservation, TestBatteryRenders |
| R4 panel and board | `406c37d` | TestDockShowsReadableSteps, TestDockEveryStepShowsTodaysList, TestBoardPreviewShowsReadableSteps, TestReadableBuiltOncePerChange, TestReadableHeaderCounts, TestHiddenPanelBuildsNothing, TestReadableLongDiffStaysCapped |
| R5 step details | `130891a` | TestClickStepOpensDiff, TestClickBashShowsCommandAndOutput, TestClickFoldShowsFiles, TestStepWithoutDetailIgnoresClick, TestModeToggleSwitchesBothViews, TestBoardPreviewClickOpensStep, TestOpenStepSurvivesNewRecords, TestNoteLinksStillOpen |
| R6 Codex fallback | `599bb35` | TestCodexHelperReadableFallback, TestNoTranscriptStillShowsNotes |
| R7 captures, timing | `64f2700` | TestReadableRenders, TestReadableFrameTiming |

`d78a0f2` only formats the pairing tests.

## Battery cost (B1, B3, B5)

Windows 11, 12th Gen Intel i7-1255U:

| | Result |
|---|---|
| One battery read (`BenchmarkRead`) | about 560 ns, 0 allocations |
| Status frame with the battery unchanged | 3 allocations, as before |
| Idle 2 minutes, window visible | 2 wakes, 1 read |
| Idle, window minimised | 0 wakes, 0 reads |
| App size | about +25 KB |

Idle CPU stays at zero: there is no timer, only Windows' notification for a real change.

## Transcript parse (R1)

`BenchmarkTranscriptRead`, a 200-record transcript (50 rounds of a note, a Bash call, its result and
an Edit) parsed from scratch: about 0.81 ms, 256 KB and 2,576 allocations. This runs once per
transcript change on the existing read cadence, never per frame, and only while a panel or preview
shows the agent. The hook step history (`BenchmarkToolHistoryFull`) stays at about 200 ns and 0
allocations.

## Readable frame timing (R7)

`TestReadableFrameTiming` (`KAZE_DOCK_BENCH=1`), headless 1000×600, agent tab, the same 21-step
fixture, Windows:

| View | Time per still frame | Allocations |
|---|---|---|
| Readable | about 200-207 µs | 73 |
| Every step | about 182-192 µs | 78 |
| `main` before (old list) | about 199 µs | 91 |

Readable allocates less per frame than the old list did on `main`, as the plan required.

## Contrast (B4, R3)

- Battery: outline, fill, text, the amber and red states and the bolt all reach 4.5:1 on the status
  bar in all five themes (`TestStatusBatteryReadableInEveryTheme`).
- Readable text, lowest per theme (`TestReadInkContrast`): Midnight 5.92, Sumi 4.51, Tide 6.77,
  Washi 4.51, Sakura 6.25.
- Diff tokens, each foreground on its own background (`TestDiffTokensInEveryTheme`): all at least
  4.5:1; the lowest are Washi's add at 5.95 and delete at 5.48.

## Captures

Development captures, rendered headless from test fixtures (a fake battery source and fixture
transcripts). They are evidence for this audit, not product screenshots.

- `docs/audits/battery-activity-names/battery-<theme>-<state>.png` for Midnight, Sumi and Washi:
  `normal`, `charging`, `low`, `verylow`, `narrow` and `zoom`.
- `docs/audits/battery-activity-names/activity-<theme>-<view>.png` for Midnight and Sumi: `dock`
  (running agent, notes, folds, a failed Bash), `open` (an Edit's diff open), `every` (Every step)
  and `board` (the board preview).

Product screenshots to retake are listed in `docs/site-assets/SCREENSHOTS.md` (0.28).

## macOS follow-up

Filed as #139 ("Battery in the status bar on macOS (IOKit source)"): add
`internal/battery/source_darwin.go` with `IOPSCopyPowerSourcesInfo` and
`IOPSNotificationCreateRunLoopSource`, narrow `source_other.go` to `!windows && !darwin`, build and
check it on the MacBook. Until then macOS shows no battery.

## Rulings

All 29 rulings are in the plan ("Rulings made while planning"). Ruling 1 (Windows only for 0.28) is
the owner's, from 2026-11-04. None were changed during the build. The ones users can notice:

- 1: Windows only for 0.28; macOS is #139.
- 2: a desktop is decided once; a battery added later shows after a restart.
- 3: notifications first, a lazy 60 s read on drawn frames second; no timer.
- 5: "charging" is Windows' flag or AC below 100%; full on AC shows no bolt.
- 6, 7: not clickable, no tooltip.
- 8: far right, dropped first.
- 9, 10: nothing in bench mode; nothing while minimised.
- 13, 14, 16: prompts are read only for contested pairings, on tool or stop events, at most 3 times
  and 64 KiB; no match keeps the order pairing.
- 17: the prompt key is the first 2 KiB.
- 18: swaps only between unsettled agents.
- 19: one window-wide Readable | Every step switch, not saved.
- 22, 23, 24: the Bash sentence, commit recognition and fold rules above.
- 26: Codex helpers fall back to hook summaries.
- 28: the caps above.

## Known limits

- No battery on macOS (#139), and no battery tooltip (Ruling 7).
- A battery plugged into a desktop after Kaze started shows only after a restart; if Windows
  notifications fail to register, an idle window may show a stale percent until something redraws.
- Two prompts identical in their first 2 KiB count as the same task, so the start order decides.
- A tool result more than 64 calls after its step shows no outcome.
- "failed · N tests" counts only top-level Go `--- FAIL:` lines.

## Owner checks

- On the Windows laptop: the battery at the far right, unplug and plug in (the bolt comes and goes,
  no bolt at 100%), the low and very low colours, a narrow window dropping it first.
- On a real Claude session: the readable panel and the board preview, a click on a Bash step, an
  Edit and a fold, and the Every step switch.
- Three agents of one type started in one message keep their own names on the board.
