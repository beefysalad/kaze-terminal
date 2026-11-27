# First-run tour, Workspaces list and Geist chrome Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a once-only first-run tour, a collapsed-row Settings Workspaces list, and Geist as the app's UI font (JetBrains Mono stays in the terminal and for code), in one PR.

**Architecture:** The tour is a new `internal/tour` package (first-run detection and the `tour.json` flag) plus an overlay in `internal/ui` modelled on the existing overlays (`win.set`, `win.board`, `win.docs`). The Workspaces list becomes a pure model (`wsListModel`) drawn by the existing settings `page` helpers. The font change splits `chromeFace` into a sans face (Geist) and a mono face; every chrome text call funnels through `chrome.labelDims` (`internal/ui/tabbar.go`), so the split starts there.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`gioui.org`), go-text/typesetting via Gio's shaper; Geist static TTFs (SIL OFL 1.1) from `vercel/geist-font`.

**Spec:** `docs/superpowers/specs/2026-11-26-first-run-tour-and-workspaces-list-design.md` (read it first; this plan only records decisions it leaves open). Mockups: https://claude.ai/artifact/Sr9HUr9nTQnNVyyRVvntVG (boards "First-run tour" and "Settings: Workspaces").

## Global Constraints

- Work only in this worktree on branch `feat/tour-workspaces-geist`. Never push to `main`. The only remote is `fork`; never push to anything else, and never set an upstream to `fork/main`.
- Commit messages: `type(scope): Verb ...`, **no AI attribution of any kind** (no `Co-Authored-By`, no "Generated with"); a hook rejects commits that have it.
- Strict TDD: write the test, watch it fail (RED), then make it pass. RED tests must fail safely: bounded sizes, timeouts, no unbounded loops or allocations.
- No emojis and no Unicode-symbol icons anywhere (copy, code, docs). Key labels like the command and shift keys in shortcuts are fine.
- Say **spaces**, never "sessions", in user-facing copy. No copy may claim agent-to-agent messaging or orchestration, or a docked agents pane.
- Fonts: Geist for UI chrome, JetBrains Mono for the terminal grid and for code (folders, branch names, commands, aligned columns); Regular and Bold plus SemiBold only; Noto Sans JP stays the Japanese fallback; never two fonts in one run of text. No other fonts, no new Go dependencies.
- Never inject global keyboard or mouse input when testing the GUI; test through code paths. Native callbacks never run Go or Gio code on the AppKit main thread (not touched here, do not regress).
- A failing or unreadable `tour.json` never shows an error and never blocks starting a shell.
- Run one package at a time when chasing a failure (`go test ./internal/<pkg>`); the full `./internal/ui` suite is slow (use `-run` filters while iterating).

## Review Focus

1. `tour.json` cannot be written (read-only config directory): the tour may show again next launch, with no error toast or crash. Test in Task 3.
2. Upgrade from an older Kaze (config.toml or sessions.json exists, no tour.json): no tour, flag written silently. Test in Task 3.
3. A very narrow window (under 480 dp) with the tour or the Workspaces list: card stays on screen, nothing overlaps, actions stay reachable. Tests in Tasks 6 and 10.
4. Zero workspaces and one workspace: the list shows the empty card as today, no filter chips, no search box. Test in Task 8.
5. Tab with no commands, a very long name, or CJK text in a workspace name: no crash, truncates with an ellipsis, "+N more" counts only typed-in commands. Test in Task 8.

---

## File Structure

- Create `assets/fonts/Geist-Regular.ttf`, `Geist-SemiBold.ttf`, `Geist-Bold.ttf`, `OFL-Geist.txt`; modify `assets/fonts/fonts.go`, `assets/fonts/SOURCES.txt`.
- Modify `internal/ui/tabbar.go` (face split), then the mono call sites (Task 5).
- Create `internal/tour/tour.go` (+ `tour_test.go`): first-run detection and the flag file.
- Create `internal/ui/tour_steps.go` (+ test): the cards' data and copy.
- Create `internal/ui/tour.go`, `tour_draw.go` (+ tests): overlay state, events, drawing.
- Modify `internal/ui/window.go` (hold state, call events and draw, startup check), `internal/ui/logic.go` and the commands list (the `actTour` palette action), `internal/ui/settings_about.go` (Replay button).
- Create `internal/ui/settings_wslist.go` (+ test): the Workspaces list model. Modify `settings_workspaces.go`, `settings_members.go`, `settings_workspaces_draw.go` for the new rows.
- Modify `README.md`.

---

### Task 1: Bundle Geist

**Files:**
- Create: `assets/fonts/Geist-Regular.ttf`, `Geist-SemiBold.ttf`, `Geist-Bold.ttf`, `OFL-Geist.txt` (from `vercel/geist-font`: `fonts/Geist/ttf/` and `OFL.txt`; fetch with `gh api repos/vercel/geist-font/contents/<path> -H "Accept: application/vnd.github.raw"`)
- Modify: `assets/fonts/fonts.go`, `assets/fonts/SOURCES.txt`
- Test: `assets/fonts/fonts_test.go`

**Interfaces:**
- Produces: `fonts.SansRegular`, `fonts.SansSemiBold`, `fonts.SansBold` (`[]byte`, embedded).

- [ ] **Step 1: Write the failing test** `TestBundledFontsParse` in `fonts_test.go`: for each of `MonoRegular, MonoBold, SansRegular, SansSemiBold, SansBold, CJK`, `opentype.Parse` succeeds and `len > 10_000`; `TestGeistLicenseShipped` reads `OFL-Geist.txt` and asserts it contains `SIL Open Font License`.
- [ ] **Step 2: Run** `go test ./assets/fonts` and confirm it fails to compile (`SansRegular` undefined).
- [ ] **Step 3: Add the three files and the license, embed them in `fonts.go` with a comment naming Geist, its license file and source, and add one entry to `SOURCES.txt` (version, URL, date 2026-11-26).**
- [ ] **Step 4: Run** `go test ./assets/fonts` and confirm PASS.
- [ ] **Step 5: Commit** `feat(fonts): Bundle Geist for the UI chrome`.

### Task 2: Split the chrome face into sans and mono

**Files:**
- Modify: `internal/ui/tabbar.go` (`chromeFace`, `newChrome`, `labelDims`), `internal/ui/titlebar_draw.go:80-92`
- Test: `internal/ui/chrome_face_test.go`

**Interfaces:**
- Produces: `const chromeFace = "Geist"` and `const monoFace = "JetBrains Mono"`; `func (c *chrome) labelMono(gtx layout.Context, s string, size unit.Sp, col color.NRGBA) (op.CallOp, image.Point)` and `func (c *chrome) labelMonoW(gtx layout.Context, s string, size unit.Sp, col color.NRGBA, w font.Weight) (op.CallOp, image.Point)`; `label`, `labelW` and `labelDims` now draw in Geist. `newChrome` registers Geist at Regular, SemiBold and Bold, JetBrains Mono at Regular and Bold, and the CJK fallback.
- Consumes: Task 1's `fonts.Sans*`.

- [ ] **Step 1: Write the failing tests:** `TestChromeLabelIsProportional` (width of `"iiiiiiii"` is less than width of `"WWWWWWWW"` through `label` at 13 sp) and `TestChromeMonoLabelIsFixedWidth` (the same two strings through `labelMono` have equal width). `TestChromeStillDrawsCJKAndSymbols` (labels containing `日本語` and the branch symbol used by `alertCard` produce non-zero width and do not panic).
- [ ] **Step 2: Run** `go test ./internal/ui -run 'TestChrome(Label|Mono|StillDraws)'`; confirm FAIL (`labelMono` undefined).
- [ ] **Step 3: Implement** the face constants, `newChrome` registration (keep `atChromeWeights` for the CJK face only) and the two mono label methods; `labelDims` takes the typeface from a small unexported helper so `labelMonoW` reuses it.
- [ ] **Step 4: Run** the three tests plus `go test ./internal/ui -run 'Titlebar|TitleBar'`; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw the chrome in Geist behind a sans/mono face split`.

### Task 3: First-run detection and the flag file

**Files:**
- Create: `internal/tour/tour.go`
- Test: `internal/tour/tour_test.go`

**Interfaces:**
- Produces: `type Decision int` with `Show`, `Skip`; `func Check(dir string) Decision` (dir is the config directory, the one holding `config.toml`; pure read, creates nothing); `func MarkSeen(dir string) error` (writes `tour.json` as `{"seen": true}` atomically; creates `dir` if missing).
- Rules from the spec: `Skip` if `tour.json` exists (or is unreadable or corrupt, **call**: treat as seen); `Skip` if it is absent but `config.toml` or `sessions.json` exists; `Show` only when none of the three exist; `dir == ""` is `Skip`.

- [ ] **Step 1: Write the failing tests** (all in `t.TempDir()`): `TestCheckFreshInstallShows`; `TestCheckUpgradeSkipsConfigToml`; `TestCheckUpgradeSkipsSessionsJSON`; `TestCheckSeenSkips`; `TestCheckCorruptFlagSkips`; `TestCheckEmptyDirArgSkips`; `TestCheckCreatesNothing` (directory listing unchanged after `Check`); `TestMarkSeenRoundTrip` (then `Check` is `Skip`); `TestMarkSeenReadOnlyDirReturnsError` (chmod 0500, skipped on Windows, returns an error and does not panic).
- [ ] **Step 2: Run** `go test ./internal/tour`; confirm FAIL (package empty).
- [ ] **Step 3: Implement** both functions with the standard library only; write through a temp file and `os.Rename`.
- [ ] **Step 4: Run** `go test ./internal/tour`; confirm PASS.
- [ ] **Step 5: Commit** `feat(tour): Detect a first install and remember the tour was seen`.

### Task 4: Tour cards (data and copy)

**Files:**
- Create: `internal/ui/tour_steps.go`
- Test: `internal/ui/tour_steps_test.go`

**Interfaces:**
- Produces: `type tourTarget int` (`tourNone, tourSidebar, tourWorkspaces, tourStatusBar, tourAgentsButton`); `type tourStep struct { Label, Title, Body string; Target tourTarget; Board bool; Hooks bool; SkipLabel, NextLabel string }`; `func tourSteps() []tourStep` returning the six cards in the spec's table order (Welcome, 1 of 5 to 4 of 5, then the hooks card last with `Hooks: true`, `SkipLabel: "Do this later"`, `NextLabel: "Finish"`; Welcome has `SkipLabel: "Skip tour"`, `NextLabel: "Show me around"`; the middle cards `"Skip tour"` and `"Next"`). The Agents board card has `Board: true, Target: tourAgentsButton`. Copy is the spec's table, in the owner's words, with no em dashes.

- [ ] **Step 1: Write the failing tests:** `TestTourStepsOrder` (six steps, Welcome first, the only `Hooks` step last, the only `Board` step is index 4); `TestTourCopyRules` (for every title and body: no emoji or symbol outside basic punctuation, no case-insensitive match for `session`, `orchestrat`, `message`, `pane`, and no em dash; "space" appears in the step 1 body); `TestTourWelcomeHasNoPitch` (Welcome body contains "Thank you" and does not contain "terminal").
- [ ] **Step 2: Run** `go test ./internal/ui -run TestTour`; confirm FAIL (undefined).
- [ ] **Step 3: Implement `tourSteps` and the types.**
- [ ] **Step 4: Run** the same tests; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Add the first-run tour's cards`.

### Task 5: Move code text to the mono face and fix width assumptions

**Files:**
- Modify: `internal/ui/controls.go:376` (`ctlText` field editor: keep mono, it holds paths and commands), `internal/ui/doc_layout.go` (code blocks and inline code keep mono; prose becomes Geist), `internal/ui/statusbar_draw.go`, `internal/ui/statusbar.go` (the `measure(s, mono bool)` hook must pick the real face), `internal/ui/tabbar.go` strip titles, `internal/ui/sidebar*.go` (folder and branch lines mono, names Geist), `internal/ui/settings_members.go` (command chips and folder mono), `internal/ui/palette.go` (shortcut keys mono, query and rows Geist), `internal/ui/settings_keys_draw.go` (shortcut keys mono)
- Test: extend `internal/ui/statusbar_render_test.go`, `sidebar_render_test.go`, `settings_sheet_render_test.go`, `palette_*` render tests; new `internal/ui/chrome_widths_test.go`

**Interfaces:**
- Consumes: Task 2's `labelMono` and `labelMonoW`. Rule of thumb in the spec: prose Geist, code mono.

- [ ] **Step 1: Write the failing tests:** `TestStatusBarMeasuresRealFace` (the closure passed to `statusPlace` returns a larger width for the same string with `mono=true` than Geist `mono=false` for a letter-heavy string, and neither is `len(s)*cellWidth`); `TestChromeNoOverlapNarrow` (render the sidebar row, a tab-strip title, and each status bar compaction state at widths 320, 480 and 900 dp with long names; assert no two text rects intersect and none extend beyond their container; reuse the existing render-test helpers).
- [ ] **Step 2: Run** the new tests; confirm FAIL where text is still measured as monospace.
- [ ] **Step 3: Migrate the listed call sites by area, one area per sub-commit if large; replace any `cell width * len(s)` column maths in chrome with measured widths.** The terminal grid (`internal/render`) is not touched.
- [ ] **Step 4: Run** `go test ./internal/ui -run 'Status|Sidebar|Chrome|Palette|Settings'` and `go test ./internal/render`; confirm PASS and unchanged render tests.
- [ ] **Step 5: Commit** per area, `fix(ui): Measure Geist text in <area> and keep code in mono`.

### Task 6: Tour overlay (state, drawing, keys)

**Files:**
- Create: `internal/ui/tour.go`, `internal/ui/tour_draw.go`
- Modify: `internal/ui/window.go` (field `tour tourState`; call `win.tourEvents(gtx)` before `win.paletteEvents`'s peers so it takes input first while open; draw last in `frame`; target rects recorded where `sideRect`, the status rect and the agents button rect are known)
- Test: `internal/ui/tour_test.go`, `internal/ui/tour_render_test.go`

**Interfaces:**
- Consumes: Task 4's `tourSteps`, Task 3's `tour.MarkSeen`, existing `win.toggleBoard()`, `win.closeBoard()`, `win.board.open`.
- Produces: `type tourState struct { open bool; step int; replay bool; boardOpenedByTour bool; hooks []tourHook }`; `func (win *window) openTour(replay bool)`; `func (win *window) closeTour()`; `func (win *window) tourNext()`, `tourBack()`; `func tourCardRect(win image.Rectangle, target image.Rectangle, card image.Point) image.Rectangle` (pure placement: card stays inside the window with an 16 dp gutter).
- Behaviour: `openTour` writes `seen` (unless `replay`); Esc skips, Enter or Right goes next, Left goes back; any close route calls `closeTour`, which closes a board that the tour opened. Step 4 opens the board and sets `boardOpenedByTour`; Back, Next and close restore it. While open, key events go to the tour and not the terminal; on close focus returns to the terminal. The ring is skipped when its target rect is empty (hidden sidebar).

- [ ] **Step 1: Write the failing tests** (no real window; use the package's existing test window helper): `TestTourOpenMarksSeenOnce`; `TestTourReplayDoesNotMarkSeen`; `TestTourKeysEscEnterLeft`; `TestTourBoardStepOpensAndRestoresBoard`; `TestTourCloseReturnsFocusToTerminal`; `TestTourCardRectStaysInsideWindow` (table of window sizes down to 320x240 dp and targets at each corner); `TestTourRingSkippedForEmptyTarget`.
- [ ] **Step 2: Run** `go test ./internal/ui -run TestTour`; confirm FAIL.
- [ ] **Step 3: Implement** the state, events and drawing with the same card styling as `alertCard` and the dialog code in `settings.go`; text through the sans `label` helpers, step counter dots as in the mockup.
- [ ] **Step 4: Run** `go test ./internal/ui -run TestTour` and the existing window tests for focus and board (`-run 'Board|Focus'`); confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Add the first-run tour overlay`.

### Task 7: The hooks card

**Files:**
- Modify: `internal/ui/tour.go`, `internal/ui/tour_draw.go`
- Test: `internal/ui/tour_hooks_test.go`

**Interfaces:**
- Consumes: the existing Claude and Codex status checks behind `settings_claude.go` and `settings_codex.go`, and the existing confirm dialog flow (`win.openFix`, `claudesetup.Fix`, `codexsetup`); Fix must stay behind that dialog.
- Produces: `type tourHook struct { Agent string; State tourHookState }` with states `hookReady`, `hookNeedsFix`, `hookNotInstalled`; `func (win *window) tourHookStates() []tourHook` (read-only, re-read each time the card is shown).

- [ ] **Step 1: Write the failing tests:** `TestTourHookStatesFromFixtures` (temp `$HOME` with no agent config files gives both `hookNotInstalled`; with a Claude settings file lacking hooks gives `hookNeedsFix`; with hooks present gives `hookReady`); `TestTourHooksCardFixOpensConfirmDialog` (pressing Fix sets the existing confirm dialog and changes no file); `TestTourFinishAndLaterChangeNoFiles` (snapshot the temp home, press Finish and Do this later, tree is identical).
- [ ] **Step 2: Run** and confirm FAIL.
- [ ] **Step 3: Implement the states and card rows** (a status dot, the agent name, "ready", or a Fix button; "not found" line for a missing agent with no button).
- [ ] **Step 4: Run** `go test ./internal/ui -run 'TestTourHook|TestTour'`; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Offer live-status hooks on the tour's last card`.

### Task 8: Start the tour once, and replay it

**Files:**
- Modify: `internal/ui/window.go` (`Run`: call `tour.Check(filepath.Dir(win.cfgPath))` before `initRestore`, store the decision; open the tour after the first frame when allowed), `internal/ui/logic.go` and the commands list (`actTour` palette action "Show the tour"; wire like an existing parameterless action and satisfy `commands_test.go`), `internal/ui/settings_about.go` ("Replay the tour" ghost button id `about_tour`, handled in the page's act function)
- Test: `internal/ui/tour_start_test.go`

**Interfaces:**
- Consumes: Tasks 3 and 6. Gate (spec): show only if `Check` returned `Show`, not bench mode, no startup workspace configured, and nothing was restored; otherwise do nothing and leave `seen` unwritten so a later plain launch shows it.

- [ ] **Step 1: Write the failing tests:** `TestTourStartsOnFreshInstall`; `TestTourSkippedOnUpgrade` (config.toml present: never opens, no `tour.json` write other than the silent `seen`); `TestTourSkippedInBenchMode`; `TestTourSkippedWhenWorkspaceRunsAtStartup`; `TestTourSkippedWhenSessionsRestored`; `TestPaletteHasShowTheTour`; `TestAboutReplayOpensTourWithoutMarkingSeen`; `TestTourReadOnlyConfigDirDoesNotCrash` (Review Focus 1: `MarkSeen` error is swallowed, no toast).
- [ ] **Step 2: Run** `go test ./internal/ui -run 'TestTourStart|TestPaletteHasShow|TestAboutReplay|TestTourReadOnly|TestCommands'`; confirm FAIL.
- [ ] **Step 3: Implement the startup hook, the action and the About button.**
- [ ] **Step 4: Run** the same set; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Show the tour on first install and let people replay it`.

### Task 9: Workspaces list model

**Files:**
- Create: `internal/ui/settings_wslist.go`
- Test: `internal/ui/settings_wslist_test.go`

**Interfaces:**
- Consumes: `workspaces.Document`, `win.wsBadge(id)`, `win.listMember(t)`.
- Produces: `type wsFilter int` (`wsAll, wsOpenOnly, wsClosedOnly`); `type wsListRow struct { ID, Name, Initial, Sub, State, Tone, RunLabel string; Dots []string; Tabs []wsListTab; Open bool }`; `type wsListTab struct { Title, Folder, Cmd, More, Wait, Color string }`; `type wsListModel struct { Rows []wsListRow; Counts [3]int; ShowSearch bool; Empty bool }`; `func buildWSList(win *window, filter wsFilter, query string, open string) wsListModel`.
- Rules: `Empty` when there are no workspaces; `ShowSearch` when there are more than 5; "Open" means any state other than "Closed"; search matches workspace name and tab titles, case-insensitive; `More` is `+N more` where N counts only non-blank commands beyond the first; `Cmd` is `no commands` for none; `Sub` is `N tab(s)` plus ` · runs at startup` when set; `RunLabel` is `Run` for Closed and `Go to tabs` otherwise.

- [ ] **Step 1: Write the failing tests:** `TestWSListEmptyAndSingle` (0 and 1 workspaces: `Empty`, no search; Review Focus 4), `TestWSListFilterCounts`, `TestWSListSearchNameAndTab`, `TestWSListShowSearchOverFive`, `TestWSListMoreCountsRealCommands` (blank commands ignored), `TestWSListNoCommandsTab`, `TestWSListLongAndCJKNames` (no panic, names untouched in the model; truncation is the drawer's job; Review Focus 5), `TestWSListOneOpenAtATime`.
- [ ] **Step 2: Run** `go test ./internal/ui -run TestWSList`; confirm FAIL.
- [ ] **Step 3: Implement `buildWSList`** as a pure function of window state; list-view state (filter, query, open id) lives on `win.wsp`.
- [ ] **Step 4: Run** `go test ./internal/ui -run TestWSList`; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Add the Workspaces list model`.

### Task 10: Draw the collapsed rows

**Files:**
- Modify: `internal/ui/settings_workspaces.go` (`listRows` uses the model; action ids `ws.toggle` with `arg` row index, `ws.filter` with the filter, `ws.search`, `ws.menu`, `ws.startup` for Run at startup; keep `ws.run`, `ws.edit`, `ws.delete` and their existing handlers and delete confirmation), `internal/ui/settings_members.go` (`memberRow` and `memberNarrow` draw the closed and open rows), `internal/ui/settings_workspaces_draw.go` (chips and search above the list)
- Test: `internal/ui/settings_workspaces_test.go`, `internal/ui/settings_sheet_render_test.go`

**Interfaces:**
- Consumes: Task 9's `buildWSList`; Task 2's sans and mono labels (command chips and folders mono, names and state Geist); the existing `p.hit`, `p.ctrl` and `rows` helpers.
- Produces: the visuals in the approved mockup. Closed row: chevron, letter tile, name, sub line, one color dot per tab, state pill, Run or Go to tabs, "..." menu. Open row: tab lines then Edit and Delete. Under 480 dp: `memberNarrow` keeps working: dots drop first, chips wrap, nothing overlaps (Review Focus 3).

- [ ] **Step 1: Write the failing tests:** `TestWSListToggleOpensOneRow`, `TestWSListRunStillRunsFromClosedRow`, `TestWSListDeleteStillConfirms` (the existing deletion dialog appears; if there is none, add it here and note it in the commit), `TestWSListMenuHasEditDeleteStartup`, `TestWSListRenderNarrowNoOverlap` (widths 320, 480, 900 dp; many workspaces; long names), `TestWSListSearchBoxOnlyOverFiveRendered`, and keep every existing `settings_workspaces_test.go` editor test passing untouched.
- [ ] **Step 2: Run** `go test ./internal/ui -run 'TestWSList|Workspaces'`; confirm FAIL.
- [ ] **Step 3: Implement the drawing and the new action ids; leave the editor (`wsDraw` draft path) unchanged.**
- [ ] **Step 4: Run** `go test ./internal/ui -run 'TestWSList|Workspaces|Settings'`; confirm PASS.
- [ ] **Step 5: Commit** `feat(ui): Collapse the Workspaces list into one-line rows`.

### Task 11: Docs and whole-branch verification

**Files:**
- Modify: `README.md` (the tour, the new Workspaces list, the font change; correct nothing else), `docs/superpowers/specs/2026-11-26-first-run-tour-and-workspaces-list-design.md` only if the build forced a spec change (record it)
- Test: none new

- [ ] **Step 1: Update the README sections** for the tour (first launch, replay from About and the palette), the Workspaces list, and the fonts.
- [ ] **Step 2: Run** `gofmt -l .` (expect no output), `go vet ./...`, `go build ./...`, then `go test ./internal/tour ./internal/config ./internal/workspaces ./assets/... ./internal/render` and `go test ./internal/ui` (allow it to run for several minutes; if it exceeds the timeout, run `-run` groups: Tour, WSList, Status, Sidebar, Chrome, Settings, Palette, Board). All PASS.
- [ ] **Step 3: Smoke-check a bundled build** per CLAUDE.md: build with `tools/package-macos.sh`, run the binary with a temporary `HOME` and confirm the process stays up after startup and the tour appears on a fresh `HOME` and not on a second launch (no synthetic input).
- [ ] **Step 4: Commit** `docs: Describe the tour, the Workspaces list and Geist`.

### Task 12: Push the branch and open the PR

- [ ] **Step 1:** `git push fork feat/tour-workspaces-geist` (never `origin`, never `main`).
- [ ] **Step 2:** Open a PR with `gh pr create --repo beefysalad/kaze-terminal --base main` titled `feat(ui): Add the first-run tour, a calmer Workspaces list and Geist chrome`, whose body is plain text (what changed, how to check it, the screenshots to retake if any), with no AI attribution lines.
- [ ] **Step 3:** Report the PR link to the owner. Do not merge, and do not cut a release (the repo's release steps start only after the owner merges).
