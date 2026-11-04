# Battery, Readable Agent Activity and Agent Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship three approved issues as one 0.28 bundle: the battery level in the status bar on Windows laptops (#111, macOS deferred), an agent's activity that reads like Claude Code's own transcript (#134), and subagents started together keeping their own names (#135).

**Architecture:** Three lanes in separate worktrees off `feat/battery-activity-names`, with no shared first task and no shared files (listed below). **Lane N** (names) records how each subagent was paired, flags pairings made while two or more calls of the same type were waiting, and settles each flagged agent once by matching the first user message of its transcript (`subagents.FirstPrompt`) against the queued calls' prompts, swapping names when a later match proves an earlier pairing wrong. **Lane B** (battery) adds `internal/battery` (Windows: `GetSystemPowerStatus` plus power-setting change notifications; every other OS, macOS included for now: "no battery"), a `batteryWork` in `internal/ui` that reads only on a change notification (or at most once a minute on frames that happen anyway) and never again on a desktop, and a `grpBattery` status bar group, far right, dropped first. **Lane R** (readable activity) teaches `subagents.Transcript` to keep each tool call's facts (description, command, path, line counts, capped diff, capped output tail, failed or not, commit subject and hash) in record order, builds a folded list of one-sentence steps once per transcript change (`readableItems`), and draws it in the docked agent panel and the board preview with click-to-open detail and an "Every step" toggle that keeps today's list. A final task merges the lanes, runs the whole check and updates README, lander, HANDOFF, SCREENSHOTS and the audit.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`gpu/headless` for opt-in captures), `golang.org/x/sys/windows` v0.49.0 (already in `go.mod`: `LazySystemDLL`, `NewCallback`), existing `internal/subagents`, `internal/notify`, `internal/theme`, `render.Readable`.

**Inputs:** issues #111 (body; look "option A" chosen 2026-11-02), #134 (body and the 2026-11-04 approval comment), #135 (body); mockups `docs/design/battery-status-mockup.html` and `docs/design/readable-activity-mockup.html`. On behaviour the issue wins; on look the mockup wins. **Scope change from the owner (2026-11-04):** #111 ships the Windows source only; the macOS IOKit source is a follow-up issue the owner builds on their MacBook (Ruling 1).

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: fixed fixture sizes (at most 3 sessions, 16 queued calls (`maxQueue`), 200 steps, 64 KiB prompt reads, 1 MiB oversized-line fixtures written to a temp file), no unbounded loops, `time.After` on any wait, fake PTYs (`recStarter`) and fake battery sources only.
- No global keyboard or mouse input. UI tests call the handlers (`dockBody`, `dockHead`, the board's event loop through `input.Router`) or queue `pointer.Event`s on an `input.Router` (`clickAt`, `typeKeys` in `testkit_test.go`), never SendInput. No test changes the machine's power state.
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`). Every `go test` in this plan means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`
  or in PowerShell `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags ("-B 0x{0:x16}" -f (Get-Random -Maximum ([long]::MaxValue))) <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off.
- **No C compiler on this Windows machine.** Nothing in this plan needs cgo. The non-Windows battery file is pure Go; `GOOS=darwin go vet ./internal/battery/` runs locally, and the macOS CI job (`.github/workflows/test.yml`, `macos-15`) is the macOS check for the whole bundle.
- Lean: **no new timer, ticker or goroutine loop.** The battery reads on a Windows change notification, else at most once per `batteryEvery` (60 s) on a frame that happens anyway (the status bar's existing piggyback, like `resourceWork.tick` and `memEvery`), never while minimized, and never again after a first check finds no battery. A transcript is still read only by `dockTick` for the shown agent tab or the board preview; the readable list is built once per transcript or agent change, never per frame; a hidden panel reads and builds nothing. The prompt check of #135 opens a file only for an agent whose pairing was contested, at most 3 times, each read capped at 64 KiB.
- Allocations: the sidebar keeps zero allocations on unchanged frames (`TestSidebarCache*` pass in every lane). The status bar's unchanged-frame allocations stay as they are with a battery shown (`TestStatusBatteryAddsNoAllocations`). The readable Activity frame allocates no more than today's Activity frame for the same agent (R7 timing).
- Colours come only from the theme (`Chrome, Active, ActiveFG, Muted, Border, Accent, Folder, Needs, Done, UsageHigh`, the existing `claudeCoral` via `agentLogoColor("claude", ...)`, and the four new diff tokens of R3) with local `render.Readable` nudges. Every new text is at least 4.5:1 on its surface in all five `theme.All()` themes, diff lines on their diff backgrounds included.
- Copy fixed by the issues: "Read <file>", "Read 3 files", "Searched the code", "Searched the code 4 times", "Edited <path> +N −M", "Committed "<subject>"", "passed", "failed · 1 test", "Every step", "Readable"; battery "82%".
- Lanes and file ownership: see "Lanes, shared files and merge order". Each lane works in its own worktree (`git worktree add ../kaze-<lane> -b feat/battery-activity-names-<lane> feat/battery-activity-names`) and merges back into `feat/battery-activity-names`.
- Commits: `type(scope): Verb ...`, **no AI attribution lines of any kind** (CLAUDE.md overrides any default). Never push to `main`; the owner merges. Release 0.28 (version bump, changelog, lander "What's new") is its own PR, not in this plan.

## Lanes, shared files and merge order

| Lane | Tasks | Owns |
|---|---|---|
| **N** names (#135) | N1 to N3 | `internal/subagents/subagents.go`, `internal/subagents/subagents_test.go`, `internal/subagents/prompt.go` (new), `internal/subagents/prompt_test.go` (new), `internal/subagents/pairing_test.go` (new), `internal/ui/subagents_ui.go`, `internal/ui/subagents_pair_test.go` (new) |
| **B** battery (#111) | B1 to B5 | `internal/battery/*` (new package), `internal/ui/battery.go` (new), `internal/ui/battery_test.go` (new), `internal/ui/statusbar.go`, `internal/ui/statusbar_draw.go`, `internal/ui/statusbar_test.go`, `internal/ui/status.go` (`statusModel` only), `internal/ui/chrome_cache.go` (`sameStatus` only), `internal/ui/window.go` (`initBattery` call beside `initResources`, `battery.close()` beside `resources.close()`, `battery.pause` beside `resources.pause` in `onConfig`, the `battery` field), `internal/ui/statusbar_render_test.go` (new) |
| **R** readable activity (#134) | R1 to R7 | `internal/subagents/transcript.go`, `internal/subagents/transcript_steps_test.go` (new), `internal/theme/theme.go`, `internal/theme/theme_test.go`, `internal/ui/dock_readable.go` (new), `internal/ui/dock_readable_test.go` (new), `internal/ui/dock_readable_draw.go` (new), `internal/ui/dock_readable_draw_test.go` (new), `internal/ui/dock_draw.go` (`drawDockActivity`, `dockHits`), `internal/ui/dock.go` (`dockSession` fields only), `internal/ui/dock_input.go` (`dockBody`), `internal/ui/board.go` (the preview press branch of the event loop), `internal/ui/board_preview.go`, `internal/ui/window.go` (one field: `actEvery bool`), `internal/ui/dock_render_test.go` |
| Final | F1 | `README.md`, `docs/index.html` (feature cards only), `docs/HANDOFF.md`, `docs/site-assets/SCREENSHOTS.md`, `docs/audits/2026-11-04-battery-activity-names.md` (new) |

**Files two lanes touch** (each edit stays inside the named lines, so git merges the hunks; the owner resolves any conflict):

| File | Lane B | Lane R | Lane N |
|---|---|---|---|
| `internal/ui/window.go` | the `battery *batteryWork` field, `win.initBattery()` after `win.initResources()`, `win.battery.close()` after `win.resources.close()`, `win.battery.pause(...)` after `win.resources.pause(...)` in `onConfig` | the `actEvery bool` field only (its own line in the `window` struct, away from B's) | none |
| `internal/subagents/*` | none | `transcript.go` and its new test file only | `subagents.go`, `prompt.go` and their tests only; `pairing_test.go` may call `fixtureLine` from `transcript_test.go` but never edits that file |

**Cross-lane contracts (unchanged signatures, nobody renames them):** `subagents.Transcript.Read(path)`, `Transcript.Version`, `Transcript.Messages`, `Transcript.Available`, `subagents.TranscriptPath(main, id)`, `win.agentTranscriptPath(a)` (Lane N calls it, Lane R leaves it), `win.dockTick(s, a, now)`, `Tracker.Apply`, `Tracker.Get`, `Agent.Task`, `Agent.Prompt`, `win.statusModel(now)`, `layoutStatusBar`.

**Merge order:** N first (smallest), then B, then R (largest; it is the only lane that touches the dock and the board, so it merges once with nothing to resolve but the `window.go` struct field). R merges `feat/battery-activity-names` into its worktree before R7 so the captures and timing run on the merged code. Then F1 runs on `feat/battery-activity-names`.

## Review Focus

1. **Desktops pay nothing after the first check** (#111 must-hold). A source that reports no battery is read once, never registers for notifications, and the status model never asks again. (B3 `TestBatteryDesktopReadsOnce`, B2 `TestWatchNotRegisteredWithoutBattery`.)
2. **No timer, nothing while hidden.** The battery reads only after a change notification or on an existing frame at most once a minute; minimized, a notification only marks it dirty, and restoring reads once. (B3 `TestBatteryPollsAtMostOncePerMinute`, `TestBatteryPausedReadsNothing`, `TestBatteryNotifyWakesOnce`.)
3. **Status bar cost unchanged:** a shown battery adds no allocations to an unchanged frame and the layout cache still hits. (B4 `TestStatusBatteryAddsNoAllocations`, `TestStatusBatteryCacheHits`.)
4. **Readable list built once per change, not per frame**, and caps hold on a huge edit or output. (R4 `TestReadableBuiltOncePerChange`, R1 `TestTranscriptStepCaps`.)
5. **A swap never steals a settled name** and an agent without a transcript keeps today's pairing. (N1 `TestSettleNeverTakesASettledName`, N3 `TestNoTranscriptFallsBackWithoutStealing`.)
6. **The uncontested case reads no file** (one Agent call per type, the common case). (N3 `TestSingleCallOpensNoFile`.)

---

## Lane N: agent names (#135), Tasks N1 to N3

### Task N1 (Lane N): Remember each pairing and settle contested ones by prompt

**Files:**
- Modify: `internal/subagents/subagents.go` (`Agent` gains unexported `call call`, `contested bool`, `checks uint8`; `Tracker` gains `contested map[queueKey]map[string]bool`; `pair`, `Apply` (`EvPrompt` and `session` clear the contested marks with the queue), new `promptKey`, `PromptCheck`, `SettlePrompt`, `setCall`)
- Create: `internal/subagents/pairing_test.go`

**Interfaces:**
- Produces:
  - `const promptKeyBytes = 2048`, `const maxPromptChecks = 3`.
  - `func promptKey(s string) string`: `strings.ReplaceAll(s, "\r\n", "\n")`, `notify.CleanReport(s, notify.MaxPrompt)`, `strings.TrimSpace`, then the first `promptKeyBytes` bytes cut back to a rune boundary. The hook's prompt is already `FitReport`ed to 16 KiB, the transcript's is whole; both keys agree because only the first 2 KiB count (Ruling 17).
  - `pair` records the chosen call in `a.call` and sets `a.contested = true` when, at that moment, the queue held 2 or more calls the rule could have taken (2+ of `e.AgentType`, or 2+ untyped when it falls back), or when that type is already marked contested for the queue; it then marks `t.contested[q][typ]`. An agent paired from a contested type keeps the flag even when it took the last call (Ruling 15).
  - `func (t *Tracker) setCall(a *Agent, c call)`: `a.call = c`; `a.Task = c.desc`, or `a.Type`, or `unknownTask` when both are empty; `a.Prompt = c.prompt`; `a.Parent, a.Depth` from `c.parent` as `pair` does today (zero when the parent is gone or is `a`).
  - `func (t *Tracker) PromptCheck(k Key) bool`: the agent exists, is `contested`, and `checks < maxPromptChecks`.
  - `func (t *Tracker) SettlePrompt(k Key, prompt string) bool` (reports whether anything shown changed; `revision++` always):
    1. Not `PromptCheck(k)` → false. `prompt == ""` → `checks++`; at `maxPromptChecks` the agent stops being contested (today's pairing stays, Ruling 16); false.
    2. `key := promptKey(prompt)`; `a.contested = false` from here on (it is read once).
    3. `promptKey(a.call.prompt) == key` → false (it was right).
    4. A call still queued under `queueKey{a.src, a.Key.Session}` whose key matches → it replaces `a.call` in the queue at the same index (a's old call goes back for a later start) and `setCall(a, c)`; true.
    5. Another agent `b` with the same `src` and session, `b.contested` still true (not settled), and `promptKey(b.call.prompt) == key` → swap: `setCall(a, b.call)`, `setCall(b, aOld)`; true. A settled agent is never a donor (Ruling 18).
    6. No match → false (today's pairing stays).
- `Agent.Prompt`, `Agent.Task` keep their meaning; `pair`'s existing rules (type first, untyped fallback, parent and depth) are unchanged, and every test in `subagents_test.go` keeps passing.

- [ ] **Step 1: Write the failing tests** in `pairing_test.go` (helpers `ev`, `agentCall`, `tool` from `subagents_test.go`; a call's prompt set on the event):
  - `TestReverseStartsSettleToTheirOwnPrompts`: three `general-purpose` calls "Lane A" / "Lane D" / "Lane L" with prompts pA, pD, pL; starts in reverse order (ids d3, d2, d1); before settling, d3 is "Lane A" (today's bug); `SettlePrompt(d3, pL)`, `SettlePrompt(d2, pD)`, `SettlePrompt(d1, pA)` → d1 "Lane A" + pA, d2 "Lane D" + pD, d3 "Lane L" + pL, and each `Prompt` matches.
  - `TestSettleTakesAStillQueuedCall`: two calls queued, one start paired with call 1, `SettlePrompt` with call 2's prompt → the agent has call 2; the next start takes call 1.
  - `TestSettleNeverTakesASettledName`: a and b contested; a settles right; b settles with a's prompt (a corrupted transcript) → no change, a keeps its name.
  - `TestSingleCallIsNotContested`: one call, one start → `PromptCheck == false`; two calls of different types → neither contested.
  - `TestEmptyPromptGivesUpAfterThreeTries`: `SettlePrompt(k, "")` 3 times → `PromptCheck == false`, name unchanged, other agents untouched.
  - `TestSettleIsOnce`: after one non-empty settle, a second with another prompt → false, unchanged.
  - `TestSettleStaysInItsSession`: same type in two sessions (same src) → a settle never swaps across sessions.
  - `TestPromptKeyNormalizes` (table): CRLF vs LF, trailing spaces and blank lines, a 20 KiB prompt vs the same prompt `FitReport`ed to `notify.MaxPrompt` → equal keys; two prompts differing only after byte 2048 → equal (documented limit); different first lines → different.
  - `TestUntypedFallbackContested`: two untyped calls, two `general-purpose` starts → both contested; settle swaps them.
  - `TestOwnerPromptClearsContested`: an `EvPrompt` → the contested marks of that queue go with it.
- [ ] **Step 2: Run** `go test ./internal/subagents/ -run 'TestReverseStarts|TestSettle|TestSingleCall|TestEmptyPrompt|TestPromptKey|TestUntypedFallback|TestOwnerPromptClears'` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/subagents/` → PASS (`TestPairsCallsInOrder`, `TestPairingMatchesType`, `TestCallPromptPairsWithItsAgent`, `TestUntypedCallPairsWithItsStart`, `TestLateTypeFillsInTheAgent` unchanged); `history_alloc_test.go` still passes.
- [ ] **Step 5: Commit** `fix(subagents): Settle agents started together by their prompt`

### Task N2 (Lane N): Read a subagent transcript's first prompt, bounded

**Files:**
- Create: `internal/subagents/prompt.go`, `internal/subagents/prompt_test.go`

**Interfaces:**
- Produces:
  - `const maxPromptRead = 64 << 10`.
  - `func FirstPrompt(path string) (string, bool)`: opens `path` (regular files only, like `Transcript.Read`), reads at most `maxPromptRead` bytes with one `io.ReadFull` into a stack-sized buffer from a `sync.Pool`, walks complete lines only, and returns the message text of the first record with `"type":"user"` whose `message.content` is a JSON string, or an array whose `text` blocks are joined with `"\n"`; a user record holding only `tool_result` blocks is skipped. Not found, missing file, or a first user line longer than the window → `"", false` (the caller counts it as a try).
- [ ] **Step 1: Write the failing tests** (temp files; `fixtureLine` from `transcript_test.go` used read-only plus a new local `userLine(content any) string`):
  - `TestFirstPromptString`, `TestFirstPromptTextBlocks` (two text blocks joined), `TestFirstPromptSkipsLeadingRecords` (a summary line, then the user line), `TestFirstPromptSkipsToolResults`.
  - `TestFirstPromptIncompleteLine`: the user line without its newline → false; with it → true.
  - `TestFirstPromptOversizedFirstLine`: a 1 MiB user line → `"", false`, and the test asserts it finished reading in one call (bounded: no loop past 64 KiB, checked by wrapping `os.Open` through a package var `openPrompt` that counts bytes read ≤ `maxPromptRead`).
  - `TestFirstPromptMissingOrDirectory` → false.
- [ ] **Step 2: Run** `go test ./internal/subagents/ -run TestFirstPrompt` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/subagents/` → PASS.
- [ ] **Step 5: Commit** `feat(subagents): Read the prompt a subagent transcript starts with`

### Task N3 (Lane N): Settle contested agents from the UI, once

**Files:**
- Modify: `internal/ui/subagents_ui.go` (`drainSubs`; new `win.settlePairing(k subagents.Key)`; package var `firstPrompt = subagents.FirstPrompt` for tests)
- Create: `internal/ui/subagents_pair_test.go`

**Interfaces:**
- Produces: in `drainSubs`, after `win.subs.Apply(e)` for `EvTool` or `EvStop` with an `AgentID`, `if win.subs.PromptCheck(k) { win.settlePairing(k) }`. `settlePairing`: `a := win.subs.Get(k)`; `path := win.agentTranscriptPath(a)`; `path == ""` (Codex, unknown main transcript) → `SettlePrompt(k, "")`, counted as a miss like an unreadable file (the tracker gives up after 3); else `p, _ := firstPrompt(path)` and `SettlePrompt(k, p)`. A change invalidates the window (the tray and board already redraw on `Revision`). Why on a tool event: by then the agent has written its first record, so the prompt is on disk (Ruling 14).
- [ ] **Step 1: Write the failing tests** (`liveWindow`; a temp main transcript `<tmp>/s.jsonl`, and `subagents.TranscriptPath` files written per agent; messages queued through `win.subIn` as `eventsOf(...)` produces them, then `drainSubs`):
  - `TestThreeAgentsStartedTogetherKeepTheirNames`: three `general-purpose` Agent calls "Lane A" / "Lane D" / "Lane L", SubagentStart in reverse, each agent's transcript starting with its own prompt, then one tool event each → `win.subs.Get(k).Task` is right for all three, and the board rows read the same (`boardRows` texts).
  - `TestNoTranscriptFallsBackWithoutStealing`: the same, but one agent's file missing → after its 3 tool events it keeps the order pairing; the other two still settle to their own names.
  - `TestSingleCallOpensNoFile`: one call, one start, 5 tool events → `firstPrompt` counter 0.
  - `TestCodexHelpersNeverRead`: two Codex helpers (`Engine: "codex"`) of one role → counter 0, today's pairing.
  - `TestSettleReadsAtMostThreeTimes`: an empty transcript file, 10 tool events → counter 3.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestThreeAgentsStarted|TestNoTranscriptFalls|TestSingleCallOpens|TestCodexHelpersNever|TestSettleReads'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Sub|Board|Dock|Sidebar|Tray'` and `go test ./internal/subagents/` → PASS.
- [ ] **Step 5: Commit** `fix(ui): Give subagents started together their own names`

---

## Lane B: battery in the status bar (#111, Windows only), Tasks B1 to B5

### Task B1 (Lane B): The `internal/battery` package and its Windows read

**Files:**
- Create: `internal/battery/battery.go` (no build tag: types and the pure decoder), `internal/battery/battery_test.go`, `internal/battery/source_windows.go`, `internal/battery/source_windows_test.go`, `internal/battery/source_other.go` (`//go:build !windows`), `internal/battery/source_other_test.go` (`//go:build !windows`)

**Interfaces:**
- Produces:
  - `type Status struct { Present, Charging bool; Percent int /* 0-100 */ }` (comparable, no pointers: the status model copies it without allocating).
  - `type Source interface { Read() (Status, error); Watch(changed func()) (stop func(), ok bool) }`: `Watch` registers for OS change notifications and returns `ok == false` when the platform has none or registration failed.
  - `func Native() Source`.
  - `func fromPowerStatus(acLine, flag, percent byte) Status`: `flag == 128` (no system battery) or `flag == 255` (unknown) → `Status{}`; `percent == 255` (unknown) → not present; else `Present`, `Percent = min(int(percent), 100)`, `Charging = flag&8 != 0 || acLine == 1 && percent < 100` (Ruling 5).
  - Windows `source_windows.go`: `kernel32 = windows.NewLazySystemDLL("kernel32.dll")`, `procGetSystemPowerStatus`; `type systemPowerStatus struct { ACLineStatus, BatteryFlag, BatteryLifePercent, SystemStatusFlag byte; BatteryLifeTime, BatteryFullLifeTime uint32 }`; `Read` calls it once (no allocation) and returns `fromPowerStatus`. `Watch` is B2; in B1 it returns `nil, false`.
  - Other OSes, **macOS included for now** (`source_other.go`, `//go:build !windows`): `Read` → `Status{}, nil`, `Watch` → `nil, false`. The comment says the darwin IOKit source is the follow-up issue and that adding `source_darwin.go` means narrowing this tag to `!windows && !darwin` (Ruling 1).
- [ ] **Step 1: Write the failing tests**:
  - `TestFromPowerStatus` (table, `battery_test.go`, runs on every OS): no battery (128), unknown (255), unknown percent, 82 on battery, 64 charging (flag 8), 100 on AC (not charging), 18, 7, a bogus 130 → 100.
  - `TestNativeReadOnWindows` (`source_windows_test.go`): `Native().Read()` returns no error and `0 <= Percent <= 100`; when `!Present`, `Percent == 0`. (Does not assume this machine has a battery.)
  - `TestNoBatteryOffWindows` (`source_other_test.go`, runs in the macOS CI job): `Read` → `Status{}`, nil; `Watch` → ok false.
- [ ] **Step 2: Run** `go test ./internal/battery/` → FAIL (package missing). `GOOS=darwin go vet ./internal/battery/` → FAIL the same way.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/battery/`, `go vet ./internal/battery/`, `GOOS=darwin go vet ./internal/battery/`, `GOOS=linux go vet ./internal/battery/` → PASS. `go test -run TestFromPowerStatus -count=1 -benchmem -bench BenchmarkRead ./internal/battery/` with a new `BenchmarkRead` on Windows: 0 allocs/op; note ns/op in the commit body (the issue's "Windows cost not verified yet").
- [ ] **Step 5: Commit** `feat(battery): Read the battery level on Windows`

### Task B2 (Lane B): Windows power change notifications

**Files:**
- Modify: `internal/battery/source_windows.go` (`Watch`)
- Test: `internal/battery/source_windows_test.go`

**Interfaces:**
- Produces:
  - `powrprof = windows.NewLazySystemDLL("powrprof.dll")`, procs `PowerSettingRegisterNotification`, `PowerSettingUnregisterNotification`; `DEVICE_NOTIFY_CALLBACK = 2`; GUIDs `GUID_BATTERY_PERCENTAGE_REMAINING` {A7AD8041-B45A-4CAE-87A3-EECBB468A9E1} and `GUID_ACDC_POWER_SOURCE` {5D3E9A59-E9D5-4B00-A6BD-FF34FF516548} as `windows.GUID` values.
  - One callback for the process, made once with `windows.NewCallback` under a `sync.Once` (Go caps callbacks per process), dispatching to the current `changed` func held in an `atomic.Pointer[func()]`. The callback does nothing else (no read, no lock, no allocation) and returns 0.
  - `Watch(changed)`: `Read` first; not present → `nil, false` and nothing registered (Review Focus 1). Else registers both GUIDs with `DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS{Callback, Context: 0}`; any failure unregisters what was registered and returns `nil, false` (the UI then polls, Ruling 3). `stop` unregisters both, clears the pointer, and is safe to call twice.
  - `procRegister`/`procUnregister` are package vars so tests can fake them.
- [ ] **Step 1: Write the failing tests** (fakes for the two procs and for `Read`):
  - `TestWatchRegistersBothSettings`: present battery → 2 registrations with the two GUIDs; `stop()` → 2 unregistrations; `stop()` again → none more.
  - `TestWatchNotRegisteredWithoutBattery`: `Read` reports no battery → no registration, ok false.
  - `TestWatchFailureUnregistersAndFallsBack`: the second registration fails → the first is unregistered, ok false.
  - `TestCallbackCallsChanged`: calling the dispatcher directly runs `changed` once; after `stop`, not at all.
  - `TestWatchRealSmoke` (real procs, this machine): `Watch` then `stop` within `time.After(2s)`; passes whether or not a battery exists.
- [ ] **Step 2: Run** `go test ./internal/battery/ -run 'TestWatch|TestCallback'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/battery/` and `go test -race ./internal/battery/` → PASS.
- [ ] **Step 5: Commit** `feat(battery): Hear about battery changes from Windows instead of polling`

### Task B3 (Lane B): `batteryWork`: read on change, never on desktops, nothing while hidden

**Files:**
- Create: `internal/ui/battery.go`, `internal/ui/battery_test.go`
- Modify: `internal/ui/window.go` (`battery *batteryWork` field; `win.initBattery()` right after `win.initResources()`; `win.battery.close()` after `win.resources.close()`; `win.battery.pause(e.Config.Mode == app.Minimized)` after `win.resources.pause(...)` in `onConfig`)

**Interfaces:**
- Produces:
  - `const batteryEvery = time.Minute`.
  - `type batteryWork struct { src battery.Source; wake func(); state uint8 /* unchecked, none, watching, polling */; status battery.Status; readAt time.Time; dirty, paused, stopped atomic.Bool; stop func(); reads int /* tests */ }`.
  - `func newBatteryWork(src battery.Source, wake func()) *batteryWork`; `func (win *window) initBattery()`: none in bench mode (`win.bench != nil`, like `initResources`, so `KAZE_BENCH` frames are unchanged, Ruling 9), else `newBatteryWork(battery.Native(), win.w.Invalidate)`.
  - `func (w *batteryWork) tick(now time.Time) battery.Status` (nil-safe; called from `statusModel` on every frame, so it is two atomic loads and a time compare when nothing is due, and never allocates):
    - `unchecked` → one `Read`; not present (or an error) → `none` for good: no more reads, no `Watch` (Ruling 2). Present → `Watch(w.changed)`: ok → `watching`, else `polling`.
    - `watching` → reads only when `dirty.Swap(false)` and not paused.
    - `polling` → reads when not paused and `now.Sub(readAt) >= batteryEvery` (only on frames that happen anyway: no wake is scheduled, Ruling 3).
    - Returns the last status.
  - `func (w *batteryWork) changed()` (the OS thread's callback): `dirty.Store(true)`; `wake()` only when not paused and not stopped.
  - `pause(p bool)`: on a change to not paused, `dirty.Store(true)` so the first frame after restoring reads once. `close()`: `stopped`, then `stop()` once.
- [ ] **Step 1: Write the failing tests** with `type fakeBattery struct { st battery.Status; watchOK bool; reads, watches, stops int; changed func() }`:
  - `TestBatteryDesktopReadsOnce`: no battery → 1000 ticks over 2 h of fake time → `reads == 1`, `watches == 0`, status not present.
  - `TestBatteryNotifyWakesOnce`: watching; 50 ticks → 1 read; `changed()` → wake count 1; next tick reads (2), the 49 after it do not.
  - `TestBatteryPollsAtMostOncePerMinute`: `watchOK = false`; ticks every 2 s for 5 min → 1 + 5 reads.
  - `TestBatteryPausedReadsNothing`: paused; `changed()` → no wake; 100 ticks → no read; `pause(false)` → the next tick reads once.
  - `TestBatteryCloseStopsWatching`: `close()` twice → `stops == 1`; `changed()` after → no wake.
  - `TestBatteryTickAllocatesNothing`: watching, nothing due → `testing.AllocsPerRun(100, tick) == 0`.
  - `TestNoBatteryInBenchMode`: a bench window → `win.battery == nil` and `statusModel` still works.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBattery|TestNoBatteryInBench'` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Battery|Resource|Status'` and `go test -race ./internal/ui/ -run TestBattery` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Read the battery only when it changes, and never on a desktop`

### Task B4 (Lane B): The battery group: far right, dropped first

**Files:**
- Modify: `internal/ui/statusbar.go` (`grpBattery`, `statusFit.noBattery`, the first compaction step, the right-hand item after `grpTabs`), `internal/ui/statusbar_draw.go` (`case grpBattery`, new `c.batteryGlyph`, `c.batteryColor`), `internal/ui/status.go` (`statusModel.Battery battery.Status`, filled from `win.battery.tick(now)`), `internal/ui/chrome_cache.go` (`sameStatus` compares `Battery`)
- Test: `internal/ui/statusbar_test.go`, `internal/ui/battery_test.go`

**Interfaces:**
- Produces:
  - `grpBattery` appended at the end of the `statusGroup` block (after `grpNotice`) so existing values keep their numbers; not clickable, no hover highlight (Ruling 6).
  - Layout: when `m.Battery.Present && !f.noBattery`, an item after `grpTabs` with a hairline before it: `2*ip + d.Battery + gap/2 + measure("100%"-style text of m.Battery.Percent, true)` where `statusDims` gains `Battery int` (`gtx.Dp(22)`, height 12 dp per the mockup's 22×12 glyph). `func() { f.noBattery = true }` is the **first** step of `layoutStatusBar` (before `noTabs`).
  - Glyph (`batteryGlyph(gtx, x, mid, pct int, charging bool, col color.NRGBA)`, mockup `glyph()`): 18×10.5 dp outline, radius 2.6 dp, 1.3 dp stroke; tip 1.8×4.4 dp at x 19.6; fill `max(1.5, 15*pct/100)` dp wide, 7.2 dp tall, radius 1.4 dp; charging: the bolt path from the mockup in `th.Done` over a 1.2 dp `th.Chrome` outline. All paths are built from fixed points (no `clip.Path` growth past the ops, no slices).
  - Colours (`batteryColor(pct)`): `pct <= 10` → `UsageHigh`, `pct <= 20` → `Needs`, else outline and tip `StatusFG`, fill and text `statusValueColor()`; every one through `render.Readable(x, th.Chrome)` (Ruling 4). Text: `strconv.Itoa(pct)+"%"`, mono 11 sp like the tabs count.
  - Percent text measurement goes through the existing `statusLayoutCache`, so an unchanged battery adds nothing to a frame.
- [ ] **Step 1: Write the failing tests**:
  - `TestStatusBatteryFarRight` (`statusAt`-style with a 1400 px bar): the battery group is the rightmost, after `grpTabs`, with a separator between them.
  - `TestStatusDropsBatteryFirst`: extend `TestStatusDropOrder`'s walk with a battery → stages begin `"full", "no battery", "no tabs", ...`; the old order follows unchanged.
  - `TestStatusNoBatteryNoGroup`: `Present == false` → no `grpBattery`, the layout equals today's for the same model.
  - `TestStatusBatteryColors` (table 82/64 charging/18/7) → the colour the draw picks, and the bolt only when charging (draw-level check through a recorded `chrome` call list or `win.stat` texts plus a colour probe like `TestStatusInEveryTheme`).
  - `TestStatusBatteryReadableInEveryTheme`: for each `theme.All()`, text and glyph colours ≥ 4.5:1 on `Chrome` at 82, 18 and 7.
  - `TestStatusBatteryCacheHits`: same model twice → `layoutStatusBar` runs once (counter); percent 82 → 81 → runs again.
  - `TestStatusBatteryAddsNoAllocations`: `statusWindow` fixture, unchanged frames of `drawStatus` with and without a fake battery present → `testing.AllocsPerRun(50, ...)` equal.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestStatus'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (`TestStatusInEveryTheme`, `TestStatusTextReadableInEveryTheme`, `TestSidebarCache*` included).
- [ ] **Step 5: Commit** `feat(ui): Show the battery level at the far right of the status bar`

### Task B5 (Lane B): Battery captures and the idle check

**Files:**
- Create: `internal/ui/statusbar_render_test.go`, `docs/audits/battery-activity-names/battery-*.png`

- [ ] **Step 1:** `TestBatteryRenders`, gated by `KAZE_BATTERY_RENDER_DIR` (pattern of `TestDockRenders`: `headless.NewWindow`, `writeRender`), a 1400×120 strip with the status bar, themes Midnight, Sumi and Washi, states normal 82, charging 64, low 18, very low 7, and a 700 px narrow bar (battery dropped). Skipped without the env var.
- [ ] **Step 2:** Run it, open every PNG, compare with `docs/design/battery-status-mockup.html`, fix drift test-first.
- [ ] **Step 3:** Idle check by hand on this machine (not a test): build a release-style `kaze.exe` in a temp folder (never over the owner's copy), leave it visible and then minimized for 2 minutes each, and note Task Manager CPU (expect 0.0 %) in the audit draft.
- [ ] **Step 4: Commit** `test(ui): Capture the battery in the status bar`

---

## Lane R: readable agent activity (#134), Tasks R1 to R7

### Task R1 (Lane R): The transcript keeps each tool call's facts

**Files:**
- Modify: `internal/subagents/transcript.go` (`record` handles `user` records for `tool_result`; `tool_use` inputs; `Transcript` gains `Steps`, `seq`, `cwd`, `stepBytes`; `TranscriptMessage` gains `Seq uint64`)
- Create: `internal/subagents/transcript_steps_test.go`

**Interfaces:**
- Produces:
  - Caps: `MaxTranscriptSteps = 200`, `maxDiffLines = 24`, `maxOutLines = 8`, `maxDetailLine = 200` (runes per kept line), `maxStepDetail = 4 << 10` (bytes per step), `maxStepsDetail = 256 << 10` (bytes for all steps: past it, the oldest steps lose their detail and keep their sentence facts).
  - `type DiffLine struct { Kind byte /* ' ', '+', '-' */; Text string }`.
  - `type TranscriptStep struct { Seq uint64; At time.Time; ID, Tool string; Desc, Command, Path, Pattern, Query string; Added, Removed int; Diff []DiffLine; Files []FileCount /* MultiEdit: one; folds use several */; Out []string; Done, Failed bool; Fails int; Commit, Subject string; Background bool }` and `type FileCount struct{ Path string; Added, Removed int }`.
  - `record`: decodes once per line (as today) into an entry struct that also reads `cwd`, `message.content[].id`, `tool_use_id`, `is_error`, `content` (`json.RawMessage`: a string or `text` blocks) and the tool input fields `description, command, file_path, old_string, new_string, content, edits[{old_string,new_string}], pattern, path, url, query, run_in_background`. Every message and step gets `Seq = tr.seq++`, so record order interleaves them exactly (Ruling 21).
    - `tool_use` (not `SubagentHandback`): today's `Tools.add` stays; also appends a `TranscriptStep`. Paths are shown relative to the record's `cwd` when inside it, else the last 3 parts (`shortPath`, the rule of `cmd/kaze` `lastParts`). Edit: `Added/Removed` and `Diff` from `lineDiff(old, new)`: common leading and trailing lines trimmed, the middle is `-` then `+`, with up to 2 context lines on each side as `' '`; counts cover the whole text, `Diff` is capped. MultiEdit: summed over `edits`, `Diff` from the first edits until the cap. Write: `Added` = lines of `content`, `Removed = 0`, `Diff` = the first lines as `+`.
    - `user` record `tool_result`: finds the step by `ID` scanning back over at most the last 64 steps (no map, no allocation); sets `Done`, `Failed = is_error`, `Out` = the last `maxOutLines` non-empty lines (each cut to `maxDetailLine`), `Fails` = lines starting with `--- FAIL:` or `FAIL ` before a tab (Go), counted over the whole output; for a Bash `Command` containing `git commit`, `Commit, Subject` from the first output line matching `[<branch> <hash>] <subject>` (Ruling 23).
  - `Version` bumps on every step change, as for messages. A transcript replaced or truncated resets everything (today's `Read` rule).
- [ ] **Step 1: Write the failing tests** (local helpers `useLine(id, name string, input map[string]any)`, `resultLine(id string, isErr bool, content any)`, `userText(s)`):
  - `TestTranscriptStepsInRecordOrder`: text, Bash, text, Edit → `Messages[0].Seq < Steps[0].Seq < Messages[1].Seq < Steps[1].Seq`.
  - `TestTranscriptBashResult`: Bash with `description` "Run go vet and the ui tests" + an `is_error` result with `--- FAIL: TestX` → `Desc`, `Failed`, `Fails == 1`, `Out` ends with the FAIL line.
  - `TestTranscriptEditCounts`: old 5 lines, new 8 lines sharing 2 → `Added`, `Removed` as `+8 −5`'s example in the mockup; `Diff` starts with context; Write and MultiEdit cases.
  - `TestTranscriptCommitSubject`: `git commit -q -m "..."` with result `[feat/x c3f9a12] refactor(ui): Remove the scratch shell key` → `Commit == "c3f9a12"`, `Subject` set.
  - `TestTranscriptResultContentBlocks`: a result whose content is `[{type:text,text:...}]`.
  - `TestTranscriptPathRelativeToCwd`: `cwd` `C:\w\kaze`, file `C:\w\kaze\internal\ui\agent.go` → `internal/ui/agent.go`; outside cwd → last 3 parts.
  - `TestTranscriptStepCaps`: an Edit whose old and new text are 10 000 short lines each (the record stays under `MaxTranscriptLine`) → `len(Diff) <= maxDiffLines`, counts exact; a Bash result of 10 000 lines → `len(Out) <= 8`, each line ≤ 200 runes; 250 steps → 200 kept; total detail bytes ≤ `maxStepsDetail`.
  - `TestTranscriptUnmatchedResultIgnored`, and every existing `transcript_test.go` test unchanged.
- [ ] **Step 2: Run** `go test ./internal/subagents/ -run 'TestTranscriptSteps|TestTranscriptBash|TestTranscriptEdit|TestTranscriptCommit|TestTranscriptResult|TestTranscriptPath|TestTranscriptStepCaps|TestTranscriptUnmatched'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/subagents/` → PASS; `BenchmarkTranscriptRead` (new, 200-record fixture) allocations noted in the commit body.
- [ ] **Step 5: Commit** `feat(subagents): Keep each tool call's description, diff and result from the transcript`

### Task R2 (Lane R): One sentence per step, small steps folded

**Files:**
- Create: `internal/ui/dock_readable.go`, `internal/ui/dock_readable_test.go`

**Interfaces:**
- Produces:
  - `type readKind uint8` (`readNote, readStep, readDropped, readWaiting, readLive, readFinished`).
  - `type readItem struct { kind readKind; id string /* first step's ID: open state survives trimming */; at time.Time; verb, obj string /* "Edited", "internal/ui/agent.go" */; objMono bool; meta []readMeta; detail readDetail; text string /* notes and end rows */; msg int }` with `type readMeta struct{ text string; tone uint8 /* plain, add, del, ok, bad */ }` and `type readDetail struct { cap string /* "$ <command>" */; lines []DiffLine }`.
  - `func readableItems(a *subagents.Agent, tr *subagents.Transcript) []readItem`:
    - Source: the transcript's `Steps` and text `Messages` merged by `Seq` when `tr != nil && tr.Available && len(tr.Steps) > 0`; otherwise (Codex, no transcript) the hook steps `a.Steps` (`Tool`, `Summary`) and the transcript notes by time as `dockTimeline` does (Ruling 26).
    - Sentences (Ruling 22): Bash → `Desc` with a leading imperative turned past ("Run"→"Ran", "Check"→"Checked", "List"→"Listed", "Find"→"Found", "Build"→"Built", "Show"→"Showed", "Create"→"Created", "Install"→"Installed", "Remove"→"Removed", "Delete"→"Deleted", "Update"→"Updated", "Fetch"→"Fetched", "Read"→"Read", "Search"→"Searched", "Get"→"Got", "Make"→"Made", "Start"→"Started", "Stop"→"Stopped", "Test"→"Tested", "Print"→"Printed", "Count"→"Counted", "Copy"→"Copied", "Move"→"Moved", "Write"→"Wrote", "Open"→"Opened", "Merge"→"Merged", "Push"→"Pushed", "Commit"→"Committed"; no match: the description as written), else "Ran" + the command's first line in mono, cut to 120 runes; meta "failed · N tests" (`Fails > 0`), "failed", "passed" (a test command: `go test`, `npm test`, `pnpm test`, `yarn test`, `pytest`, `cargo test`, `jest`, `vitest`), "done", "started" (background), none while running. A commit → verb "Committed", obj `"<subject>"`, meta the hash in the ok tone. Edit/Write/MultiEdit → "Edited" + path, meta `+N` (add) `−M` (del). Read → "Read" + file. Grep/Glob → "Searched the code". WebFetch → "Fetched" + host. WebSearch → "Searched the web for" + query. TodoWrite → "Updated the to-do list". Other tools → "Used" + tool name. Hook-step fallback uses `Summary` as the object and has no meta.
    - Folding (Ruling 24): consecutive steps of one fold class merge, ignoring the plumbing tools `ToolSearch, Monitor, TaskStop, TaskOutput, BashOutput, KillShell` (never shown in this view, counted in the header): reads → "Read N files" (distinct paths; detail = the file list); searches → "Searched the code N times" (detail = `pattern in path` per search); edits → "Edited N files" with summed `+N −M` (detail = `path +a −r` per file) when 2 or more distinct files, else one file's sentence and diff; TodoWrite runs → one. Bash, commits, web and others never fold. A note between two steps ends a run.
    - Detail: an edit's capped `Diff`; Bash `cap = "$ " + Command` (first 200 runes) and `Out` as dim lines; a fold's list; none for TodoWrite.
    - End rows as `dockTimeline` (waiting text, "Finished in …", "working"); `a.Dropped` / `tr.Dropped` → one "… earlier steps hidden" row.
  - `func readableCounts(a, tr) (steps, messages int)` for the header ("21 steps · 3 messages").
- [ ] **Step 1: Write the failing tests** (fixtures built in memory as `*subagents.Transcript` with `Steps`/`Messages`; `Available: true`):
  - `TestReadableBashSentence` (table: "Run go vet and the ui tests" + Fails 1 → "Ran go vet and the ui tests", meta "failed · 1 test"; "List files" + Done → "Listed files", "done"; no `Desc` → "Ran" + `git status`; `go test ./...` passed → "passed"; background → "started"; still running → no meta).
  - `TestReadableCommit`: → `Committed "refactor(ui): Remove the scratch shell key"`, meta `c3f9a12`.
  - `TestReadableEditSentence`: `Edited internal/ui/scratch_gone_test.go`, meta `+8`, `−5`; detail lines from `Diff`.
  - `TestReadableFoldsReads`: Read a, ToolSearch, Read b, Read a → one item "Read 2 files", detail [a, b].
  - `TestReadableFoldsSearches` (4 → "Searched the code 4 times") and `TestReadableFoldsEdits` (3 files → "Edited 3 files" `+6 −41`; the same file twice → single-file sentence with summed counts).
  - `TestReadableNoteBreaksFold`: Read, note, Read → 3 items.
  - `TestReadablePlumbingHidden`: Monitor, TaskStop alone → no step items; counted by `readableCounts`.
  - `TestReadableHookFallback`: no transcript, `a.Steps` Bash "go test" / Read x / Read y → "Ran go test", "Read 2 files".
  - `TestReadableMessagesLead`: the notes come out in `Seq` order between steps with their text intact.
  - `TestReadableIDsStable`: dropping the first step from `Steps` keeps the remaining items' `id`s.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestReadable` → FAIL (undefined).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Readable|DockTimeline'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Turn an agent's steps into short folded sentences`

### Task R3 (Lane R): Diff colours in every theme

**Files:**
- Modify: `internal/theme/theme.go` (`Chrome` gains `DiffAddBG, DiffAddFG, DiffDelBG, DiffDelFG color.NRGBA` for all five themes), `internal/theme/theme_test.go`
- Create (in R4's file): `internal/ui/dock_readable_draw.go` `readInk`

**Interfaces:**
- Produces:
  - Values (Ruling 25): Midnight add `#122419`/`#8fe0b0`, del `#2a1418`/`#f3a0a8`; Sumi add `#16261a`/`#a9d79a`, del `#2c1513`/`#f0a093` (both from the mockup); Tide, Washi and Sakura picked in the same pattern (a 10-15 % tint of the theme's `Done` / `UsageHigh` over its `Chrome` for the background, the hue lightened for dark themes or darkened for Washi until ≥ 4.5:1).
  - `type readInk struct { say, sayBar, step, strong, mono, meta, add, del, ok, bad, detailBG, detailLine, dim, addBG, addFG, delBG, delFG color.NRGBA }` from `func (c *chrome) readInk(bg color.NRGBA) readInk`: `sayBar = agentLogoColor("claude", th.Accent)` (the coral bar of the mockup, Ruling 27), `say = ActiveFG`, `step = Muted`→`render.Readable`, `strong = mono = ActiveFG`, `meta = Muted`, `add = DiffAddFG`, `del = DiffDelFG`, `ok = Done`, `bad = UsageHigh`, `detailBG = th.Chrome`, `detailLine = Border`, `dim = Muted`; every text through `render.Readable` on its own surface; cached per theme and background like `boardInk`.
- [ ] **Step 1: Write the failing tests**: `TestDiffTokensInEveryTheme` (`theme_test.go`: none zero; `DiffAddFG` on `DiffAddBG` and `DiffDelFG` on `DiffDelBG` ≥ 4.5 with the raw tokens, before any nudge); `TestReadInkContrast` (`dock_readable_draw_test.go`): for each `theme.All()`, every text field on the panel background, `dim`/`mono` on `detailBG`, `addFG` on `addBG`, `delFG` on `delBG` ≥ 4.5.
- [ ] **Step 2: Run** `go test ./internal/theme/ ./internal/ui/ -run 'TestDiffTokens|TestReadInk'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/theme/ ./internal/ui/ -run 'Theme|Contrast|Ink'` → PASS.
- [ ] **Step 5: Commit** `feat(theme): Add diff colours to every theme`

### Task R4 (Lane R): Draw the readable Activity, built once per change

**Files:**
- Modify: `internal/ui/dock.go` (`dockSession` gains `read readCache; open map[string]bool`), `internal/ui/dock_draw.go` (`drawDockActivity`: a mode row, then `drawReadable` or today's list; `dockHits` gains `mode, modeRead, modeEvery image.Rectangle; steps []readHit`), `internal/ui/window.go` (`actEvery bool`: the "Every step" choice)
- Create: `internal/ui/dock_readable_draw.go`, `internal/ui/dock_readable_draw_test.go`

**Interfaces:**
- Produces:
  - `type readCache struct { ver uint64; nSteps, dropped int; state subagents.State; trOK bool; items []readItem; steps, msgs int; builds int /* tests */ }` and `func (s *dockSession) readable(a, tr) []readItem`: rebuilds only when `tr.Version`, `len(a.Steps)`, `a.Dropped`, `a.State` or availability changed; reuses `items[:0]` (Ruling 20). The "Every step" list (`dockTimeline`) gets the same cache key in `s.read` so it too stops being rebuilt every frame.
  - Mode row (first row of the Activity body, 26 dp, mockup `.tools`): left "ACTIVITY" 11 sp SemiBold `meta` + "21 steps · 3 messages" 12 sp `meta`; right a two-button segment "Readable" | "Every step" (the dock's Activity/Report segment style: `tint(th.Border, 0x80)` track, `th.Chrome` on the chosen one). Shared by the dock and the board preview because both call `drawDockActivity`.
  - `drawReadable(gtx, s, a, inner, ink, h, texts)`: notes (mockup `.say`): a 2 dp `sayBar` at the left, markdown through the existing `mdLayout` / `s.cache.notes` (13.5 sp equivalent of `panelStyle`), time `15:04` 11 sp mono `meta` at the top right. Steps (mockup `.step`): 16 dp chevron column ("›" in `dim`, drawn rotated ▾-style as "⌄" when open; none when there is no detail), verb 13 sp `step`, object bold `strong` (or 12 sp mono `mono` for paths and commands), meta at the right 11 sp mono in its tone; cut with `fitSide` to the row. Open detail (mockup `.more`): inset 24 dp, radius 8 dp, `detailBG` with a 1 px `detailLine`, an optional caption row "$ command" 11 sp `dim` under a 1 px line, then lines 12 sp mono, each one line cut with "…" (`fitSide`), `+ `/`− ` prefixes on `addBG`/`delBG` with `addFG`/`delFG`, context and output in `dim`. Heights per item are computed per frame from the cached items (row 24 dp, detail `12 + lines*18` dp); only rows in view are drawn, as today.
  - `type readHit struct { id string; rect image.Rectangle }` appended for every drawn step that has detail.
  - `win.actEvery == false` (default) draws the readable view; `true` draws today's `dockTimeline` rows unchanged below the mode row (Ruling 19).
- [ ] **Step 1: Write the failing tests** (`dockOne` / `previewScene` fixtures with a fake transcript file written under a temp main transcript so `dockTick` reads it):
  - `TestDockShowsReadableSteps`: an agent whose transcript has a note, Read ×3, an Edit and a failed Bash → `win.dock.texts` contain the note's words, "Read 3 files", "Edited", "+8", "−5", "Ran go vet and the ui tests", "failed · 1 test"; never "Bash".
  - `TestDockEveryStepShowsTodaysList`: `win.actEvery = true` → texts contain `Bash` and the raw command (today's rows).
  - `TestBoardPreviewShowsReadableSteps`: the same transcript in the board preview → the same sentences in `b.texts`.
  - `TestReadableBuiltOncePerChange`: 20 frames with the transcript unchanged → `s.read.builds == 1`; append a record and advance past `agentTranscriptPoll` → 2.
  - `TestReadableHeaderCounts`: texts contain "5 steps · 1 message" for the fixture (singular rule).
  - `TestHiddenPanelBuildsNothing`: hide the panel, 5 frames over 3 s of fake time → `builds` unchanged and `trAt` unchanged (with `TestDockHiddenAgentReadsNothing`).
  - `TestReadableLongDiffStaysCapped`: a 20 000-line edit fixture → the open detail draws at most `maxDiffLines` lines (`texts` count).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDockShowsReadable|TestDockEveryStep|TestBoardPreviewShowsReadable|TestReadableBuilt|TestReadableHeader|TestHiddenPanelBuilds|TestReadableLongDiff'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (`TestDock*`, `TestBoard*`, `TestSidebarCache*`).
- [ ] **Step 5: Commit** `feat(ui): Show an agent's activity as readable steps in the panel and the board`

### Task R5 (Lane R): Click a step for its detail, switch Readable and Every step

**Files:**
- Modify: `internal/ui/dock_input.go` (`dockBody`: a press on `hits.modeRead`/`modeEvery` sets `win.actEvery`; a press on a `readHit` toggles `s.open[id]`), `internal/ui/board.go` (the preview's press branch in the board event loop: the same two checks against `b.prevHits` and `b.prev`), `internal/ui/board_preview.go` (keeps `b.prevHits` including the new hits)
- Test: `internal/ui/dock_readable_draw_test.go`

**Interfaces:**
- Produces: `func (win *window) readPress(s *dockSession, h *dockHits, p image.Point) bool` shared by the dock and the board; true when it handled the press (then the body's other press handling, link Ctrl+click and "read the report", is skipped). Toggling invalidates once; `s.open` entries whose `id` is no longer in the items are dropped at the next rebuild (bounded by the 200-step cap). The mode is one window-wide choice for the dock and the board, not saved to the config (Ruling 19). Scrolling, Ctrl+click links in notes and `follow` behave as today; opening a detail while `follow` keeps the newest row in view.
- [ ] **Step 1: Write the failing tests** (presses queued through `clickAt` on an `input.Router` or `dockBody` with a `pointer.Event`; no global input):
  - `TestClickStepOpensDiff`: click the "Edited" row → the next frame's texts contain the diff's `+ ` and `− ` lines; click again → gone.
  - `TestClickBashShowsCommandAndOutput`: → "$ cd kaze-lane-d && …" and the last FAIL line.
  - `TestClickFoldShowsFiles`: "Read 3 files" → the three paths.
  - `TestStepWithoutDetailIgnoresClick`: TodoWrite row → no change, `redraws` unchanged.
  - `TestModeToggleSwitchesBothViews`: click "Every step" in the dock → `win.actEvery`; the board preview then draws today's list too; click "Readable" → back.
  - `TestBoardPreviewClickOpensStep`: the same toggle in the board preview.
  - `TestOpenStepSurvivesNewRecords`: open a step, append records → still open.
  - `TestNoteLinksStillOpen`: Ctrl+click a `.md` link in a note still calls `openDocRef` (port of the dock link test).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestClickStep|TestClickBash|TestClickFold|TestStepWithout|TestModeToggle|TestBoardPreviewClick|TestOpenStep|TestNoteLinks'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Open a step's diff or output with a click`

### Task R6 (Lane R): Codex and agents without a transcript

**Files:**
- Modify: `internal/ui/dock_readable.go` (fallback path only, if R2 left gaps), `internal/ui/dock_readable_draw.go`
- Test: `internal/ui/dock_readable_draw_test.go`

**Interfaces:**
- Consumes: hook steps from `cmd/kaze/agents.go` `toolSummary` (unchanged: Bash first line, file `lastParts(…, 3)`, Grep pattern, `apply_patch`'s first file).
- Produces: in the fallback, `apply_patch` reads "Edited <file>" (no counts), Bash "Ran <first line>" in mono, no pass/fail meta and no detail chevrons except for folds (their list). The header counts hook steps.
- [ ] **Step 1: Write the failing tests**: `TestCodexHelperReadableFallback` (a Codex helper with `apply_patch`, Bash and two Reads → "Edited internal/ui/x.go", "Ran go test ./...", "Read 2 files", no "passed"); `TestNoTranscriptStillShowsNotes` (no transcript path → the steps only, no crash, "working" row last).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestCodexHelperReadable|TestNoTranscriptStill'` → FAIL (or PASS if R2 covered it: then the commit is test-only and says so).
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Keep readable steps for Codex helpers from their hook summaries`

### Task R7 (Lane R): Captures and frame timing

**Before this task:** merge `feat/battery-activity-names` (Lanes N and B merged) into the Lane R worktree and run `go test ./...`.

**Files:**
- Modify: `internal/ui/dock_render_test.go` (`TestDockTabsRender` gains readable states; `TestDockTabsFrameTiming` gains a readable case), `internal/ui/board_render_test.go` (a preview state)
- Create: `docs/audits/battery-activity-names/activity-*.png`

- [ ] **Step 1:** Opt-in captures (the existing `KAZE_DOCK_RENDER_DIR` gate and the board's render gate), Midnight and Sumi at 1400×860: `activity-<theme>-dock.png` (running agent, notes, folds, a failed Bash), `activity-<theme>-open.png` (the Edit detail open), `activity-<theme>-every.png` (Every step), `activity-<theme>-board.png` (board preview). Compare with `docs/design/readable-activity-mockup.html`; fix drift test-first.
- [ ] **Step 2:** `TestDockTabsFrameTiming` (`KAZE_DOCK_BENCH=1`): log ns and allocations per still frame for the agent tab in Readable, Every step and with one detail open, on the same 21-step fixture. Readable must allocate no more per frame than Every step does on `main` (measure `main` in a `git worktree add ../kaze-before main` copy with the same test).
- [ ] **Step 3: Commit** `test(ui): Capture the readable activity and time its frames`

---

### Task F1 (after all lanes): Merge check, audit, README, lander, HANDOFF

**Files:**
- Create: `docs/audits/2026-11-04-battery-activity-names.md`
- Modify: `README.md`, `docs/index.html`, `docs/HANDOFF.md`, `docs/site-assets/SCREENSHOTS.md`

- [ ] **Step 1: Full check** on `feat/battery-activity-names` after the three merges: `go test ./...`, `go vet ./...`, `go test -race ./internal/ui/ ./internal/subagents/ ./internal/battery/`, `GOOS=darwin go vet ./internal/battery/ ./internal/subagents/` (pure Go only; `./internal/ui` needs cgo on darwin and is left to CI), Windows build `go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze` in a temp folder (never over the owner's copy). Push the branch and open the PR only when the coordinator says so; the macOS CI job on the PR is the macOS check.
- [ ] **Step 2: README:** "## The status bar": the battery at the far right on Windows laptops (icon fills to the level, percent, a green bolt while charging, amber at 20 % or less, red at 10 % or less; nothing on desktops; dropped first in a narrow window; updated when Windows reports a change, no polling timer; **not yet on macOS**). "## Subagents": the readable Activity (the agent's words first, one sentence per step, folded reads/searches/edits, click a step for the diff or the command and its last lines, "Every step" for the full list, same in the board preview; Codex helpers from hook summaries); agents started together keep their own names. Roadmap: add the three items, with "Battery on macOS" as a follow-up.
- [ ] **Step 3: Lander `docs/index.html`, feature cards only:** "The status bar." card (`#f-status`) mentions the battery on Windows laptops; "Every subagent" card (`#f-agents`) says the panel reads like Claude's own transcript (words first, a sentence per step, click for the diff). Do **not** touch `#changelog`, the release markup or "What's new" (the release PR does). Record in `docs/site-assets/SCREENSHOTS.md` that `kaze-agents.png` and `kaze-board.png` show the old step list and need retaking, and that a battery shot needs a real Windows laptop; do not fake them.
- [ ] **Step 4: HANDOFF:** a dated "Where things stand" entry at the top (built on `feat/battery-activity-names`, release 0.28 next, screenshots to retake, owner checks); "Next": the macOS battery source (follow-up issue: IOKit `IOPSCopyPowerSourcesInfo` and `IOPSNotificationCreateRunLoopSource` in `internal/battery/source_darwin.go`, narrowing `source_other.go` to `!windows && !darwin`, built and checked on the MacBook); "Mac milestone checklist": a line for the battery; "Known gaps": no battery on macOS yet, no battery tooltip (Ruling 7), the 2 KiB prompt-key limit (Ruling 17).
- [ ] **Step 5: Audit** `docs/audits/2026-11-04-battery-activity-names.md`: per lane what changed, the tests per task with commits, the battery read cost on Windows (B1 benchmark) and the idle CPU note (B5), the transcript benchmark (R1), the readable frame numbers (R7), the contrast minimum per theme (B4, R3), the captures labelled as development captures, the draft text of the macOS follow-up issue (the owner files it), and the rulings below with any the owner changed.
- [ ] **Step 6: Commit** `docs: Record the battery, readable activity and agent names`

## Rulings made while planning

Each is cheap to overturn in its own task.

1. **Windows only for 0.28** (owner, 2026-11-04). `internal/battery` builds everywhere; every non-Windows OS, macOS included, reports "no battery" from pure Go (`source_other.go`, `//go:build !windows`), so nothing shows. The macOS IOKit source is a follow-up issue the owner builds on the MacBook; adding `source_darwin.go` narrows the tag to `!windows && !darwin`. No cgo anywhere in this plan. (B1, F1)
2. **A desktop is decided once:** the first read finding no battery (or failing) turns the battery off for the life of the window: no notifications, no more reads. A battery plugged in later (rare) shows after a restart. (B3)
3. **Change notifications first, a lazy poll second:** Windows `PowerSettingRegisterNotification` with a callback for battery percentage and AC/DC source; if that fails, a read at most every 60 s on frames that happen anyway, with no wake scheduled for it. An idle window with a failed registration may show a stale percent until something redraws; accepted (no timer). (B2, B3)
4. **Colours:** normal uses `StatusFG` outline and the status value colour for fill and text; 20 % or less `Needs`; 10 % or less `UsageHigh`; the bolt `Done`, as the mockup; all nudged to 4.5:1 on `Chrome`. (B4)
5. **"Charging"** is Windows' charging flag, or on AC below 100 %; full on AC shows no bolt. (B1)
6. **Not clickable:** the battery group has no hover highlight and no click. (B4)
7. **No tooltip in 0.28:** the issue's "nice to have" time-remaining tooltip is left out to keep the bar unchanged in cost; recorded in Known gaps. (F1)
8. **Placement and drop:** far right after the tab count with a hairline, and the first group dropped (a new first compaction step, before the tab count). (B4)
9. **No battery in bench mode** (`KAZE_BENCH`), like the resource sampler, so bench frames are unchanged. (B3)
10. **Minimized pauses:** a notification while minimized only marks the value dirty; the first frame after restoring reads once. (B3)
11. **Percent only, no decimals;** values above 100 are clamped. (B1)
12. **Prompt matching is in the tracker, file reading in the UI:** `internal/subagents` stays clock-free and the tracker reads no file; `FirstPrompt` is a separate bounded helper the UI calls. (N1, N2, N3)
13. **Only contested pairings are checked:** two or more candidate calls of the type (or untyped) waiting when an agent paired, or the type already contested in that queue since the owner's last prompt. The common one-call case opens no file. (N1, N3)
14. **When to read the prompt:** on the agent's tool or stop events (the transcript exists by then), at most 3 times, 64 KiB per read. (N2, N3)
15. **A contested type stays contested** for every later agent of that type in the same queue until the owner's next prompt or a new session, so the last agent (which got the leftover call) is still checked. (N1)
16. **No transcript or no match keeps today's pairing;** after 3 empty reads the agent is left as paired. (N1, N3)
17. **Prompt key:** CRLF folded, `notify.CleanReport`, trimmed, first 2 KiB. Two prompts identical for their first 2 KiB are treated as the same; then the order pairing stands (they are effectively the same task). Recorded in Known gaps. (N1)
18. **Swaps only between unsettled agents** of the same source tab and session, or with a still-queued call; a settled name is never taken. (N1)
19. **"Every step" is one window-wide switch** for the dock and the board preview, default Readable, not saved in `config.toml` (cheap to persist later if the owner wants). (R4, R5)
20. **Built once per change:** the readable list (and today's list) is rebuilt only when the transcript's `Version`, the hook step count, the dropped count, the agent's state or the transcript's availability changes. (R4)
21. **Order from the file:** transcript messages and steps carry a record sequence number and interleave by it, not by timestamps. (R1)
22. **Bash sentence:** Claude's `description` with a leading imperative turned past tense from a fixed table (description as written when the verb is not in it); no description → "Ran" plus the command's first line in mono. Meta: "failed · N tests" (Go `--- FAIL:` lines), "failed", "passed" for test commands, "done" for others, "started" for background runs, nothing while running. (R2)
23. **Commits** are recognised from a Bash `git commit` whose output has `[<branch> <hash>] <subject>`; otherwise it reads as a normal Bash step. (R1, R2)
24. **Folding:** only consecutive steps of one class fold (reads by distinct file, searches, edits across 2 or more files, to-do updates); plumbing tools (`ToolSearch`, `Monitor`, `TaskStop`, `TaskOutput`, `BashOutput`, `KillShell`) are hidden in Readable and do not break a run; a note ends a run; Bash, commits and web steps never fold. (R2)
25. **Diff tokens** are new theme fields in all five themes, Midnight and Sumi taken from the mockup, the others picked to the same pattern, each foreground ≥ 4.5:1 on its own background. (R3)
26. **Fallback source:** with no transcript (Codex, or a session whose transcript Kaze never learned), the readable view is built from the hook steps (`toolSummary`), with sentences but no counts, results or diffs. (R2, R6)
27. **The note bar is Claude's coral** (`claudeCoral`, as the mockup), the step rows stay in the theme's muted and foreground colours. (R3, R4)
28. **Caps:** 200 transcript steps; per step 24 diff lines, 8 output lines, 200 runes a line, 4 KiB; 256 KiB of detail across all steps (the oldest lose their detail first). (R1)
29. **No shared first task and no shared code file between lanes** except one struct field each for B and R in `window.go`; merge order N, B, R puts the largest lane last. (all)
