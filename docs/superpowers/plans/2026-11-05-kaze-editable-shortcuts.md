# Editable Shortcuts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In Settings > Shortcuts, click a shortcut and press the new keys: it is saved to `config.toml` under `[keys]` and acts at once. A clash asks (Replace moves the keys and leaves the other action with `none`). Keys with no modifier are refused, and plain Ctrl+letter gets a warning. Each row can be reset, and "Reset all to defaults" asks first.

**Architecture:** Bottom-up in one worktree. `internal/config` gains `SetKeys`, a line-preserving writer for the `[keys]` table that sets or removes several lines in one atomic write and refuses keys written another way. `internal/ui/logic.go` learns `none`, resolves clashes after applying every override (so a swap or a Replace resolves cleanly), and gains `shortcutText`, the inverse of `parseShortcut`. Every reader of `win.binds` copes with an unbound action. `win.applyKeys` builds the keymap, binds and reserved keys the way start-up does, and runs again after each save (`win.reloadKeys`). The Shortcuts page gets a recording state in `settingsState`. It catches the next key press before the keymap sees it, decides the outcome (refused, same, clash, warning, saved), and draws the rows, strips and reset controls in Settings' own tokens.

**Tech Stack:** Go 1.27, Gio v0.10.3, BurntSushi/toml (validation only, never re-encoding), existing `internal/ui` settings page (`page`, `ctrl`, `dialogBox`, `keyCap`), `render.Readable`, `gpu/headless` for opt-in captures.

**Spec:** `docs/superpowers/specs/2026-11-05-kaze-editable-shortcuts-design.md` (approved 2026-11-05, binding, including section 8). **Mockup:** `docs/design/shortcuts-edit-mockup.html`. On behaviour the spec wins; on look the mockup wins. The mockup's refused text ("Ctrl, Alt or Shift") is wrong: the spec's wording applies (Ruling 4).

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: temp files under `t.TempDir()` of at most a few KB, at most 50 repetitions in loops, every wait at most 3 s (`emitTo`) or a single `time.Sleep(50 * time.Millisecond)` as `TestTerminalGetsNoKeysWhileSettingsShown` does, fake PTYs only (`recStarter` / `recPTY`).
- **No global keyboard or mouse input.** UI tests queue events on an `input.Router` (`r.Queue(key.Event{...})`, `clickAt`, `typeKeys` in `testkit_test.go`) and run frames with `sizedFrame` (`settings_frame_test.go`). Never SendInput, never another window.
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`). Every `go test` in this plan means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`
  or in PowerShell `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags ("-B 0x{0:x16}" -f (Get-Random -Maximum ([long]::MaxValue))) <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off.
- **No C compiler on this machine.** `GOOS=darwin go vet ./internal/config/` runs locally. `./internal/ui` needs cgo on darwin and is left to the macOS CI job (`.github/workflows/test.yml`). Tests that depend on the OS take `goos` as a parameter (as `resolveBindings` does) so both OSes run on Windows.
- **Lean:** nothing new runs while Settings is closed. Recording adds no timer, ticker, goroutine or animation (the dot is still). A save does one file read, one write and one `config.Load`.
- **Config edits** only through `config.SetKeys`, which re-encodes nothing. Comments, order, other tables and CRLF stay byte for byte, and only the named `[keys]` lines change. The file is replaced atomically (`replace`) and symlinks are followed, as `config.Set` does.
- **Editable rows** are exactly the action names in `defaultBindings(goos)`, in `kazeCommands` order inside `shortcutGroups` (spec 8.1). Mouse gestures stay read-only rows.
- **Copy fixed by the spec** (exact, `<x>` filled in):
  - hint: "Click a shortcut to change it. Changes save at once."
  - recording: "Press the new keys… Esc cancels"
  - refused: "<keys> needs Ctrl or Alt with it, or it would stop you typing it. <action> keeps <old keys>." (macOS: "Ctrl, Option or Cmd", Ruling 4)
  - clash: "<keys> is used by <other>. Use it here instead? <other> will have no shortcut." Buttons **Cancel**, **Replace** (strong).
  - warning: "Saved. Your shell and Claude Code use Ctrl+<L> (<what it does>). Kaze will take it from them." Button **OK**.
  - save error: "Couldn't save: <error>"
  - Settings: "Settings needs a shortcut. Pick other keys."
  - unbound row: "None" (muted italic). The tag reads "changed".
  - reset all: link "Reset all to defaults"; dialog title "Reset every shortcut to its default?", buttons **Cancel**, **Reset all** (strong).
  - `<keys>`, `<old keys>` are `formatShortcut(goos, sc)` ("Ctrl+Shift+F", "Cmd+K"). `<action>`, `<other>` are the `kazeCommands` labels.
- **Saved text form** is what `parseShortcut` reads back: lower case, `+`-joined, modifiers in the order `ctrl`, `alt`, `shift`, `cmd` (`ctrl+shift+f`, `cmd+k`, `f5`, `ctrl+,`).
- **Colours** only from the theme through `pageColors` / chrome tokens. Every text at least 4.5:1 on its surface and every mark at least 3:1, in all five `theme.All()` themes.
- Commits: `type(scope): Verb ...`, **no AI attribution of any kind** (CLAUDE.md overrides any default). Never push to `main`, and this plan pushes nothing (the coordinator opens the PR). The release (0.31: version, changelog, lander "What's new") is its own PR after the merge, not part of this plan.

## Review Focus

1. **A text field had the keyboard.** The person types in "Search shortcuts", then clicks a shortcut and presses keys. The press must be recorded and must not go into the field or run an action. (Task 5 `TestRecordingAfterTypingInSearch`.)
2. **Hand-written `[keys]` shapes:** a quoted name (`"find" = ...`), a header with a comment (`[keys] # mine`), a value with a trailing comment, a file with no final newline, a `[keys]`-looking line inside a multiline string. Each must be edited in place or left alone, never duplicated. (Task 1 `TestSetKeysHandWrittenShapes`, `TestSetKeysSkipsMultilineString`.)
3. **Map order:** `resolveBindings` must give the same binds and warnings whatever order Go iterates the overrides in. (Task 2 `TestResolveIsOrderIndependent`, 50 runs.)
4. **Stale keys elsewhere after a save:** the title bar, the palette's action list, the alert card caps and the branch launcher must show the new keys (or none) on the next frame, not the start-up ones. (Task 4 `TestReloadRefreshesShownKeys`.)
5. **The file changed by hand while the page is open:** a page save must keep the person's other edit, and the keymap must take it in as well. (Task 6 `TestPageSaveKeepsAHandEdit`.)

---

### Task 1: `config.SetKeys`, the `[keys]` writer

**Files:**
- Modify: `internal/config/write.go` (new `SetKeys`, `KeysEditable`, `ErrKeysForm`; factor the symlink + `MkdirAll` + `replace` tail of `Set` into `writeFile(path, out string) error` and reuse it in both)
- Test: `internal/config/write_test.go`

**Interfaces:**
- Produces:
  ```go
  var ErrKeysForm = errors.New("shortcuts are not written in a [keys] table")
  // SetKeys sets (value != "") or removes (value == "") each named line inside
  // [keys], in one write. Names are applied in sorted order.
  func SetKeys(path string, edits map[string]string) error
  // KeysEditable is nil when SetKeys can edit path (a missing file counts),
  // a parse error for a broken file, or an error wrapping ErrKeysForm.
  func KeysEditable(path string) error
  ```
- Rules:
  - The probe decode runs before and after the edit, as in `Set`. A file that does not parse is never written.
  - The table is the line matching `^\s*\[\s*keys\s*\]\s*(#.*)?\r?$` while `tomlScan` is at top level, up to the next table header. If the decoded file has a `keys` value but no such header (`keys = {...}`, `keys.find = ...`, `["keys"]`), the error wraps `ErrKeysForm` and nothing is written (spec 8.6, Ruling 8).
  - **Set:** an existing line (found with `assignsKey`) becomes `name = "<value>"` plus `commentOf(old)`. A new line goes right after the table's last assignment, or right after the header when the table is empty. If there is no table, the file gets one at the end: one blank line (when the file is not empty and does not end in a blank line), then `[keys]`, then the lines in sorted order.
  - **Remove:** the line is deleted. A missing line is a no-op. The header always stays.
  - `eol` is detected as in `Set`. If the edits change nothing, nothing is written. If removing from a missing file, no file is created.
  - Names must match `^[a-z_]+$`, otherwise there is an error. Values go through `tomlLiteral`.

- [ ] **Step 1: Write the failing tests** in `write_test.go` (each writes a temp file, calls `SetKeys`, compares the whole file text):
  - `TestSetKeysReplacesKeepingComment`: `"[keys]\nfind = \"f5\" # mine\n"` + `{"find": "ctrl+alt+f"}` → `"[keys]\nfind = \"ctrl+alt+f\" # mine\n"`.
  - `TestSetKeysAddsAfterLastLineOfTable`: `"theme = \"washi\"\n\n[keys]\npalette = \"ctrl+shift+p\"\n\n# colours\n[colors]\nbg = \"#000000\"\n"` + `{"find": "f5"}` → `find = "f5"` lands on the line after `palette`, and every other line is unchanged.
  - `TestSetKeysRemovesLine`: removing `palette` leaves `"[keys]\n"` plus everything else. Removing a name that is not there writes nothing (the file's mod time is unchanged).
  - `TestSetKeysCreatesTable`: `"theme = \"tide\"\n[colors]\nbg = \"#000000\"\n"` + `{"find": "f5"}` → the same text + `"\n[keys]\nfind = \"f5\"\n"`.
  - `TestSetKeysKeepsCRLF`: a CRLF file with a comment and `[keys]` → the result has no lone `\n`, and the new and changed lines end in `\r\n`.
  - `TestSetKeysTwoLinesOneWrite`: `{"find": "ctrl+shift+b", "toggle_sidebar": "none"}` → both lines. `Load(path)` gives `Keys["find"] == "ctrl+shift+b"`, `Keys["toggle_sidebar"] == "none"` and no warnings.
  - `TestSetKeysRefusesOtherForms`: `"keys = { find = \"f5\" }\n"`, `"keys.find = \"f5\"\n"` and `"[\"keys\"]\nfind = \"f5\"\n"` each give `errors.Is(err, ErrKeysForm)` and leave the file unchanged.
  - `TestSetKeysRefusesBrokenFile`: `"[keys\n"` → error, file unchanged.
  - `TestSetKeysMissingFile`: remove → no file. Set → the file is exactly `"[keys]\nfind = \"f5\"\n"`.
  - `TestSetKeysHandWrittenShapes` (Review Focus 2): `"[keys] # mine\n\"find\" = \"f5\"\npalette = \"ctrl+k\"   # k"` (no final newline) + `{"find": "f6", "palette": ""}` → `"[keys] # mine\nfind = \"f6\"\n"`. The quoted name is replaced, not duplicated, and the last line plus its comment is removed.
  - `TestSetKeysSkipsMultilineString`: a top-level `script = '''\n[keys]\nfind = "x"\n'''` and no real table → a real `[keys]` table is appended and the string is unchanged.
  - `TestKeysEditable`: nil for a good file and a missing one, a non-nil error that is not `ErrKeysForm` for `"[keys\n"`, and `ErrKeysForm` for `"keys.find = \"f5\"\n"`.
  - `TestSetKeysFollowsASymlink` (skip if `os.Symlink` fails, as `TestSetFollowsASymlink` does).
- [ ] **Step 2: Run** `go test ./internal/config/ -run 'SetKeys|KeysEditable'` → FAIL (undefined: SetKeys).
- [ ] **Step 3: Implement** `SetKeys`, `KeysEditable`, `ErrKeysForm` and `writeFile` in `write.go`, by the rules above.
- [ ] **Step 4: Run** `go test ./internal/config/` and `GOOS=darwin go vet ./internal/config/` → PASS (the old `Set` tests still pass after the `writeFile` refactor).
- [ ] **Step 5: Commit** `feat(config): Set and remove shortcut lines in the [keys] table`

### Task 2: `none`, the order-independent clash rule and `shortcutText`

**Files:**
- Modify: `internal/ui/logic.go` (`resolveBindings`, `buildKeymap`, new `shortcutText`, `isNone`)
- Test: `internal/ui/logic_test.go`

**Interfaces:**
- Produces:
  ```go
  func isNone(text string) bool                          // strings.EqualFold(strings.TrimSpace(text), "none")
  func shortcutText(sc input.Shortcut) (string, bool)    // the saved form; false when parseShortcut could not read it back
  // resolveBindings keeps its signature; an unbound action is absent from the map.
  ```
- `resolveBindings` algorithm (spec 3.3, 8.5):
  1. Start from `defaultBindings(goos)`. Go through the override names in sorted order and skip unknown ones. `none` deletes the action's binding, except `open_settings`, which keeps its default with the warning `keys.open_settings: Settings needs a shortcut; using the default` (Ruling 2). A text that does not parse keeps the default with today's warning.
  2. Once every override is applied, group the actions by final shortcut. In each group of two or more, each *overridden* member goes back to its default, with `keys.<name>: "<text>" is already <other>; using the default`, where `<other>` is the first other name in the group in sorted order. That is today's text for today's case. One pass only: a default that still clashes is left as is.
- `buildKeymap` adds names in sorted order, so a clash left over from a hand edit always gives the same keymap (Ruling 13).
- `shortcutText`: modifiers `ctrl`, `alt`, `shift`, `cmd` in that order (`ModSuper` → false). The key names are `tab space enter esc backspace delete pageup pagedown home end up down left right f1`..`f12`, or a single-rune name lower-cased. These give false: `+`, `key.NameEnter` (numpad), the modifier names (`key.NameCtrl`, `NameShift`, `NameAlt`, `NameSuper`, `NameCommand`), and any other multi-rune name.

- [ ] **Step 1: Write the failing tests** (`logic_test.go`):
  - `TestNoneUnbinds`: `{"find": " None "}` on `"windows"` → no `"find"` in binds, no warnings, and no `actFind` value in `buildKeymap`'s map.
  - `TestSettingsCannotBeNone`: `{"open_settings": "none"}` → `binds["open_settings"] == {Name: ",", Mods: key.ModCtrl}`, and the one warning is exactly the text above.
  - `TestSwappedOverridesResolveCleanly`: `{"palette": "ctrl+shift+f", "find": "ctrl+shift+k"}` → palette Ctrl+Shift+F, find Ctrl+Shift+K, no warnings.
  - `TestReplaceResolvesCleanly`: `{"find": "ctrl+shift+b", "toggle_sidebar": "none"}` → find Ctrl+Shift+B, no toggle_sidebar, no warnings.
  - `TestOverrideOntoADefaultStillWarns`: `{"find": "ctrl+shift+k"}` → find keeps Ctrl+Shift+F, palette keeps Ctrl+Shift+K, warning `keys.find: "ctrl+shift+k" is already palette; using the default`.
  - `TestRealClashKeepsBothDefaults`: `{"find": "ctrl+shift+q", "palette": "ctrl+shift+q"}` → both defaults, two warnings, one starting `keys.find:` and one `keys.palette:`.
  - `TestResolveIsOrderIndependent` (Review Focus 3): 50 runs of each case above give equal binds (`maps.Equal`) and equal warnings.
  - `TestShortcutTextRoundTrips`: for every default shortcut on `"windows"` and `"darwin"`, plus F5, Ctrl+Alt+K, Cmd+Shift+Space, Ctrl+`,` and Ctrl+Tab, `parseShortcut(shortcutText(sc))` gives `sc` back. Exact: Ctrl+Shift+F → `ctrl+shift+f`, Cmd+K → `cmd+k`, F5 → `f5`, Ctrl+`,` → `ctrl+,`.
  - `TestShortcutTextRefusesUnnameable`: `+` with Ctrl, `key.NameEnter`, `key.NameCtrl`, `key.NameCommand`, `"Insert"`, and Ctrl+Super+K → false.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'None|Swapped|Replace|Override|Clash|OrderIndependent|ShortcutText'` → FAIL.
- [ ] **Step 3: Implement** by the rules above. Keep the existing `TestCommandsShowsOverrides` and the other keymap tests green.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Keymap|Bind|Shortcut|None|Clash|Override|Swapped|Replace|OrderIndependent'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Let a shortcut be none and resolve clashes after every override`

### Task 3: Every reader of `win.binds` handles a missing binding

**Files:**
- Modify: `internal/ui/commands.go` (new `win.keyText`), `internal/ui/empty_window.go:127`, `internal/ui/titlebar_draw.go:76`, `internal/ui/settings_commands.go` (`keysOf`), `internal/ui/palette_window.go:166`, `internal/ui/branch_launcher.go:495` (these two use it, so behaviour stays the same), `internal/ui/alerts_card.go:244` (already handles a missing binding; check only)
- Test: `internal/ui/commands_test.go`

**Interfaces:**
- Produces: `func (win *window) keyText(name string) string`: `formatShortcut(runtime.GOOS, sc)` for a bound action, "" for an unbound one. The empty window draws no shortcut line when it is "" (and the block's height leaves it out). The title bar's palette pill draws no key hint when it is "". On the Shortcuts page, `keysOf` returns "" and the row shows "None" (Task 8 styles it).

- [ ] **Step 1: Write the failing tests:**
  - `TestKeyTextUnbound`: `win.binds = map[string]input.Shortcut{}` → `keyText("palette") == ""`, and with defaults it is `formatShortcut(runtime.GOOS, ...)`.
  - `TestEveryReaderWithNoBindings`: a window with `win.binds` emptied (and `win.km` emptied) draws one frame each of the empty window (no sessions), the title bar, the palette open, Settings > Shortcuts, an alert card and the branch launcher, without panicking. The empty window's `win.empty.shortcut == ""`, every palette action's `Key == ""`, the Shortcuts page texts hold "None" once per editable action, and `nextWaitingCaps()` is nil.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'KeyTextUnbound|NoBindings'` → FAIL (the empty window shows a stray label, and the title bar formats a zero shortcut).
- [ ] **Step 3: Implement** `keyText` and use it in the readers listed.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'KeyText|NoBindings|EmptyWindow|TitleBar|Palette|Alert|Launcher|Commands'` → PASS.
- [ ] **Step 5: Commit** `fix(ui): Show no keys for an action without a shortcut`

### Task 4: Rebuild the keymap live

**Files:**
- Modify: `internal/ui/window.go` (`Run` ~:223-241 calls `applyKeys`), new `internal/ui/settings_keys.go`
- Test: new `internal/ui/settings_keys_test.go`

**Interfaces:**
- Consumes: `buildKeymap`, `resolveBindings` (Task 2), `config.Load`.
- Produces:
  ```go
  // applyKeys sets win.km, win.binds, win.opt.Config.Keys and input.SetReserved
  // together from [keys] text, and drops cached key labels. Returns the warnings.
  func (win *window) applyKeys(keys map[string]string) []string
  // reloadKeys re-Loads win.cfgPath and applies only its Keys (spec 8.2).
  // On a load error it returns the error and changes nothing.
  func (win *window) reloadKeys() error
  ```
- `Run` builds the window, then calls `applyKeys(opt.Config.Keys)` and appends the warnings to `opt.Warnings`, so start-up and a save take the same path. The cached labels dropped are `win.side.launch.work.shortcutReady = false`. The alert caps cache is keyed by the shortcut and refreshes by itself, and the title bar and palette read `keyText` every time. Then `win.w.Invalidate()`.

- [ ] **Step 1: Write the failing tests** (`t.Cleanup` puts back `input.SetReserved(runtime.GOOS, ...)` with the defaults):
  - `TestReloadKeysReplacesKeymapAndBinds`: a temp `config.toml` with `[keys]\nfind = "ctrl+alt+f"\n`, `win.cfgPath` set, `reloadKeys()` → nil. Then `win.km[{F, Ctrl|Alt}] == actFind`, no `win.km[{F, Ctrl|Shift}]`, `win.binds["find"]` is Ctrl+Alt+F, `input.IsReserved("F", Ctrl|Alt, runtime.GOOS)` is true and `IsReserved("F", Ctrl|Shift, ...)` is false.
  - `TestReloadKeysLeavesKeymapOnError`: a file of `"[keys\n"` → error, and `win.km` / `win.binds` are the same maps as before (`maps.Equal`).
  - `TestReloadKeysTouchesNothingElse`: a file with `theme = "washi"`, `font_size = 20` and a `[keys]` line → after reload, the window's theme key and `opt.Config.FontSize` are unchanged and `opt.Config.Keys` is the file's.
  - `TestReloadRefreshesShownKeys` (Review Focus 4): bind `new_branch_session` and `palette` to new keys in the file and reload → the launcher's `shortcut` after its next draw, `keyText("palette")` and the palette's action list (opened after the reload) all show the new keys. With `next_waiting = "none"`, `nextWaitingCaps()` is nil.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'ReloadKeys|ReloadRefreshes'` → FAIL.
- [ ] **Step 3: Implement** `applyKeys` and `reloadKeys`, and switch `Run` over to them.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Reload|Keymap|Settings'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Rebuild the keymap from config.toml without a restart`

### Task 5: Recording on the Shortcuts page

**Files:**
- Modify: `internal/ui/settings.go` (`settingsState` fields, the recording check at the top of `settingsKey` before the `win.km` lookup, the cancel in `settingsPointer`, stopping recording in `selectSection` / `hideSettings` / `closeSettings`), `internal/ui/settings_commands.go` (`commandsSection` gets `show`, a dynamic hint, the `key_rec` control and its `act`), `internal/ui/window.go:437` (losing window focus stops recording), `internal/ui/settings_keys.go`
- Test: `internal/ui/settings_keys_test.go`

**Interfaces:**
- Produces:
  ```go
  // in settingsState:
  keyRec  string     // the action being recorded; "" none
  keyMsg  keyMessage // the strip under one row; zero value none
  keysErr string     // why editing is off; "" when it is on

  type msgKind int // msgNone, msgRefused, msgClash, msgWarn
  type keyMessage struct {
      action string         // the row it sits under
      kind   msgKind
      text   string
      keys   input.Shortcut // clash: the keys asked about
      other  string         // clash: the action that has them
      reset  bool           // clash raised by a reset (Replace removes the line instead of setting it)
  }
  func editableActions(goos string) []string        // kazeCommands order, filtered by defaultBindings(goos)
  func (win *window) checkKeys()                    // sets keysErr; the section's show
  func (win *window) startRecording(action string)  // no-op when keysErr != ""; clears keyMsg
  func (win *window) stopRecording()
  func (win *window) recordKey(e key.Event)         // only called while keyRec != ""
  func (win *window) keyPressed(action string, sc input.Shortcut) // outcomes; this task saves plainly, Task 6 completes it
  func (win *window) saveKeys(edits map[string]string) error      // config.SetKeys then reloadKeys
  ```
- Control ids: `key_rec`, with `arg` an index into `editableActions(runtime.GOOS)`. Enter and Space on a focused `key_rec` go through the existing `activate`, and the Enter press itself is not recorded (only the *next* press is).
- `recordKey` order. These are ignored, and recording goes on: releases, modifier-only names, any press with `ModSuper` (spec 3.6, on both OSes, Ruling 3), and names `shortcutText` refuses (spec 8.4). Esc with any modifiers → `stopRecording`. Anything else → `stopRecording`, then `keyPressed`. Edit events are already dropped by `settingsEvents`, so AltGr text never arrives as a press. Ctrl+Alt+K is taken only when the key event carries both modifiers.
- `checkKeys` hints (Ruling 14):
  - `cfgPath == ""` → "Shortcuts can't be saved: Kaze has no config folder on this computer."
  - a parse error → "config.toml has an error, so shortcuts can't be changed here: <err>"
  - `ErrKeysForm` → "Your shortcuts are written outside a [keys] table in config.toml. Move them into a [keys] table to change them here."
  - otherwise the spec hint. `checkKeys` runs on `show` and after each save.
- Test helpers (`settings_keys_test.go`):
  - `shortcutsWindow(t, cfg string) (*window, *input.Router, *recStarter, string)`: `testWindow` with one session, `cfgPath` = a temp file holding `cfg`, Settings open on Shortcuts, one `sizedFrame(t, win, r, 1000, 900)`.
  - `clickKeys(t, win, r, action)`: `clickAt` the centre of the `key_rec` control for that action.
  - `pressIn(t, win, r, name, mods)`: queue a press and a release, then two frames.

- [ ] **Step 1: Write the failing tests:**
  - `TestClickStartsRecording`: click `find` → `keyRec == "find"`, and the page texts hold "Press the new keys… Esc cancels".
  - `TestEnterStartsRecording`: Tab to the `find` control, then Enter → recording. The file is unchanged and the row is still recording (the Enter was not taken as the new keys).
  - `TestEscCancels`: record, then Esc (and Shift+Esc) → `keyRec == ""`, file bytes and `win.km` unchanged.
  - `TestClickElsewhereCancels`: record `find`, then click the search field → not recording. Record `find`, then click `palette`'s keys → recording `palette`.
  - `TestLeavingTheSectionCancels`: record, then `selectSection(0)` → not recording. The same after `hideSettings` and after the window loses focus.
  - `TestIgnoredPressesKeepRecording`: `key.NameCtrl` alone, `key.NameShift` alone, Ctrl+`+`, `key.NameEnter`, Ctrl+Super+K, and an `EditEvent{Text: "€"}` each leave `keyRec == "find"` and the file unchanged.
  - `TestPlainPressSaves`: record `find`, then Ctrl+Alt+F → the file has `find = "ctrl+alt+f"`, `win.km[{F, Ctrl|Alt}] == actFind`, no `win.km[{F, Ctrl|Shift}]`, and recording has ended.
  - `TestSavedKeysActInTheSameBatch`: one router batch with the `key_rec` click for `toggle_sidebar` and then Ctrl+Alt+B → after that single frame, the file holds `toggle_sidebar = "ctrl+alt+b"`, `win.km[{B, Ctrl|Alt}] == actToggleSidebar`, there is no `win.km[{B, Ctrl|Shift}]` and `keyRec == ""`. This proves the rebuild is synchronous.
  - `TestRecordingReachesNoActionOrShell`: record `find`, then Ctrl+Shift+T (new session's keys) → the session count is unchanged, and after `time.Sleep(50 * time.Millisecond)` `st.ptys[0].sent() == ""`.
  - `TestRecordingAfterTypingInSearch` (Review Focus 1): click the search field and `typeKeys` "fi", then click `find`'s keys and press Ctrl+Alt+F → saved, and the search text is still "fi".
  - `TestEditingOff`: with `cfgPath = ""`, with `"[keys\n"` and with `"keys.find = \"f5\"\n"`, the hint is the matching text above, and clicking `find`'s keys does not record. The rows still show the active keys.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Recording|Cancels|IgnoredPresses|PlainPress|SameBatch|EditingOff|StartsRecording'` → FAIL.
- [ ] **Step 3: Implement** by the interfaces above. `keyPressed` for now: if `sc` equals the default, `saveKeys({action: ""})`, else `saveKeys({action: text})` (Ruling 1). On an error, `keyMsg` is refused-kind with "Couldn't save: <err>".
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Settings|Shortcut|Commands|Recording|Cancels|Ignored|PlainPress|SameBatch|EditingOff'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Record a new shortcut on the Shortcuts page`

### Task 6: What happens to the keys pressed

**Files:**
- Modify: `internal/ui/settings_keys.go` (`keyPressed`, text builders, `keysAct` for the strip buttons), `internal/ui/settings_commands.go` (`commandsAct` sends the `key_*` ids to `keysAct`)
- Test: `internal/ui/settings_keys_test.go`

**Interfaces:**
- Produces:
  ```go
  func needsModifier(sc input.Shortcut) bool // true unless F1-F12 or Mods has Ctrl, Alt or Command
  func refusedText(goos, label string, sc input.Shortcut, old input.Shortcut, hasOld bool) string
  func clashText(goos string, sc input.Shortcut, otherLabel string) string
  func warnText(sc input.Shortcut) (string, bool) // only Mods == ModCtrl and Name a single letter A-Z
  var shellKeyUse = map[key.Name]string{"R": "reverse search", "C": "stop a command", "D": "end input",
      "L": "clear the screen", "A": "go to line start", "E": "go to line end",
      "U": "delete the line", "W": "delete a word", "Z": "suspend"}
  ```
- `keyPressed(action, sc)` order (spec 3.2):
  1. `needsModifier` → msgRefused, `refusedText`. With no old binding, the end reads "<action> stays without a shortcut." (Ruling 4.)
  2. `sc == win.binds[action]` → nothing.
  3. Another action `o` has `win.binds[o] == sc`: if `o == "open_settings"`, msgRefused "Settings needs a shortcut. Pick other keys." at once, with no question (Ruling 2). Otherwise msgClash with `clashText` (keys, other).
  4. Save. If `sc` is the action's default the line is removed, otherwise it is set to `shortcutText(sc)`. Then, if `warnText` holds, msgWarn.
- Strip buttons: `key_ok` clears `keyMsg`. `key_cancel` clears it and saves nothing. `key_replace` runs `saveKeys({action: <text, or "" if default>, other: "none"})` (for a `reset` clash the action's value is "") and then the step 4 warning check. Any save error replaces the strip with "Couldn't save: <err>". Recording another row clears the strip.

- [ ] **Step 1: Write the failing tests:**
  - `TestRefusedTexts` (pure, both OSes): K on `"windows"`, label "Copy", old Ctrl+Shift+C → "K needs Ctrl or Alt with it, or it would stop you typing it. Copy keeps Ctrl+Shift+C." The same on `"darwin"` with old Cmd+C → "K needs Ctrl, Option or Cmd with it, or it would stop you typing it. Copy keeps Cmd+C." With no old binding → "… Copy stays without a shortcut."
  - `TestNeedsModifier`: K, Shift+K, Enter, Esc, Tab, Space, Up, Shift+Up, `1`, `,` → true. F5, Shift+F5, Ctrl+K, Alt+K, Cmd+K → false.
  - `TestWarnTexts`: Ctrl+R → "Saved. Your shell and Claude Code use Ctrl+R (reverse search). Kaze will take it from them." Ctrl+Q → "Saved. Your shell and Claude Code use Ctrl+Q. Kaze will take it from them." Ctrl+Shift+R, Alt+R, Ctrl+1 and F5 → false.
  - `TestRefusedOnThePage`: record `copy`, press K → the strip text is as above, the file is unchanged and an OK control exists. OK clears it.
  - `TestFKeyAllowed`: record `find`, press F5 → `find = "f5"`.
  - `TestSamePressChangesNothing`: record `find`, press Ctrl+Shift+F → file bytes unchanged, no strip.
  - `TestClashCancel`: record `find`, press Ctrl+Shift+B → texts hold "Ctrl+Shift+B is used by Toggle sidebar. Use it here instead? Toggle sidebar will have no shortcut." and Cancel / Replace controls. Cancel → file and `win.km` unchanged, no strip.
  - `TestClashReplace`: the same, then Replace → the file has `find = "ctrl+shift+b"` and `toggle_sidebar = "none"`, `win.km[{B, Ctrl|Shift}] == actFind`, no `binds["toggle_sidebar"]`, and the Toggle sidebar row reads "None".
  - `TestClashWithSettingsRefused`: record `palette`, press Ctrl+`,` → "Settings needs a shortcut. Pick other keys.", no Replace control, file unchanged.
  - `TestWarningOnThePage`: record `palette`, press Ctrl+R → saved `palette = "ctrl+r"`, and the strip holds the Ctrl+R text and OK. Recording another row clears it.
  - `TestDefaultKeysRemoveTheLine`: with a file of `find = "ctrl+alt+f"`, record `find` and press Ctrl+Shift+F → the `find` line is gone.
  - `TestSaveFailureKeepsKeymap`: open the page, then overwrite the file with `"[keys\n"`, record `find`, press Ctrl+Alt+F → the strip starts with "Couldn't save: " and `win.km` is unchanged.
  - `TestPageSaveKeepsAHandEdit` (Review Focus 5): open the page, append `palette = "ctrl+alt+p"` under `[keys]` by hand, record `find` and press Ctrl+Alt+F → the file has both lines, and `binds["palette"]` is Ctrl+Alt+P.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Refused|NeedsModifier|WarnTexts|FKey|SamePress|Clash|Warning|DefaultKeys|SaveFailure|HandEdit'` → FAIL.
- [ ] **Step 3: Implement** `keyPressed`, the text builders and `keysAct`.
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Settings|Shortcut|Commands|Refused|Clash|Warn|Keys'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Refuse, ask or warn about the keys pressed`

### Task 7: Reset a row and reset all

**Files:**
- Modify: `internal/ui/settings_keys.go` (`resetKey`, `resetAllKeys`, `keysDialogBox`), `internal/ui/settings_commands.go` (`key_reset` and `key_reset_all` controls, `dialog: keysDialogBox`)
- Test: `internal/ui/settings_keys_test.go`

**Interfaces:**
- Produces:
  ```go
  func (win *window) keyChanged(action string) bool // binds differ from the default, or unbound (spec 8.7)
  func (win *window) resetKey(action string)        // the clash question when the default is now another action's
  func (win *window) resetAllKeys()                 // SetKeys with every editableActions name -> ""
  func keysDialogBox(win *window) *dialogBox        // non-nil while settingsState.keysConfirm is true
  ```
- `settingsState` gains `keysConfirm bool`.
- `key_reset` (with `arg` the action index) exists only on changed rows, and only while editing is on.
- `resetKey`: if another action `o` has the default → `o == "open_settings"` gives "Settings needs a shortcut. Pick other keys.", otherwise a msgClash with `reset: true`. If not, `saveKeys({action: ""})`.
- The `key_reset_all` link is drawn whenever editing is on (Ruling 10). It sets `keysConfirm`. The dialog buttons are `keys_cancel` ("Cancel") and `keys_reset_all` ("Reset all", strong). `resetAllKeys` leaves the names of unknown actions alone.

- [ ] **Step 1: Write the failing tests:**
  - `TestChangedRowsHaveTagAndReset`: a file with `find = "ctrl+alt+f"` and `copy = "none"` → exactly two `key_reset` controls, and "changed" appears twice in the texts. With no file, there are none.
  - `TestResetRemovesTheLine`: reset `find` → the line is gone and `binds["find"]` is the default.
  - `TestResetAsksOnClash`: `find = "ctrl+alt+f"`, `palette = "ctrl+shift+f"` → reset `find` → "Ctrl+Shift+F is used by Open the palette. Use it here instead? Open the palette will have no shortcut." Replace → the `find` line is gone, `palette = "none"`, and `win.km[{F, Ctrl|Shift}] == actFind`.
  - `TestResetCannotUnbindSettings`: `find = "ctrl+alt+f"`, `open_settings = "ctrl+shift+f"` → reset `find` → the Settings text, file unchanged.
  - `TestResetAllAsks`: click `key_reset_all` → the dialog title is "Reset every shortcut to its default?", and Cancel → file unchanged.
  - `TestResetAllRemovesOnlyKnownLines`: a file with a comment, `[keys]` holding `find`, `palette`, `copy = "none"` and an unknown `zoom = "ctrl+="`, then `[colors]` → after Reset all, the text is the same with only those three lines gone (the comment, header, `zoom` and `[colors]` stay byte for byte), and `win.binds` equals `defaultBindings(runtime.GOOS)`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'ChangedRows|Reset'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/ -run 'Settings|Shortcut|Commands|Reset|Keys'` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Reset one shortcut or all of them to the defaults`

### Task 8: Draw the page as the mockup, with contrast and captures

**Files:**
- Modify: `internal/ui/settings_commands.go` (`shortcutCard` row layout, the strip, the reset-all link), `internal/ui/settings_style.go` (`iconNames` gains `"reset"`, drawn by `chrome.icon` as an open circle arc with an arrowhead, ↺)
- Create: `internal/ui/shortcuts_render_test.go`
- Test: `internal/ui/settings_keys_test.go`

**Interfaces:**
- Produces:
  ```go
  type shortcutInk struct {
      label, action, none, capText, capFace, tagText, tagBG,
      btnFocusLine, btnFocusBG, recLine, recBG, recText, recDot,
      resetIcon, card color.NRGBA
      strip [4]struct{ bg, line, text color.NRGBA } // indexed by msgKind; [msgNone] unused
      btnText, btnBG, strongText, strongBG color.NRGBA
  }
  func (c *chrome) shortcutInk() shortcutInk
  ```
- Mockup values, in dp/sp:
  - row min height 44 and `padding 0 10 0 16`. The label is Manrope 13 in `title`, with the action name under it in JetBrains Mono 11.5 in `dim` (the 130 dp right-hand config column goes).
  - the "changed" tag is Manrope SemiBold 10.5, 8 dp after the label, radius 999, padding 1×7, with `accent` text (through `Readable`) on `tint(accent, 0x24)`.
  - the keys button has min width 150, padding 5×8 and radius 8, with caps right-aligned 4 apart (today's `keyCap`). It is borderless at rest. With keyboard focus it gets a 1 dp `line` hairline and a `CapHover` fill (Ruling 5).
  - recording: a 1 dp `accent` border on `tint(accent, 0x1f)`, a 7 dp still dot, then the recording text in Manrope 13.
  - "None" is Manrope 13 italic in `muted`.
  - the reset button is 28×28, radius 7, with the `reset` icon in `muted`. Its slot is kept on every row so the caps line up.
  - strip: margin `0 10 10 16`, radius 9, padding 10×12, gap 10, text Manrope 13. Fill is the kind's token at alpha 0x1a, border at 0x59. Refused uses `DiffDelFG`, clash `accent`, warning `needs` (Ruling 7). The text is `Readable(title, fill over card)`, and the keys in it are SemiBold. Buttons are Manrope SemiBold 12.5, radius 7, padding 5×11: Cancel and OK on `raised` with a `line` border, Replace with `accent` fill and `Readable(chrome bg, accent)` text. Below 200 dp of text width, the buttons drop under the text (spec section 9).
  - "Reset all to defaults" is a Manrope 13 `muted` underlined link, right-aligned 6 dp under the last shortcut card, above "Command line".
- [ ] **Step 1: Write the failing tests:**
  - `TestShortcutsContrast`: for each of `theme.All()`, `newChrome()` and `setTheme(th)`, then `shortcutInk()`. Each text pair is at least 4.5:1 (`contrast` from `statusbar_test.go`): label, action, none and tagText on its composite tagBG; capText on capFace; recText on recBG over card; each strip's text on its bg over card; btnText on btnBG; strongText on strongBG. Each mark is at least 3:1 against card: recLine, recDot, resetIcon and each strip's line over card.
  - `TestRowLayout`: at 1000 dp the action name sits under its label (a larger y, the same x), and every row's reset slot has the same right edge.
  - `TestNoneDrawn`: with `copy = "none"`, `set.texts` holds "None" right after "Copy" / "copy", and there are no caps for that row.
  - `TestNarrowStripWraps`: a clash strip at a 520 dp-wide page stays inside the card, and its buttons sit below the text.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'ShortcutsContrast|RowLayout|NoneDrawn|NarrowStrip'` → FAIL.
- [ ] **Step 3: Implement** `shortcutInk`, the row, strip and link drawing, and the `reset` icon. Nudge only alpha values or the `Readable` input if a theme fails a contrast check. Never hard-code a hex.
- [ ] **Step 4: Captures:** `TestShortcutsRenders`, skipped unless `KAZE_SHORTCUTS_RENDER_DIR` is set. In Midnight and Washi it writes 10 PNGs: `default`, `recording`, `clash`, `warning`, `refused`. Model it on `TestFindRenders`, at 1000×900. Run it with the env var set to `docs/audits/shortcuts`, look at all 10 against the mockup, and fix what differs.
- [ ] **Step 5: Run** `go test ./internal/ui/` and `go vet ./...` → PASS.
- [ ] **Step 6: Commit** `feat(ui): Draw editable shortcut rows, messages and reset` (the 10 captures go in the same commit, under `docs/audits/shortcuts/`)

### Task 9: Docs

**Files:**
- Modify: `README.md` (the "## Shortcuts" section, the `[keys]` row in "## Config", Roadmap), `docs/site-assets/SCREENSHOTS.md`, `docs/HANDOFF.md`

- [ ] **Step 1: Full check:** `go test ./...`, `go vet ./...`, `GOOS=darwin go vet ./internal/config/`, and a Windows build `go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze` in a temp folder (never over the owner's copy).
- [ ] **Step 2: README:** after the Shortcuts table, add a paragraph. It says any of them can be changed in Settings > Shortcuts (click, press the keys, saved at once; a clash asks; keys with no Ctrl, Alt or Cmd are refused; plain Ctrl+letter warns that the shell loses it; reset per row or all) as well as under `[keys]` in `config.toml`. It also says `none` leaves an action without a shortcut (still in the palette), and that Settings always keeps one. The `[keys]` row in "## Config" mentions `none`. Roadmap: editable shortcuts are shipped.
- [ ] **Step 3: SCREENSHOTS.md:** note that the Settings > Shortcuts screenshot (if the lander or README uses one) needs retaking for the new row layout. No fakes.
- [ ] **Step 4: HANDOFF:** a dated entry at the top of "Where things stand": built on `feat/shortcuts-edit`, plan and spec paths, release 0.31 next. Owner checks: change Find to Ctrl+Alt+F and use it without a restart; Replace a clash; Ctrl+R warning; Reset all; Washi readability; and on the Mac, Cmd keys recorded as `cmd`. Under "Decisions so far": `none`, the order-independent clash rule, Settings always bound, and the page writes only through `config.SetKeys`. Under "Known gaps": no chords or second shortcuts, no tooltips on reset, hover not shown (focus only), and keys `parseShortcut` can't name (Insert, numpad, `+`) can't be recorded.
- [ ] **Step 5: Commit** `docs: Record editable shortcuts`

## Rulings made while planning

Each is cheap to overturn in its own task.

1. **Pressing an action's default removes its line** instead of writing the default out, so the row goes back to unchanged and the file stays minimal. (Tasks 5, 6)
2. **Settings is refused at once.** A press, Replace or Reset that would take Settings' keys shows "Settings needs a shortcut. Pick other keys." without asking first. A hand-written `open_settings = "none"` keeps the default with a warning, so `binds["open_settings"]` always exists. (Tasks 2, 6, 7)
3. **Esc with any modifier cancels.** A press with the Windows key (or Super on macOS) is ignored on both OSes, and the row keeps recording. (Task 5)
4. **Refused wording:** on Windows "needs Ctrl or Alt", on macOS "needs Ctrl, Option or Cmd", matching the caps the page shows there (`formatShortcut` labels Alt "Option" on macOS). The spec says Alt. With no old binding the sentence ends "<action> stays without a shortcut." (Task 6)
5. **No hover in Settings today,** so the keys button's hairline and `CapHover` fill show on keyboard focus only. The reset button has no tooltip, because Settings has no tooltip component; the ↺ and the "changed" tag carry the meaning. (Task 8)
6. **Recording ends** when the section changes, Settings closes or the window loses focus. The strip clears with it. (Task 5)
7. **Strip colours:** refused `DiffDelFG`, clash `accent`, warning the theme's `Needs` amber. Each is a fill at alpha 0x1a and a border at 0x59, and the text is the page title colour through `render.Readable` over the composite. No new theme tokens. (Task 8)
8. **Only a bare `[keys]` header is edited.** `["keys"]`, an inline table or dotted keys turn editing off with the "move them into a [keys] table" hint. (Task 1)
9. **A save error and the Settings refusal** use the refused strip, with OK. (Tasks 5 to 7)
10. **"Reset all to defaults"** shows whenever editing is on (even with nothing changed, where it removes nothing), right-aligned under the last shortcut card, above "Command line". (Task 7)
11. **A plain save shows no strip;** the new caps and the "changed" tag are the feedback. (Task 6)
12. **A new `[keys]` table** goes at the end of the file after one blank line. New lines go after the table's last assignment. (Task 1)
13. **`buildKeymap` adds names in sorted order,** so a clash left over from a hand edit always gives the same keymap. (Task 2)
14. **Editing-off hints** have the fixed wording in Task 5, one per cause (no config folder, broken file, keys outside `[keys]`). The rows keep showing the active keys. (Task 5)
