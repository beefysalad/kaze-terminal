# Task 13 report: Tabs and the main window

Status: DONE. Worked in the main checkout on feature/plan-1-terminal-core (base c84380a).

## Commits
- dec3bc2 feat(tabs): Add the tab manager
- 6a82895 fix(config): Reject bad font sizes and empty shells, bind real key actions
- f949ca3 feat(ui): Add tabs and the main window

## What was built
- `internal/tabs/manager.go`: the brief's Manager interface, plus `Size()`, `Empty()` and `CloseAll()`.
  - Env is os.Environ() with any inherited KAZE_TAB_ID removed (matched ignoring case, as Windows does), plus `KAZE_TAB_ID=<id>`. IDs are never reused.
  - Title and Exited are refreshed by `Tabs()` and `Active()` on the UI goroutine.
  - Each session's onChange goes through `notify`. The active tab asks for a frame. A background tab asks only when its title or exit state changed. That is one lock and a string compare on the notifier goroutine.
  - Closing the active tab activates its right neighbour, or the left one when it was last. `Close` kills the PTY in a goroutine because ConPTY can block for up to a second. `CloseAll` (on window destroy) closes all tabs in parallel and waits.
- `internal/ui/logic.go`: pure, tested logic.
  - `gridSize` (minimum 1x1, capped at 4096 like term).
  - `wheel.notches`: maps Gio's sign (Y > 0 is down) to lines > 0 is up, and adds up touchpad deltas.
  - `parseShortcut`, `buildKeymap`: config `[keys]` over the defaults. A bad or clashing shortcut keeps the default and adds a warning.
  - `routeKey`, `textGate`, `committedText`/`editCommits` (IME), `barHits`, `applyColors`, `tabLabel`, `injectWarnings`.
- `internal/ui/tabbar.go`: Midnight chrome using Gio's shaper over the bundled JetBrains Mono and Noto Sans JP.
  - Tab bar sits under the native title bar (R20). Chrome #0b0d12, active tab #0f1117 with #ffffff text, 9dp rounded top corners and a #8b7bff dot. Inactive text is #8a90a2.
  - Each tab has a ×, and there is a + after the tabs. Left-click switches. Middle-click or × closes.
  - Status bar is #0b0d12 with a #1d2230 hairline and #7d8394 text: "kaze <MB> · <N> tabs" on the right.
- `internal/ui/window.go`: the event loop.
  - The first shell starts on the first frame, at the real grid size.
  - A DPI change rebuilds the renderer at `PxPerSp * font_size`.
  - Rows and columns are computed from the area between the tab bar and the status bar (minus 8dp padding), then `ResizeAll` runs.
  - Only the active tab is drawn. Switching tabs calls MarkAllDirty on the shown tab, so the shared renderer's cell diff redraws what differs. One renderer and one glyph cache serve all tabs.
  - Focus: ConfigEvent.Focused calls SetFocused, and EncodeFocus goes to the active tab. Switching tabs also sends focus-out to the old tab and focus-in to the new one.
  - Wheel: Windows counts 120 per notch, at 3 lines per notch; macOS counts cell-height pixels per line. In mouse mode the wheel uses EncodeWheel. Otherwise it scrolls the view with SetViewOffset under the lock (not on the alt screen).
  - Typing, pasting or encoded keys snap the view to the bottom.
  - Mouse press, release and drag are reported in mouse modes. Drag and Move filters are only registered while a mouse mode needs them, so hovering never wakes an idle window.
  - Paste reads the clipboard (`clipboard.ReadCmd`, at most 64MB) and writes `EncodePaste(text, modes)`.
  - The window title follows the active tab ("kaze · <label>").
  - Gio's editor state is reset after committed text so it doesn't grow forever.
  - The last tab closing calls `ActionClose`, which leads to DestroyEvent, then CloseAll, then exit.
- `internal/ui/mem_windows.go`: the status bar shows the process working set (K32GetProcessMemoryInfo). Other platforms fall back to Go runtime stats.
- `cmd/kaze/main.go`:
  - Loads the config. A TOML syntax error becomes a warning and defaults are used.
  - An empty Shell resolves to `ptyx.DefaultShell()` (R10). The working directory is the user's home.
  - The window is 1100x720dp. `var r` is for the Smart App Control `-X main.r=` rebuild trick.

## Rulings and carried-over items
- Keys:
  - Plain Ctrl+C sends 0x03.
  - Reserved actions are handled by kaze: new/close/next/prev tab, copy and paste.
  - Printable keys without Ctrl, Alt or Cmd are left to the text event, which carries the layout's real character. This covers Space too.
  - AltGr (Ctrl+Alt plus a printable key on Windows) is left to the text event.
  - Option-as-Alt: an Alt-encoded press suppresses the next text event until the next key event.
  - Gio sends Ctrl+? as "/" + Ctrl+Shift on a US layout. It is mapped to "?" and gives 0x7f.
- Config `keys` IS fed into the reserved table: `input.SetReserved(goos, keymap.shortcuts())` runs at startup, so a remapped default reaches the shell again. `KnownActions` is now exactly new_tab, close_tab, next_tab, prev_tab, copy and paste. The zoom actions were removed because nothing implements them.
- Config validation:
  - `font_size` rejects NaN, Inf and values <= 0 (default 14 plus a warning).
  - Values over 72 are capped at 72 with a warning.
  - `shell` must start with a non-blank program (`[""]` and `["  ", ...]` fall back to the default with a warning).
- Config warnings in the first tab: ConPTY sends `ESC[2J ESC[H` when the shell starts (captured with tools/capture), so text written into the screen would be wiped. Instead, `injectWarnings` prints dim "kaze: ..." lines, pushes them into scrollback and starts the view scrolled back by that many lines. They sit above the prompt until the first keystroke snaps the view down, and they stay in history after that. A test covers the 2J case.
- The 1MB paste test moved from Task 11: `internal/term/paste_write_test.go` writes 1MB through Session.Write to a PTY whose Write blocks, and checks it returns in under 5ms. It passes.
- Bug found and fixed in ptyx: go-pty resolves a bare argv[0] against `Dir` instead of PATH once Dir is set. Running `pwsh` with Dir=home failed with "C:\Users\john\pwsh not found". `ptyx.Start` now runs LookPath on bare names when dir is set. Test: `TestBareProgramNameWithWorkingDir` (RED, then GREEN).
- R18: tests use bounded sizes and 5s timeouts. No test allocates by a size under test.

## TDD evidence
- tabs: 11 tests, all RED against a compiling stub, then GREEN. Passes with -count=20.
- ui logic: 12 tests, all RED against stubs, then GREEN. One real bug was caught in GREEN: Gio names keys like ⏎ and ↑ with a single rune, so they looked printable. Fixed with a symbol-key set. Warning injection: RED, then GREEN. Passes with -count=20.
- config: 3 new tests RED (NaN, Inf and huge sizes; empty shell; KnownActions), then GREEN.
- input: `TestSetReservedReplacesThePlatformTable` RED (compile), then GREEN.
- Full suite: all 9 test packages pass. Some packages needed reruns with `-ldflags "-X main.r=<random>"` because Smart App Control blocked their test exes. `go vet ./...` and gofmt are clean.

## Manual run (Windows 11, pwsh 7.6.9; screenshots in C:\Users\john\Desktop\kaze-wt\captures\)
How it ran: a GUI build (-H=windowsgui, plus random -X when blocked). A PowerShell script used SendKeys and took screenshots with System.Drawing CopyFromScreen of the window rect.
- `02-four-tabs.png`: 4 tabs from Ctrl+Shift+T. Midnight tab bar with the active-tab dot, × on each tab, + button. Status bar shows "kaze 23MB · 4 tabs" (Go-heap number in that build; it now shows the working set).
- `03-claude-git-japanese.png`:
  - `git log --oneline -3` shows in colour (yellow hashes, green/cyan refs).
  - `Get-Content -Encoding utf8 jp-utf8.txt` shows 日本語のテスト：こんにちは、世界。 / 全角ＡＢＣ　半角abc　カタカナ　漢字. Wide characters are aligned with no gaps.
  - PSReadLine syntax colouring works.
  - `claude --version` printed "2.1.312 (Claude Code)" in the first run (seen in that run's screenshot, later overwritten). In the second run's frame it is scrolled out of view.
- `04-ctrl-tab-to-first.png`: Ctrl+Tab wraps from the last tab to the first.
- `05-exit-marker.png` (real-shell check): typing `exit` in a pwsh tab printed "[process exited 0]". The tab stayed open and readable, labelled "pwsh.exe · exited". The window title became "kaze · pwsh.exe · exited".
- Closing all tabs with Ctrl+Shift+W quit the app (first run: HasExited=True). The pwsh process count was 11 before and 11 after, so no shells leaked.
- Numbers:
  - Window shown in ~0.8s on a warm launch (4.6s on a first launch, which looked like the Smart App Control scan).
  - Working set ~135MB, private ~166MB with 4–6 tabs.
  - Idle CPU over 5s: 62ms in the clean run.
- Input check: with a temporary debug log, every one of 41 SendKeys characters arrived as an EditEvent in order, and Gio's editor state reset to 0..0 after each one. The debug log was removed before the commit.
- Caveat: during the second scripted run someone typed in and clicked the kaze window: "cl", "dir", "claude", and the + button. That run ended with 6 tabs, `06-*.png` shows the typed "cldir", and the final close loop didn't empty it, so the script killed it. In the first run a few characters looked dropped, and the 4th tab was missing. Most likely the same interference. The second run's log shows no input loss from kaze.
- All kaze and debug processes were closed and the scratch exes removed. `kaze.exe` (built with `go build -o kaze.exe ./cmd/kaze`, gitignored) is in the repo root.

## Concerns / follow-ups
1. Copy is a no-op (`TODO(Task 12)` in window.go `onKey`). Selection and copy wiring wait for Task 12. `Layout` receives `nil` for the selection.
2. `scrollback_lines` from config is not wired: term.NewSession uses screen.New's default cap. Wiring it needs a term API (e.g. a scrollback parameter). `font_family` is ignored; only bundled fonts are used.
3. IME: committed text is sent and preedit is held back, but the preedit is not drawn in the grid and the IME candidate window is not placed at the cursor (no key.SelectionCmd caret bounds). Japanese IME typing should work but looks plain. Not tested with a real IME.
4. Mouse release always reports button 0. Gio reports the buttons still held, not the released one; SGR apps mostly ignore it. Alternate-scroll (wheel to arrows on the alt screen without mouse mode) is not done; the wheel does nothing there.
5. `go build -o kaze.exe ./cmd/kaze` makes a console-subsystem exe, so a console window appears next to kaze. Add `-ldflags -H=windowsgui` for daily use (log output is then lost).
6. Tab overflow: tabs that don't fit are not drawn. They stay reachable with Ctrl+Tab. There is no scrolling or shrinking yet.
7. macOS paths (Option-as-Alt, Cmd shortcuts, pixel wheel units) are unit-tested through routeKey only, not run. Cross-vet for darwin needs cgo.
