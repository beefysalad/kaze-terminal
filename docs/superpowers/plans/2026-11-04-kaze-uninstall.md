# Uninstall Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Settings > Uninstall page that previews and then removes every Kaze hook in Claude Code and Codex, Kaze's status line, Kaze's settings and state folder and (macOS) its chime cache, then quits with instructions to delete the app.

**Architecture:** Pure "remove Kaze's entries" functions beside the existing `Fix` in `claudesetup` and `codexsetup`; a small `internal/uninstall` package that builds the preview and runs the removal from one list (so the preview can't drift from what runs); a one-way "stop writing state" switch in the window; a new settings section that draws the preview, the confirm dialog and the result, and quits.

**Tech Stack:** Go 1.27, Gio, existing `orderedjson`, `BurntSushi/toml`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-11-04-kaze-uninstall-design.md` (approved by the owner 2026-11-04; read it first).

## Global Constraints

- Strict TDD: every test seen failing (RED) before the code that passes it. RED tests bounded: temp dirs, small inputs, 1s waits.
- Tests use temp dirs for home, config, cache and Codex paths. Never read or write the real `~/.claude`, `~/.codex`, `%APPDATA%\kaze`, `~/Library/...` or `C:\Users\john\AppData\Local\Programs\Kaze`. Never inject global input; no GUI launch except headless captures.
- "Kaze's" means what the existing recognizers say: Claude `kazeCommand` / `isKaze` (`internal/claudesetup/setup.go:155-178`), Codex `own` (`internal/codexsetup/setup.go:137`). Any Kaze copy counts (exe base name `kaze` / `kaze.exe`), OK or Broken. Remove **every** match, duplicates included.
- Never edit Codex `config.toml`. Never delete `.claude` / `.codex` folders, shared settings files, or any `*.bak-*` backup. A file with no Kaze content left is written, not deleted.
- Shared files are written only through `claudesetup.Write` (backup first, never overwrites a backup, atomic rename, follows a symlink). An unparseable or wrongly shaped file is never written.
- Kaze's own folder is `filepath.Dir(config.Path())`; the macOS cache is `<os.UserCacheDir()>/kaze`. Refuse to delete a folder whose base name isn't `kaze`, or that is empty, a volume root, or the home folder.
- Cancel changes nothing. Opening the page changes nothing. The "stop writing state" switch is one-way and starts only after Remove is pressed.
- Nothing per frame: the page reads files when it opens and after a run, never while drawing. File work runs off the UI goroutine, then invalidates.
- Copy: plain, short, no em dashes.
- Commits `type(scope): Verb ...`, **no AI attribution** of any kind. Never push to `main`.
- Windows: if Smart App Control blocks a test binary, rerun with `-ldflags "-B 0x<random hex>"`.

## Review Focus

1. **A Kaze hook in the same group as the person's own hook** (one `hooks` list holding both): only Kaze's handler goes, the group and the other handler stay. (Task 1 `TestRemoveKeepsNeighbourInGroup`; Task 2 the same for Codex.)
2. **Pressing Remove a second time, or reopening the page after a partial failure**: nothing is listed that is already gone, and a second run is a no-op that reports success for what's left. (Task 3 `TestRunTwice`.)
3. **The folder delete fails on Windows because a file is open** (a lock or log handle): the hooks are still removed, the failure names the folder and the manual step, and the result view shows both. (Task 3 `TestRunReportsFolderFailure` with a read-only / unremovable stand-in; Task 4 proves the session lock is released before the delete.)
4. **Kaze writes state during quit** (session flush at `shutdown`, a setting save from a late frame, a worktree record prune): the folder must not come back. (Task 5 `TestNoWriteAfterUninstall`.)
5. **Another Kaze window is open**: pressing Remove removes nothing and says to close it. (Task 4 `TestBusyWhenAnotherOwnerHoldsLock`; Task 6 `TestRemoveRefusedWhenBusy`.)

---

### Task 1: `claudesetup.Remove`

**Files:**
- Modify: `internal/claudesetup/setup.go` (beside `Fix` and `Replace`)
- Test: `internal/claudesetup/remove_test.go`

**Interfaces:**
- Produces: `func Remove(settings []byte) (out []byte, removed []string, err error)`. `removed` is one plain line per removed entry, e.g. `"Stop hook"`, `"PreToolUse hook"`, `"Status line"` (one per handler; duplicates give two lines). When nothing is Kaze's: `out` is the input unchanged and `removed` is empty. When `settings` is nil (file absent): `nil, nil, nil`. When `load` fails: `nil, nil, err`.

- [ ] **Step 1: Write the failing tests** in `remove_test.go`:
  - `TestRemoveAllKazeHooks`: settings with Kaze hooks for Stop, Notification, UserPromptSubmit, SubagentStart, SubagentStop, PreToolUse, SessionStart (built with the package's own `command(exe, args)`) → `out` has no `hooks` key (every list emptied, so groups, events and `hooks` all go); `removed` has 7 lines.
  - `TestRemoveKeepsNeighbourInGroup`: one Stop group holding a Kaze handler and `{"type":"command","command":"say done"}` → the group stays with only `say done`.
  - `TestRemoveKeepsOtherEventsAndKeys`: a `PostToolUse` group of another tool, `"model": "x"`, `"permissions": {...}` → all present in `out` exactly as `Encode` writes them.
  - `TestRemoveDuplicatesAndOtherCopies`: two PreToolUse Kaze handlers, one at `C:\old\kaze.exe` and one at `/Applications/Kaze.app/Contents/MacOS/kaze` → both removed.
  - `TestRemoveStatusLine`: a Kaze `statusLine` → key gone, `"Status line"` in `removed`; a `statusLine` running `ccusage` → kept, not in `removed`.
  - `TestRemoveNothingToDo`: no Kaze content → `bytes.Equal(out, in)`, `len(removed) == 0`.
  - `TestRemoveMalformed`: `{"hooks": []}` and `not json` → `err != nil`, `out == nil`.

- [ ] **Step 2: Run to see them fail.** `go test ./internal/claudesetup/ -run TestRemove -v` → FAIL (`Remove` undefined).

- [ ] **Step 3: Implement `Remove`.** Walk every event key under `hooks` (not only the seven Kaze writes); drop a handler when its `command` string matches `kazeCommand(cmd, a)` for any Kaze hook args (`argsDone`, `argsNeeds`, `argsPrompt`, `argsSession`, `argsTool`, `argsAgentStart`, `argsAgentStop`, and any other `args*` hook constant in the file). Then prune empty `hooks` lists, empty groups, empty events, an empty `hooks` object. Drop `statusLine` when `kazeCommand(cmd, argsStatus)` matches. Return the input bytes untouched when nothing was removed.

- [ ] **Step 4: Run to see them pass.** `go test ./internal/claudesetup/ -v` → PASS, existing tests included.

- [ ] **Step 5: Commit** `feat(claudesetup): Remove Kaze's hooks and status line`

---

### Task 2: `codexsetup.Remove` and the config.toml list

**Files:**
- Modify: `internal/codexsetup/setup.go`
- Test: `internal/codexsetup/remove_test.go`

**Interfaces:**
- Produces:
  - `func Remove(data []byte) (out []byte, removed []string, err error)`: same contract as Task 1 for `hooks.json` (lines like `"PreToolUse hook"`).
  - `type InlineHook struct{ Event, Command string }`
  - `func InlineKaze(config []byte) ([]InlineHook, error)`: every Kaze handler in `config.toml` (via the existing `inline` parser and `own`), in file order; nil config → nil, nil.

- [ ] **Step 1: Write the failing tests**:
  - `TestRemoveCodexHooks`: hooks.json with Kaze handlers for every event in `events` (commands like `"<exe>" notify tool --agent codex`) plus another tool's PreToolUse handler in the same group → only Kaze's go, the other stays; 8 lines in `removed`.
  - `TestRemoveCodexMultiple`: two Kaze PreToolUse handlers (the "multiple hooks" case `Fix` refuses) → both removed.
  - `TestRemoveCodexNothingAndMalformed`: no Kaze → input unchanged; duplicate keys or non-object → error, `out == nil`.
  - `TestInlineKaze`: a config.toml with an inline Kaze `Stop` hook and an unrelated hook → `[]InlineHook{{"Stop", "<that command>"}}`; the function never returns or writes TOML.

- [ ] **Step 2: Run to see them fail.** `go test ./internal/codexsetup/ -run 'TestRemoveCodex|TestInlineKaze' -v` → FAIL.

- [ ] **Step 3: Implement**, reusing `load`, `inline`, `own` (any kind recognized counts) and the same pruning as Task 1 (share nothing across packages; a small local helper is fine).

- [ ] **Step 4: Run to see them pass.** `go test ./internal/codexsetup/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(codexsetup): Remove Kaze's hooks from hooks.json and list inline ones`

---

### Task 3: `internal/uninstall`: one list for the preview and the run

**Files:**
- Create: `internal/uninstall/uninstall.go`
- Test: `internal/uninstall/uninstall_test.go`

**Interfaces:**
- Consumes: `claudesetup.Remove`, `claudesetup.Write`, `claudesetup.Relayout`, `codexsetup.Remove`, `codexsetup.InlineKaze`, `branchsession.Record`.
- Produces:
  ```go
  type Paths struct {
      ClaudeSettings string // ~/.claude/settings.json; "" skips
      CodexDir       string // ~/.codex; "" skips
      ConfigDir      string // filepath.Dir(config.Path())
      CacheDir       string // macOS <UserCacheDir>/kaze; "" elsewhere
      Home           string // for the folder guard
  }
  type Step struct {
      Where string   // "Claude Code", "Codex", "Kaze settings", "Cached sounds"
      Path  string
      Lines []string // what goes: "Stop hook", "Status line", "config.toml (preferences)", ...
  }
  type Plan struct {
      Steps        []Step
      StatusBackup string            // newest settings.json.bak-* in ~/.claude when a Kaze status line goes; "" otherwise
      Relayout     bool              // claudesetup.Relayout said the file will be reformatted
      Inline       []codexsetup.InlineHook // left alone, listed with CodexDir/config.toml
      Worktrees    []branchsession.Record  // registry entries whose Folder exists
      Unreadable   []Failure         // shared files that can't be parsed: left as is
  }
  type Failure struct{ Path, Err, Manual string }
  type Result struct{ Done []Step; Failed []Failure }
  func Inspect(p Paths) Plan
  func Run(p Paths, now time.Time) Result
  func (p Plan) Empty() bool // no Steps
  ```
  `Run` calls `Inspect` itself and acts on exactly its `Steps`, in order: Claude, Codex, Kaze settings folder, cached sounds. Worktrees are read from `<ConfigDir>/worktrees.json` with `branchsession.Store{Path: ...}.Records()`.

- [ ] **Step 1: Write the failing tests** (every path under `t.TempDir()`):
  - `TestInspectListsEverything`: fake Claude settings with Kaze hooks and status line, fake Codex `hooks.json` with Kaze hooks and `config.toml` with one inline Kaze hook, a config dir `<tmp>/cfg/kaze` holding `config.toml`, `workspaces.toml`, `sessions.json`, `worktrees.json` (one record whose folder exists, one whose folder doesn't), a cache dir `<tmp>/cache/kaze` → 4 steps in order; the settings step's lines name the four files with what they hold (`"config.toml (preferences)"`, `"workspaces.toml (workspace recipes)"`, `"sessions.json (tabs restored at start)"`, `"worktrees.json (which worktrees Kaze made)"`); `Inline` has 1; `Worktrees` has only the existing one; `StatusBackup` is the newest of two `settings.json.bak-*` files created in the test.
  - `TestInspectNothing`: all paths absent → `Empty()`, no failures.
  - `TestRunRemovesAndKeeps`: after `Run` on the first setup: Claude and Codex files have no Kaze content and their other content intact; the pre-existing `.bak-*` files are still there and one new backup exists for each; `config.toml` of Codex is byte-identical; the config and cache dirs are gone; the worktree folder still exists; `Result.Failed` empty.
  - `TestRunTwice`: a second `Run` → no steps, no failures, nothing changes on disk.
  - `TestRunUnreadableLeftAsIs`: Claude settings `not json` → file byte-identical, one `Unreadable` / `Failed` entry naming the path with a manual step ("Remove the lines that run kaze yourself."), and the Codex and folder steps still run.
  - `TestFolderGuard`: `ConfigDir` set to the home dir, to `<tmp>/notkaze`, to `""`, to a volume root → not removed, a `Failure` each (where it was a step).
  - `TestRunReportsFolderFailure`: make the config dir unremovable (on Windows hold an open handle to a file inside it with `os.Open`; elsewhere `chmod 0500` on the dir, skipping when running as root) → hooks still removed, one `Failure` with the folder path and `Manual` saying to delete it by hand. Close the handle / restore perms in `t.Cleanup`.

- [ ] **Step 2: Run to see them fail.** `go test ./internal/uninstall/ -v` → FAIL (package missing).

- [ ] **Step 3: Implement.** Folder removal is `os.RemoveAll` after the guard; on error, `Failure{Path, err.Error(), "Delete this folder yourself: " + path}`. Absent files and folders are not steps and not failures.

- [ ] **Step 4: Run to see them pass.** `go test ./internal/uninstall/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(uninstall): Preview and remove Kaze's hooks and files from one list`

---

### Task 4: Session store: stop without saving, and a busy probe

**Files:**
- Modify: `internal/sessionstate/state.go` (beside `Close` / `CloseKeepingOwnership` :513-535)
- Test: `internal/sessionstate/abandon_test.go`

**Interfaces:**
- Produces:
  - `func (s *Store) Abandon() error`: stops the worker without writing (pending changes are dropped), releases and closes the lock file. Safe after `Freeze`; a second call or a call after `Close` returns nil.
  - `func Busy(path string) (bool, error)`: opens `path+".lock"` (without creating the parent folder; a missing lock or folder is `false, nil`), tries the lock, releases it at once. `true` when another owner holds it (`ErrOwned`).

- [ ] **Step 1: Write the failing tests**:
  - `TestAbandonWritesNothing`: `Open` in a temp dir, `SetTabs` with one tab, `Abandon()` → `sessions.json` doesn't exist (or is byte-identical to before when it existed); `Open` on the same path afterwards succeeds (lock released).
  - `TestAbandonTwice`: second `Abandon` and a `Close` after it return nil, no panic.
  - `TestBusyWhenAnotherOwnerHoldsLock`: store A `Open`ed → `Busy(path)` is `true`; after `A.Abandon()` → `false`.
  - `TestBusyMissing`: path in a folder that doesn't exist → `false, nil`, and the folder still doesn't exist.

- [ ] **Step 2: Run to see them fail.** `go test ./internal/sessionstate/ -run 'TestAbandon|TestBusy' -v` → FAIL.

- [ ] **Step 3: Implement.** `Abandon` sends the worker a stop request flagged to skip the final write (extend `request` with a `discard bool`), waits for `done`, then `unlockFile` and `Close` on the lock. `Busy` must not use `os.O_CREATE` on a missing folder; creating the lock file itself when the folder exists is acceptable only if it's removed again in the same call (prefer opening without create and treating "not exist" as `false`).

- [ ] **Step 4: Run to see them pass.** `go test ./internal/sessionstate/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(sessionstate): Stop without saving and tell when another Kaze owns the sessions`

---

### Task 5: The window's one-way "stop writing state" switch

**Files:**
- Create: `internal/ui/uninstall_state.go`
- Modify: `internal/ui/settings_workspaces.go` (the two `workspaces.Save` calls near :852 and :894: refuse with no write when `win.cfgPath == ""`), and any other writer found in step 3
- Test: `internal/ui/uninstall_state_test.go`

**Interfaces:**
- Consumes: `sessionstate.(*Store).Abandon` (Task 4).
- Produces: `func (win *window) stopWritingState()`: under `restoreCaptureMu`, detaches the state observer, `Abandon`s `win.restore` and sets it to nil; sets `win.cfgPath = ""` and `win.restorePath = ""`; replaces the branch store with `&branchsession.Store{Path: ""}` (`branch_launcher.go:48-57`). After it, `saveSetting`, workspace saves, worktree records and `shutdown`'s restore flush write nothing.

- [ ] **Step 1: Write the failing test** `TestNoWriteAfterUninstall`: a `testWindow` whose `cfgPath` is `<tmp>/kaze/config.toml` with `initRestore` run against `<tmp>/kaze/sessions.json` (the way `session_restore_test.go` sets it up); call `stopWritingState()`, then `os.RemoveAll(<tmp>/kaze)`; then `win.saveSetting("cursor_blink", true)`, a workspace save through the same path the Workspaces page uses, `win.branchStore()` record write (call whatever exported `Store` method writes, against a fake repo or by asserting `Path == ""`), and `win.shutdown()` → `<tmp>/kaze` does not exist. Also `TestStopWritingStateTwice` (no panic).

- [ ] **Step 2: Run to see it fail.** `go test ./internal/ui/ -run 'TestNoWriteAfterUninstall|TestStopWritingStateTwice' -v` → FAIL.

- [ ] **Step 3: Implement**, then grep for every other writer into the config folder (`config.Set`, `workspaces.Save`, `sessionstate`, `branchsession`, `os.WriteFile`, `os.Create`, `CreateTemp` under `internal/ui` and `internal/*` packages it calls) and every handle held open there; list them in the commit message and guard any that `cfgPath == ""` doesn't already stop.

- [ ] **Step 4: Run to see it pass.** `go test ./internal/ui/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(ui): Stop writing settings and state once Kaze is uninstalling`

---

### Task 6: Settings > Uninstall page

**Files:**
- Create: `internal/ui/settings_uninstall.go`
- Modify: `internal/ui/settings.go:115-129` (append `uninstallSection()`, icon `"uninstall"`, group `"Kaze"`), `internal/ui/settings_style.go` (an `"uninstall"` glyph near :202: a simple trash can in the same stroke style), `internal/ui/window.go` (window state field `uninst uninstallState`)
- Test: `internal/ui/settings_uninstall_test.go`

**Interfaces:**
- Consumes: `uninstall.Paths`, `uninstall.Inspect`, `uninstall.Run`, `sessionstate.Busy`, `win.stopWritingState()`, `win.closeWindow()` (`updates.go:160`), the existing `dialogBox` (`settings.go:103-112`).
- Produces: `func uninstallSection() settingsSection`; `func uninstallAct(win *window, id string, arg int)` with ids `"uni_remove"` (opens the dialog), `"uni_cancel"`, `"uni_confirm"`, `"uni_quit"`, `"uni_open_folder"` (Windows: open the exe's folder in Explorer after quit; may be omitted if awkward, then the path is shown instead); `func (win *window) uninstallPaths() uninstall.Paths` (Claude path `win.claudePath`, Codex `win.codexDir`, config `filepath.Dir(win.cfgPath)`, cache only on darwin, home `win.home`).

Behaviour (copy is a guide; plain, no em dashes):
- Opening the section runs `uninstall.Inspect` on a goroutine once and on each reopen; drawing uses the stored `Plan`.
- Page: "Remove Kaze from this computer. Do this before you delete the app." Then "What will be removed" (each `Step`: where, path with `~`, its lines; the `StatusBackup` line when set: "If you had your own status line before Kaze, it's saved in <file> in ~/.claude."; a relayout note when `Relayout`). Then "Will not be touched" when `Inline` or `Worktrees` is non-empty (texts from spec section 4.3). Unreadable files show with their manual step. One strong button "Remove Kaze data and hooks". When `Plan.Empty()`: "Nothing of Kaze's is left on this computer." plus the app-deletion text and no button.
- Dialog: title "Remove Kaze's data and hooks?", lines "Your preferences, workspace recipes and restored tabs will be lost." and, when any tab runs an agent, "<N> tabs are still running. They will close.", buttons Cancel / Remove.
- Confirm: on a goroutine: `win.stopWritingState()` is NOT called first; instead `Abandon` the restore store, then `sessionstate.Busy(<cfgdir>/sessions.json)`; if busy → result "Close your other Kaze windows, then try again. This window won't save its tabs until you open Kaze again." and nothing is removed (cfgPath stays, so settings still save). Otherwise `win.stopWritingState()`, then `uninstall.Run`, then store the `Result` and invalidate.
- Result: success "Kaze's data and hooks are removed." plus per-platform text: Windows "Kaze will close now. Then delete the folder it runs from: <dir of os.Executable()>."; macOS "Kaze will close now. Then drag Kaze to the Trash." with the `.app` path when it isn't under `/Applications`; and "Claude Code and Codex sessions that are already open keep the old hooks until you restart them." Failures list path, error and manual step. Button "Quit Kaze" → `win.closeWindow()`.

- [ ] **Step 1: Write the failing tests** (fake `uninstallPaths` via a window field the tests can set, temp dirs, no real home):
  - `TestUninstallIsLastSection`: `defaultSections()` last title is `"Uninstall"`, group `"Kaze"`, icon `"uninstall"`.
  - `TestUninstallCancelChangesNothing`: set up the Task 3 fixture; open section, `uni_remove`, `uni_cancel` → every fixture file byte-identical and `win.restore` still non-nil.
  - `TestUninstallConfirmRuns`: `uni_remove`, `uni_confirm`, wait (bounded, 1s) for the result → Kaze content gone, config dir gone, `win.cfgPath == ""`, result shows success.
  - `TestRemoveRefusedWhenBusy`: hold `sessionstate.Open` on the fixture's `sessions.json` from a second store before confirming → nothing removed, result says to close other Kaze windows, `win.cfgPath` unchanged.
  - `TestUninstallEmptyPlan`: nothing on disk → no `uni_remove` button in the drawn controls.
  - `TestUninstallDialogCountsRunningAgents`: two tabs with a running agent → the dialog lines include "2 tabs are still running. They will close."
  - `TestUninstallPageDrawNoAlloc` (or the package's equivalent frame check, if other sections have one): drawing the page twice reads no files (assert `Inspect` called once via a counter on the injectable function).

- [ ] **Step 2: Run to see them fail.** `go test ./internal/ui/ -run 'TestUninstall|TestRemoveRefusedWhenBusy' -v` → FAIL.

- [ ] **Step 3: Implement** following `settings_updates.go` (section, act, draw, `dialogBox` for the confirm).

- [ ] **Step 4: Run to see them pass**, then a headless capture of the page in dark and light themes (the capture route the `docs/audits/` reports use) and check text contrast like earlier audits (lowest at least 4.5:1). `go test ./internal/ui/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(ui): Add Settings > Uninstall to remove Kaze's data and hooks`

---

### Task 7: Docs

**Files:**
- Modify: `README.md` (a short "Uninstalling" section: open Settings > Uninstall, press the button, then delete the app; what's removed and what's left), `docs/HANDOFF.md` (where things stand: the feature, owner checks on Windows and macOS: run it on a throwaway user profile or with copies of real settings, never on the owner's live setup without asking), `docs/index.html` FAQ ("How do I uninstall Kaze?") only if the FAQ list exists and has room.

- [ ] **Step 1:** Write the docs. No version bump, changelog or "What's new" (the release PR does those).
- [ ] **Step 2:** `go test ./... && go vet ./...` → PASS.
- [ ] **Step 3: Commit** `docs: Explain how to uninstall Kaze`

## After the tasks

Whole-branch review, then push `feat/uninstall` and open a PR that closes #112. The owner merges. Then the release (next minor, per `CLAUDE.md` "Shipping a feature"), bundled with #143's fix if that hasn't shipped yet.
