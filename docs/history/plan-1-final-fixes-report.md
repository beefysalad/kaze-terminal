# Final fix wave, Plan 1 (2026-10-28)

Branch `feature/plan-1-terminal-core`, base fa875cb, head 60df039. Not pushed. Author patrick-gogo, no AI attribution.
Env: Go 1.27.0, `GOTMPDIR=C:/Users/john/Desktop/kaze-wt/gotmp-main`, Smart App Control retries with `-ldflags "-s -w -X main.r=<random>"`. kaze was never launched; no global input was sent.

## 1. Shell env: kaze's own terminal identity (bcc82b1)
- Test: `internal/tabs/manager_test.go` `TestNewTabSetsKazeTerminalIdentity` (next to `TestNewTabSetsTabIDEnv`). Sets TERM_PROGRAM=vscode, TERM_PROGRAM_VERSION, WT_SESSION, WT_PROFILE_ID, TERM=dumb, COLORTERM="" and checks the tab env (case-insensitive name match).
- RED: the 3 inherited vars passed through; TERM=dumb, COLORTERM=, TERM_PROGRAM=vscode.
- GREEN: `baseEnv` drops KAZE_TAB_ID, TERM, COLORTERM, TERM_PROGRAM, TERM_PROGRAM_VERSION, WT_SESSION, WT_PROFILE_ID by name with `strings.EqualFold`, then appends TERM=xterm-256color, COLORTERM=truecolor, TERM_PROGRAM=kaze. Inherited TERM/COLORTERM are dropped too so each appears exactly once. tabs package passes.

## 2. font_family warns (0b66f89)
- Test: `internal/config/config_test.go` `TestFontFamilyWarnsNotSupported`: exactly one warning "font_family is not supported yet; using the bundled font", and the value is kept.
- RED: no warnings.
- GREEN: a non-empty string warns (an empty string means the bundled font, so it stays silent). Field kept; its comment updated.

## 3. No sync hold when nothing is drawn (ac176cc)
- Tests: `internal/render/lifecycle_test.go` `TestSyncHoldDoesNotHoldAFreshRenderer` (draw, start mode 2026, Release, frame must redraw 24 rows, next frame is held at 0) and `TestSyncHoldDoesNotHoldTheFirstFrame` (first frame ever during sync draws 24).
- RED: both frames redrew 0 rows.
- GREEN: `snapshot` holds only when `r.cols != 0 && len(r.strips) != 0`. Existing `TestSynchronizedOutputHoldsFrame` still passes. No change to window.go was needed (Release already zeroes cols/strips).

## 4. .gitignore (d8a6d30)
- Added `*.exe` and `capture.bin`; existing entries kept. No tracked file matches. No test (no behavior).

## 5. Japanese system font fallback (3074d12)
- Test: `internal/render/fallback_test.go` `TestRareKanjiFallsBackToSystemJapaneseFont` (Windows only, skips without YuGothM.ttc): U+20B9F and U+4FF1 are not in the bundled fonts, come from a system face, draw ink, come from YuGothM.ttc, and are two cells wide.
- RED: both resolved to face 3 (faceMissing).
- GREEN: Windows list appends YuGothM.ttc then msgothic.ttc last; macOS list appends `/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc` then `/System/Library/Fonts/Hiragino Sans GB.ttc` last. Lazy loading and the 32-font cap are unchanged; a missing file is skipped by the existing os.Open check.

## 6. Message box when no shell can start (5486b93)
- Tests: `cmd/kaze/fatal_test.go` `TestFatalErrorIsShownAndLogged` (swaps the `showError` func var and the log writer; checks one call with title "kaze" and the error text, and the stderr log); `cmd/kaze/fatal_windows_test.go` `TestMessageBoxIsAvailable` (user32 MessageBoxW resolves; not called, so no dialog blocks the test).
- RED: build failed (`showError`, `reportFatal`, `procMessageBoxW` undefined).
- GREEN: `cmd/kaze/fatal.go` (`showError` var, `reportFatal` logs then shows), `fatal_windows.go` (MessageBoxW via syscall, MB_OK|MB_ICONERROR|MB_SETFOREGROUND, no cgo), `fatal_other.go` (no-op; the stderr log is the message). main.go calls `reportFatal(err)` before `os.Exit(1)` for any error from `ui.Run`.

## 7. README (60df039)
- Rewritten: intro, status (Windows works, macOS untested), requirements (Go 1.27.0, Xcode CLT for cgo), build/run, Smart App Control, full shortcut table with macOS variants and Alt/Option+Enter, mouse, config path and keys (font_family marked not supported), env vars kaze sets, PSReadLine setup, benchmarks link, roadmap, and the two placeholders `<!-- LICENSE: pending owner -->` and `<!-- MODULE PATH: pending owner -->`.
- Claims checked against code: default bindings (`internal/ui/logic.go`), Alt prefix encoding (`internal/input/keys.go`), click counting and Shift override (`internal/ui/selection.go`, `window.go`), wheel and alt-screen behavior, config keys and limits, default shells.

## Final verification
- `go vet ./...`: clean. `gofmt -l internal cmd`: clean. `GOOS=darwin go vet cmd/kaze/fatal.go cmd/kaze/fatal_other.go`: clean (full darwin vet of cmd/kaze needs cgo).
- `go test ./... -count=1`, per package with SAC retries: all 12 test packages ok, 3 with no test files.

## Concerns
- The macOS Hiragino filename is from macOS docs, not checked on a Mac; Hiragino Sans GB is a Simplified Chinese font and may not cover rare Japanese kanji. Both are skipped when missing. Verify at the Mac milestone.
- Any error from `ui.Run` (not only a shell start failure) now shows the message box, since that is the only fatal path; the wording "kaze has to close:" fits both.
- A rune missing from every font now also opens Yu Gothic (~14MB, read through the file, not loaded into memory) and MS Gothic once per process.
- README Shift+Enter note reflects today's encoding (Shift+Enter sends CR).

## Owner decisions (head 354894d, base 60df039)

Commits, author patrick-gogo, no attribution, history not rewritten, not pushed:
- 285a7a2 `refactor: Rename the module to kaze-terminal`: go.mod plus every import in 50 files (internal/, cmd/, tools/; bench/flood and spike/gio import nothing internal), and the module path in the plan doc. `git grep -P 'patrick-gogo/kaze(?![-\w])'` is now empty. Binary and app name stay `kaze` (cmd/kaze).
- 28b5958 `docs: Add the MIT license`: standard MIT `LICENSE`, `Copyright (c) 2026 patrick-gogo`. README License section: MIT for the code; bundled fonts (JetBrains Mono, Noto Sans JP subset) keep their SIL OFL 1.1 files in assets/fonts (untouched), sources in SOURCES.txt. The section sits at the end of the README, where readers expect it.
- 9dc6a61 `docs: Add install instructions to the README`: an extra small commit (item 3 named none). Both placeholders removed; Install section with `go install github.com/patrick-gogo/kaze-terminal/cmd/kaze@latest` (plus the `-ldflags "-H=windowsgui"` variant for Windows), and "Build and run from a clone" with `git clone https://github.com/patrick-gogo/kaze-terminal.git`.
- 354894d `docs: Make the design docs generic for a public repo`.

### Grep (the given pattern, `git grep -n -i -E "sat\b|sir sat|サトル|V3-[0-9]|devbox|gauntlet|samurai|sony|obsidian|vault|gogo-it-lab|C:\\Users\\john"`)
Before: 20 text hits + 1 binary: spec (sir Sat x4, Obsidian vault, `feature/V3-3152`, devbox x2), look-midnight.html (V3-3152/V3-3160 x9, sony_reauthorize.py x3), plan (john@gogo-it-lab.com), README ("notes vault"). The given pattern's `C:\Users\john` part did not match through the shell escaping, so I also ran `users[\/]+john`, which found the spec repo path, the benchmark results path and test data in internal/render/selection_test.go.

Changes: sir Sat -> "a teammate" / "a colleague's terminal"; Obsidian vault -> "a notes vault (Patrick's knowledge base)"; `feature/V3-3152` -> `feature/TICKET-123`; mockup tabs -> TICKET-123/TICKET-124, `sony_reauthorize.py` -> `reauthorize.py`; devbox -> "a remote agent runner"; plan commit line -> "Commits use the repo's configured git identity."; spec repo path -> `<repo>` (e.g. `%USERPROFILE%\Desktop\kaze`); benchmark logs path -> `%USERPROFILE%\Desktop\kaze-wt\bench\`; test data `C:\Users\john\file.go` -> `C:\Users\me\file.go`.

After, hits left on purpose:
- README.md:183 "A graph view of my notes vault." and spec:140 "a notes vault ... not the whole vault": the requested neutral wording.
- assets/fonts/NotoSansJP-Regular-subset.ttf: binary font data matching by chance.
Also left (outside the pattern): "Patrick" across the docs (the owner's own name; not asked to change); "kaze-wt" in bench/run.ps1 and the benchmark doc (a relative sibling-folder default, `<parent>\kaze-wt\bench`, not a personal path); the mockup's "re-authorize path ... hold expiry before calling AUTH" sample agent text (no product or company name left).

### Verification
- `go build ./...`, `go vet ./...`: clean; Windows GUI build of cmd/kaze (`-H=windowsgui`) ok.
- `go test -count=1`, one package at a time with Smart App Control retries: bench/flood, cmd/kaze, internal/{bench,config,input,ptyx,render,screen,tabs,term,ui,vt} all ok; assets/fonts, spike/gio, tools/capture have no test files.
