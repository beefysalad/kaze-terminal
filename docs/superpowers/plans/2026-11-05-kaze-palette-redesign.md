# Palette Redesign (sessions first, skills behind `/`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The palette opens short and about sessions (waiting, sessions, up to 3 recent skills, "Browse all N skills"), turns into a grouped skill picker on `/` or Tab, and gets the approved mockup's look in all five themes, with no more cost per frame than today.

**Architecture:** The model stays a pure function: `buildPalette(palInput) []palItem` in `internal/ui/palette_model.go`, now with two scopes derived from the query text (a leading `/` is Skills). Skill grouping (`groupSkills`) is a second pure function over the cached catalog, run once per catalog load, not per frame. Rows carry their group's header text, so the drawing draws a header wherever the group changes. Keys and the scope switch live in `palette_window.go` (one helper, `setPaletteQuery`, does every scope change). Drawing in `palette.go` is split: the box, field row, switch, chip and footer in Task 5, the list rows in Task 6, both coloured from one cached `palInk` built through `render.Readable`.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`widget.Editor`, `paint.LinearGradientOp`, `gpu/headless` for opt-in captures), existing `internal/skills`, `internal/theme`, `render.Readable`.

**Spec:** `docs/superpowers/specs/2026-11-05-kaze-palette-redesign-design.md` (binding, including section 8). Look: `docs/design/palette-mockup.html` (v3). On behaviour the spec wins; on look the mockup wins; this plan's Rulings (end) settle what neither says.

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: at most 1000 skills (`skills.MaxSkills`), at most 20 sessions, every wait through the existing bounded helpers (`waitSkills`, `emitTo`, 3 s deadlines); no loop "until found" without a bound.
- **No global keyboard or mouse input.** Tests queue events on an `input.Router` (`pressKey`, `typeKeys`, `palType`, `clickAt`, `palMove`) and run frames with `sideFrame` / `sizedFrame`; never SendInput, never another window.
- **Running tests on Windows (Smart App Control):** `mkdir -p /c/Users/john/Desktop/gotmp` once. Every `go test` here means, in Bash,
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-B 0x$(printf '%04x%04x%04x%04x' $RANDOM $RANDOM $RANDOM $RANDOM)" <args>`
  or in PowerShell `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags ("-B 0x{0:x16}" -f (Get-Random -Maximum ([long]::MaxValue))) <args>`.
  "An Application Control policy has blocked this file" means **blocked, not failed**: rerun with a fresh hex value. Never turn Smart App Control off.
- **No C compiler on this Windows machine.** `./internal/ui` needs cgo on darwin, so `GOOS=darwin go vet ./internal/ui/` is left to the macOS CI job; `GOOS=darwin go vet ./internal/skills/` runs locally.
- **Lean (spec 5):** nothing runs while the palette is closed. The list is rebuilt exactly when it is today (each frame while open, and before Enter); `groupSkills` runs only when a scan delivers a new list; the action list is built once per open. A still open frame allocates no more than today's (Task 1 measures today, Task 7 checks).
- **Colours only from theme tokens**, no new `[colors]` keys: `ToastBG`, `ToastLine`, `ToastFG`, `ToastGo`, `Muted`, `Accent`, `Needs`, `Done`, `Border`, `Chrome`, `CapGlyph` and the theme's ANSI colours (`c.ansi`). Every text through `render.Readable` on the surface it sits on: text at least 4.5:1, marks and tiles at least 3:1, in all five `theme.All()` themes.
- **Fonts:** `sansFace` (Manrope) for labels, descriptions, the editor, status words, pills, headers and footer words; `chromeFace` (JetBrains Mono) for agent badges, skill names, keycaps, header counts, tile initials and the `/`.
- **Copy (verbatim from spec and mockup):** headers "Waiting on you", "Sessions", "Recent skills", "Kaze", "Skills", "Recent", "Yours", "This project", "Other" (drawn upper case); rows "Browse all N skills", "N more matching skills", "N more in <plugin>"; placeholders "Search sessions and Kaze actions" / "Search skills by name or what they do"; switch "Sessions" | "Skills" + `/` keycap; chip "Skills" (controller ruling after the final review: no "/" of its own; the "/" stays in the field text and the switch keycap); "go to session ↵"; pills "needs you", "running", "idle" (plus "done", "shell", Ruling 7); empty `No results for "<text>"` then "Press / to search skills instead." (Sessions) or "Try part of what the skill does, like /review." (Skills), and "No skills found" alone in Skills when there are no skills at all, none installed or the scan not back (final review); footer "↑↓ move · ↵ run here · Shift ↵ new session · Esc close" on a skill row, else "↑↓ move · ↵ open · / skills · Esc close".
- **Sizes (spec 8.9, mockup):** box 600 dp (`min(600dp, 92% of main)` as today), corners 14 dp, field row 54 dp, rows 38 dp with 8 dp corners inset 6 dp, list at most 420 dp, short-window logic unchanged.
- Commits: `type(scope): Verb ...`, **no AI attribution lines of any kind**. Work on `feat/palette` in `C:\Users\john\Desktop\kaze-wt-palette`; never touch `C:\Users\john\Desktop\kaze`; never push to `main`. The 0.31 release (version, changelog, lander "What's new") is its own PR after the merge, not in this plan.

## Review Focus

1. **A scan that finishes after the palette opened** (first open, empty cache): the groups, the Browse count and Recent must follow the new list on the next frame, not stay empty until reopen. (Task 4 `TestPaletteGroupsFollowALateScan`.)
2. **Fast typing across the scope boundary:** Tab, `/` and text queued in one frame keep their order (Tab then "x" gives "/x"; "x" then Tab gives "/x"), and Enter in the same frame acts on the list for the final text. (Task 4 `TestPaletteTabThenTypeSameFrame`, `TestPaletteTypeThenTabThenEnterSameFrame`.)
3. **Selection stays unique and sensible** when the same skill appears twice (Recent and its group on an empty `/`), when a group opens (the selection lands on its first newly shown skill), and after a scope switch (the first row). Hover on the second copy must not select the first. (Task 3 `TestRecentKeysAreUnique`; Task 4 `TestMoreInPluginOpensAndSelects`, `TestScopeSwitchSelectsFirstRow`.)
4. **Odd skill names:** a user skill with a `:` in its name stays in "Yours"; a plugin skill without `:` goes to "Other" (last); `a:b:c` is plugin `a`; an empty plugin part (`:x`) is "Other"; plugin group order is case-insensitive. (Task 2 `TestGroupSkillsOddNames`.)
5. **Narrow windows:** under 440 dp of box width the switch is hidden, Tab still switches, the editor never overlaps the chip or the switch, and nothing draws outside the box. (Task 5 `TestPaletteNarrowHidesSwitch`.)

---

### Task 1: Measure today's palette frame

**Files:**
- Create: `internal/ui/palette_alloc_test.go`

**Interfaces:**
- Produces: `func palAllocScene(t *testing.T) (*window, *input.Router)` (palWindow plus 40 fake skills: 15 user, 9 `devbox:*`, 6 `superpowers:*`, 5 `v3-gauntlet:*`, 5 project; 5 sessions, one needing you), `func palStillAllocs(t *testing.T, win *window, r *input.Router, w, h int) float64` (3 warm frames then `testing.AllocsPerRun(20, frame)` at 1000×700), and the constants `palAllocsEmpty`, `palAllocsTyped` (today's numbers, Windows).

- [ ] **Step 1: Write the test** `TestPaletteFrameAllocs`: open the palette in the scene, measure an empty field and the field with "re"; `t.Logf` both. On `runtime.GOOS == "windows"` assert each `<= palAllocsEmpty` / `<= palAllocsTyped`; elsewhere only log (Ruling 12). Start both constants at 0, so it fails.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run TestPaletteFrameAllocs -v` → FAIL, the log shows today's numbers.
- [ ] **Step 3: Set** the two constants to the logged numbers (on unchanged code), with a comment naming the commit they were measured on (`94f942c`).
- [ ] **Step 4: Run** it again → PASS.
- [ ] **Step 5: Commit** `test(ui): Measure today's palette frame allocations`

### Task 2: Group the skills (pure)

**Files:**
- Modify: `internal/ui/palette_model.go`
- Test: `internal/ui/palette_model_test.go`

**Interfaces:**
- Produces:
  ```go
  type palGroup struct {
      Head   string         // drawn header: "YOURS", "THIS PROJECT", "DEVBOX", "OTHER"
      Plugin string         // plugin groups only: "devbox"; "" for Yours, This project, Other
      Skills []skills.Skill // sorted by name, case-insensitively
  }
  func groupSkills(list []skills.Skill) []palGroup // Yours, This project (if any), plugins A to Z, Other (if any); empty groups left out
  func pluginOf(s skills.Skill) string             // text before the first ":" for Scope == skills.Plugin, else ""
  func recentSkills(list []skills.Skill, recent []string) []skills.Skill // recent prompts still in list, most recent first, at most 3
  const palRecentMax = 3
  const palFoldAt = 4 // skills shown per folded plugin group
  ```
  Group by `Scope` (spec 8.4): `User` is Yours even with a `:`; `Project` is This project; `Plugin` uses `pluginOf`, and "" goes to Other. `Head` is `strings.ToUpper` of the name, computed here once.

- [ ] **Step 1: Write the failing tests:**
  - `TestGroupSkillsOrder`: user `zeta`, `alpha`; project `local`; plugins `superpowers:b`, `devbox:run`, `Devbox:kit` (sic, upper case), `devbox:dash` → heads `["YOURS","THIS PROJECT","DEVBOX","SUPERPOWERS"]`; Yours is `alpha, zeta`; `Devbox` and `devbox` are two groups only if the plugin text differs, so assert `Devbox:kit` forms its own group sorted next to `devbox` case-insensitively (Ruling 4: grouping is case-sensitive, order case-insensitive).
  - `TestGroupSkillsOddNames` (Review Focus 4): user `team:review` → Yours; plugin `orphan` (no `:`) → Other, last; plugin `a:b:c` → group `a`; plugin `:x` → Other.
  - `TestGroupSkillsNoProject`: no project skills → no "THIS PROJECT" group; nil list → nil.
  - `TestRecentSkillsCappedAndPresent`: recent `["/gone","/c","/a","/b","/d"]`, list has `a,b,c,d` → `c,a,b` (spec 8.3).
  - `TestGroupSkillsAllocsBounded`: 1000 skills → `testing.AllocsPerRun(5, ...)` at most `len(groups)+8` (one slice per group plus the sort; bounded, Global Constraints).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'GroupSkills|RecentSkills' -v` → FAIL (undefined).
- [ ] **Step 3: Implement** the three functions in `palette_model.go`.
- [ ] **Step 4: Run** the same → PASS.
- [ ] **Step 5: Commit** `feat(ui): Group palette skills by where they come from`

### Task 3: Build both scopes (pure)

**Files:**
- Modify: `internal/ui/palette_model.go` (new `palItem`, `palInput`, `buildPalette`; `palSection`, `Member`, `Color`, `in.Members` removed), `internal/ui/palette.go` and `internal/ui/palette_window.go` (only what keeps the package compiling: headers from `Group`, `palAct` on `Kind`), `internal/ui/dock_entrypoints_test.go` (delete `TestPaletteDocksAMember`: members leave the palette by the owner's decision)
- Test: `internal/ui/palette_model_test.go` (rewrite `samplePal` and the section tests to the new items)

**Interfaces:**
- Consumes: Task 2's `palGroup`, `groupSkills`, `recentSkills`, `palRecentMax`, `palFoldAt`.
- Produces:
  ```go
  type palKind int
  const ( palSession palKind = iota; palSkill; palAction; palBrowse; palMoreMatches; palMoreGroup )

  type palItem struct {
      Kind        palKind
      Key         string // "tab:<id>", "skill:<name>", "recent:<name>", "act:<config>", "ws:<id>", "browse", "more:matches", "more:<plugin>"
      Group       string // header text above it (upper case); a header is drawn where it changes
      GroupCount  string // count right of the header, "" for none
      Plugin      string // plugin group: tile initial and colour; palMoreGroup: which group to open
      Name        string // title, action name, "/plugin:skill", or the row text for browse and more
      Dim         int    // bytes of Name drawn dim: len("/devbox:") for a plugin skill, else 0
      Agent       string // badge: "claude", "codex"; "" none
      State       string // pill: "needs you", "running", "idle", "done", "shell"
      Detail      string // skill description
      Keys        []string // action shortcut parts: ["Ctrl","Shift","T"]
      Icon        string   // action tile icon (iconNames), "" none
      Mark        mark
      Running     bool
      Tab         tabs.TabID
      Prompt      string // "/" + skill name
      Act         action
      Workspace   string
      Count       int    // browse and more rows: the N
      Query       string // palMoreMatches: the text to switch to
  }
  type palAction struct { Name, Config, Workspace, Icon string; Keys []string; Act action } // Keys from formatShortcut split at "+"
  type palInput struct {
      Query    string
      Sessions []sideRow
      Skills   []skills.Skill // the catalog, for counts and Sessions-scope matches
      Groups   []palGroup     // groupSkills(Skills), cached by the window
      Recent   []string
      Actions  []palAction
      Open     map[string]bool // plugin groups opened by "N more in <plugin>"
  }
  func palSkillScope(q string) bool // strings.HasPrefix(strings.TrimLeft(q, " "), "/")
  func buildPalette(in palInput) []palItem
  ```
  Group head constants: `palHeadWaiting = "WAITING ON YOU"`, `palHeadSessions = "SESSIONS"`, `palHeadRecentSkills = "RECENT SKILLS"`, `palHeadKaze = "KAZE"`, `palHeadSkills = "SKILLS"`, `palHeadRecent = "RECENT"`.
  Rules, from spec 3.2, 3.3 and 8: Sessions scope empty: waiting, sessions (sidebar order), recent skills (`recent:<name>` keys, Group "RECENT SKILLS"), Browse (`Count = len(Skills)`, Group "RECENT SKILLS", or "SKILLS" when there is no recent, Ruling 5; hidden when no skills); no actions. Sessions scope with text: matching waiting and sessions (today's ranking), matching actions (`palTier(q, a.Name, strings.Join(a.Keys,"+"))`, spec 8.5), at most 3 matching skills (ranked by tier over `orderSkills` order) under "SKILLS" with `GroupCount` = all matches, then `palMoreMatches` when more than 3 (`Count = matches-3`, `Query = "/" + trimmed text`). Skills scope empty: "RECENT" (`recent:` keys), then each group; Yours and This project in full, plugin groups and Other folded at `palFoldAt` unless `Open[plugin]` (Ruling 6: Other folds like a plugin, `Plugin = "Other"` for its key), then `palMoreGroup` with `Count` = the skills not shown, `Plugin` = the group and `Name = "N more in <plugin>"`; `GroupCount` = the group's size. Skills scope with text: no Recent, every match in its group, nothing folded, ranked by `palTier(q, s.Name, s.Desc)` inside the group, empty groups left out, `GroupCount` = matches. A session's `State` is "needs you" for `markNeeds`, else `sessionWord(r)`; `Agent` is `r.Agent`, or "claude" when `r.Claude` and no agent.

- [ ] **Step 1: Write the failing tests** (fixtures: 1 needing session, 3 others, user skills `review-mine`, `ship-ticket`, `ci-watch`, plugin `devbox:run..devbox:z6` (6 skills), plugin `superpowers:writing-plans`, recent `["/ci-watch","/review-mine"]`, 4 actions):
  - `TestEmptySessionsScope`: groups in order `WAITING ON YOU, SESSIONS, RECENT SKILLS`; recent rows `/ci-watch, /review-mine`; last row `Kind == palBrowse`, `Name == "Browse all 10 skills"`, `Count == 10`; no `palAction`, no member (spec 6).
  - `TestNoRecentNoBrowseWithoutSkills`: no recent → Browse under "SKILLS"; no skills → no Browse.
  - `TestSessionsScopeText`: query "r" → actions under "KAZE" before "SKILLS"; exactly 3 skill rows; then `palMoreMatches` with `Count == total-3` and `Query == "/r"`; "SKILLS" `GroupCount` is the total as a string.
  - `TestActionMatchesShortcut`: query "shift+t" finds the action whose Keys are `["Ctrl","Shift","T"]`.
  - `TestEmptySkillsScope`: query "/" → heads `RECENT, YOURS, DEVBOX, SUPERPOWERS`; Yours lists all 3; DEVBOX lists 4 then `palMoreGroup` "2 more in devbox" with `Key == "more:devbox"`; DEVBOX `GroupCount == "6"`.
  - `TestOpenGroupShowsAll`: `Open{"devbox": true}` → 6 devbox rows, no "more" row.
  - `TestSkillsScopeTextUnfolds`: "/d" → no RECENT; every devbox match (6, unfolded); a group with no match left out; inside a group a name-prefix match ranks before a description match.
  - `TestRecentKeysAreUnique` (Review Focus 3): on "/" every `Key` is distinct; `/ci-watch` appears as `recent:ci-watch` and `skill:ci-watch`.
  - `TestPluginSkillDim`: `/devbox:run` has `Dim == len("/devbox:")`; `/ci-watch` has 0.
  - `TestSkillScopeIgnoresLeadingSpace`: " /x" is the Skills scope (Ruling 3).
  - Keep and adapt: `TestNeedsSessionOnlyUnderWaiting`, `TestPaletteRanksNamePrefixFirst`, `TestPluginNameRanksByItsOwnPart`, `TestSessionsKeepSidebarOrder`, `TestRunWorkspaceItems`, `TestRecentKeepsTenDistinct`, `TestSessionRowsGroupsThenLoose`. Delete `TestSessionDetails` (replaced by a `TestSessionState` asserting `State` and `Agent`).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Scope|Palette|Recent|Session|Plugin|Action' -v` → FAIL (compile errors on the new fields count as RED).
- [ ] **Step 3: Implement** the model; adapt `layoutPalette` to draw a header where `Group` changes (`firstInSection` becomes `firstInGroup`) and `palAct` to switch on `Kind` (`palBrowse`, `palMoreMatches`, `palMoreGroup` do nothing yet, Task 4); `win.palInput()` stops reading members and fills `Groups` with `groupSkills(p.skills)` for now (cached in Task 4).
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (whole package: the window tests still pass on the new items, with `palette_window_test.go` assertions on member rows removed).
- [ ] **Step 5: Commit** `feat(ui): List sessions first and put skills behind a slash`

### Task 4: Keys, scope switching and the browse and more rows

**Files:**
- Modify: `internal/ui/palette_window.go`
- Test: `internal/ui/palette_window_test.go`, `internal/ui/palette_input_test.go`

**Interfaces:**
- Consumes: Task 3's `palItem`, `palKind`, `palSkillScope`, `palAction`, `palInput.Open`, `groupSkills`.
- Produces (used by Task 5's clicks):
  ```go
  func (win *window) setPaletteQuery(text string)  // SetText, caret at the end, sel "", scroll 0, open groups cleared, Invalidate
  func (win *window) togglePaletteScope()          // "/"+text or text without its leading "/", keeping the rest
  // paletteUI gains:
  open    map[string]bool // plugin groups opened; cleared on any text change, scope change, open and close
  groups  []palGroup      // groupSkills(skills), set where p.skills is set
  actions []palAction     // built once in openPalette (Keys split, Icon from Ruling 9)
  ```
  `isPaletteKey` also takes `key.NameTab` (any modifiers) and `key.NameDeleteBackward` only when the text starts with "/" and the caret is at 0 with no selection. `paletteKey`: Tab without Shift → `togglePaletteScope`; Shift+Tab → nothing (spec 8.7); that Backspace → `togglePaletteScope`. `palAct`: `palBrowse` → `setPaletteQuery("/")`; `palMoreMatches` → `setPaletteQuery(it.Query)`; `palMoreGroup` → `open[it.Plugin] = true`, then `sel = "skill:" + ` the group's skill at index `palFoldAt`, `reveal = true`; none of the three closes the palette. The ChangeEvent branch of `updatePaletteEditor` also clears `open`.

- [ ] **Step 1: Write the failing tests** (palWindow with skills `review-mine`, `ship-ticket`, `devbox:a..devbox:f`):
  - `TestTabTogglesScopeKeepingText`: type "rev", Tab → text "/rev", Tab → "rev"; Shift+Tab → unchanged.
  - `TestTypingSlashEntersSkills`: type "/" → first row's Group is "YOURS" or "RECENT".
  - `TestBackspaceAtStartLeavesSkills`: text "/x" with caret 1, Backspace → "x" (native); text "/x" with caret 0 (`ed.SetCaret(0,0)`), Backspace → "x" (spec 6).
  - `TestBrowseRowSwitchesScope`: empty field, select Browse (arrows), Enter → text "/", palette still open.
  - `TestMoreMatchingRowSwitchesWithText`: type "e", select `more:matches`, Enter → text "/e".
  - `TestMoreInPluginOpensAndSelects` (Review Focus 3, spec 8.1): "/", select `more:devbox`, Enter → 6 devbox rows, `selected(win).Key == "skill:devbox:e"`; typing "x" then deleting it → folded again.
  - `TestScopeSwitchSelectsFirstRow`: arrow down twice, Tab → `selected` is the first row.
  - `TestEnterAndShiftEnterOnSkillUnchanged`: keep `TestEnterOnASkillTypesIt` and `TestShiftEnterOpensANewSession` passing on a skill reached in the Skills scope too.
  - `TestPaletteGroupsFollowALateScan` (Review Focus 1): `skillsFn` blocks on a channel; open, run 2 frames (no skills, no Browse), release the channel, `waitSkills` → Browse row says "Browse all 8 skills" and "/" lists DEVBOX, without reopening; `groupSkills` ran once (a counter seam `p.groupBuilds`) over 10 further frames.
  - `TestActionsBuiltOncePerOpen`: 10 frames open → `p.actions` slice header unchanged (same backing pointer).
  - `palette_input_test.go`: `TestPaletteTabThenTypeSameFrame` (Tab and "x" queued in one frame → "/x"), `TestPaletteTypeThenTabThenEnterSameFrame` ("rev", Tab, Enter queued in one frame → `review-mine` typed into the session, as `TestPaletteQueryAndEnterSameFrame` checks today) (Review Focus 2).
  - Keep `TestEscClosesAndReturnsKeys`, `TestPaletteIdleCostsNothing`, `TestSkillsScannedOnlyOnOpen` green.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Tab|Scope|Backspace|Browse|More|LateScan|BuiltOnce|SameFrame' -v` → FAIL.
- [ ] **Step 3: Implement** in `palette_window.go`.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Switch palette scope with Tab, slash and the browse rows`

### Task 5: The box, field row, switch, chip, footer and inks

**Files:**
- Modify: `internal/ui/palette.go` (constants, `palState`, `palHits`, `layoutPalette` frame parts), `internal/ui/palette_window.go` (`palettePointer` clicks on the switch and chip; `palState` filled), `internal/ui/tabbar.go` (`setTheme` resets the ink cache)
- Create: `internal/ui/palette_ink.go`
- Test: `internal/ui/palette_test.go`

**Interfaces:**
- Consumes: Task 4's `setPaletteQuery`, `togglePaletteScope`; Task 3's `palSkillScope`.
- Produces (Task 6 uses `palInk`):
  ```go
  const ( palW = 600; palRadius = 14; palFieldH = 54; palRowH = 38; palRowRadius = 8; palInset = 6
          palHeadH = 30; palFootH = 36; palListMaxH = 420; palEmptyH = 96; palSwitchMinW = 440 )
  type palInk struct {
      bg, line, outer, raised, sel, selBar, foot, switchBG    color.NRGBA // surfaces
      text, muted, faint, go, chipFG, chipBG, chipLine        color.NRGBA
      keyFG, keyBG, keyLine, badge                             color.NRGBA
      pillNeedsFG, pillNeedsBG, pillRunFG, pillRunBG, pillIdleFG, pillIdleBG, pillDoneFG, pillDoneBG color.NRGBA
      tileBG, tileFG                                           color.NRGBA // browse and more tiles
      plugins [6]color.NRGBA // tile colours: ansi 1..6 made readable on bg at 3:1 or more
      pluginInk [6]color.NRGBA // the initial on each
  }
  func (c *chrome) palInk() *palInk           // built once per theme (cached on chrome, reset by setTheme)
  func palPluginColor(plugin string) int      // FNV-1a 32 of the name, mod 6: an index into plugins
  // palState gains: skills bool; query string (for the empty line); footSkill bool
  // palHits gains: switchSessions, switchSkills, chip image.Rectangle (zero when not drawn)
  ```
  Values: `bg = ToastBG`; `raised = mix(ToastBG, ToastFG, 0.08)` (Ruling 2); `sel = mix(bg, Accent, 0x21/255.)` (13%); `selBar = Accent`; `outer = {A: 0x59}` (1 dp outside the box); `line = ToastLine`; `switchBG = Chrome`; `foot = mix(bg, Chrome, 0.6)`; `chipBG = mix(bg, Accent, 0x29/255.)`, `chipLine = mix(bg, Accent, 0x59/255.)`, `chipFG = Readable(ToastGo, chipBG)`; `text = Readable(ToastFG, sel)` (readable on the selected fill, which is the harder surface; check on `bg` too); `muted = Readable(Muted, sel)`; `go = Readable(ToastGo, sel)`; keycap `keyBG = raised`, `keyFG = Readable(CapGlyph, raised)`; pills: `pillNeedsBG = mix(bg, Needs, 0.16)`, `pillNeedsFG = Readable(Needs, pillNeedsBG)`, run uses Accent / ToastGo, done uses Done, idle uses Muted at 0.12. Top hairline: two `paint.LinearGradientOp` halves inset 14 dp, transparent → `tint(ToastGo, 0x73)` → transparent, 1 dp.
  Layout (mockup `.top`, `.scope`, `.foot`): magnifier = `c.icon("search")` 16 dp in `muted`, 14 dp from the left; gap 10 dp; in Skills the chip (mono "/" + sans semibold 12 sp "Skills", padding 2/8/2/6 dp, radius 6 dp); the editor at 16 sp `sansFace` in `text`, placeholder in `faint` per scope; the switch at the right when `box.Dx() >= gtx.Dp(palSwitchMinW)`: 2 dp padded `switchBG` with a 1 dp `Border` line, radius 8 dp, two parts (sans 12 sp, padding 4/10 dp, radius 6 dp; on = `raised` fill + 1 dp `line`, `text`; off = `muted`), the `/` keycap mono 11 sp, `go` when Skills is on. Footer: 1 dp `Border` line, `foot` fill, keycaps (mono 11 sp, 1 dp `keyLine`, 2 dp bottom, radius 5 dp, padding 0/5 dp) and sans 12 sp `muted` words, gap 16 dp, "Esc close" right-aligned; the skill set when `footSkill`. Empty list: `palEmptyH` tall, centered `No results for "<text without its slash>"` sans 14 sp `text`, then the scope's line in `muted` 13 sp.
  Clicks (spec 8.8): switch "Sessions" → leave Skills if in it; "Skills" → enter if not; the chip → leave Skills (Ruling 10); all keep the keyboard in the field (`wantFocus`).

- [ ] **Step 1: Write the failing tests:**
  - `TestPaletteBoxIsCentered` (adapt): box 600 dp wide at scale 1 and 2, centered; field 54 dp.
  - `TestPaletteNarrowMainView` (keep): box inside a 300 px main view.
  - `TestPaletteNarrowHidesSwitch` (Review Focus 5): box under 440 dp → `hits.switchSkills` empty; at 600 dp → both parts non-empty, right of the editor and inside `hits.field`; in Skills the chip lies left of the editor and does not overlap the switch.
  - `TestSwitchAndChipClicks` (window test): `clickAt` on `hits.switchSkills` → text "/"; on `hits.chip` → text ""; on `hits.switchSessions` with "/rev" → "rev"; the palette stays open and `wantFocus` is set.
  - `TestPaletteEmptyLines`: no items, query "zzz" → headers nil and the drawn strings (a `palHits.lines []string` test seam, filled only by the empty state) are `No results for "zzz"` and "Press / to search skills instead."; "/zzz" gives the Skills line.
  - `TestPaletteInkReadableInEveryTheme`: for each `theme.All()`: `text`, `muted`, `go` at least 4.5 on both `bg` and `sel`; `chipFG` on `chipBG`, `keyFG` on `keyBG`, each pill FG on its BG at least 4.5; `selBar` on `sel` and every `plugins[i]` on `bg` at least 3; `pluginInk[i]` on `plugins[i]` at least 4.5 (spec 8.10). Uses the existing `contrast` helper.
  - `TestPalPluginColorStable`: `palPluginColor("devbox")` is the same across calls and in `[0,6)`; "devbox" and "superpowers" differ (pin the two values).
  - `TestPalInkCachedPerTheme`: two `palInk()` calls return the same pointer; after `setTheme` a different one.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'PaletteBox|PaletteNarrow|SwitchAndChip|PaletteEmpty|PaletteInk|PalPlugin|PalInk' -v` → FAIL.
- [ ] **Step 3: Implement** `palette_ink.go` and the frame parts of `layoutPalette`; rows keep Task 3's plain drawing until Task 6.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw the palette's new box, field and scope switch`

### Task 6: Rows and headers

**Files:**
- Modify: `internal/ui/palette.go` (`palRow`, header drawing)
- Test: `internal/ui/palette_test.go`

**Interfaces:**
- Consumes: Task 5's `palInk`, `palPluginColor`, sizes; Task 3's `palItem`.
- Produces: `func (c *chrome) palRow(gtx layout.Context, it palItem, r image.Rectangle, selected bool, ink *palInk)` and `func (c *chrome) palHeader(gtx layout.Context, it palItem, r image.Rectangle, ink *palInk)`; `palHits` gains `badges []image.Rectangle` (test seam: the agent badge and pill rects of visible rows, so tests can check they stay inside the row).
  Look (mockup `.hd`, `.row`): header 30 dp, sans semibold 11 sp `faint` text 10 dp from the row inset, bottom-aligned with 5 dp under it; a plugin group (`it.Plugin != ""` on a skill or more row whose Group is the plugin's) gets a 16 dp radius-4 tile in `plugins[palPluginColor]` with the upper-case initial, mono bold 9 sp in `pluginInk`; the count right-aligned in mono 11 sp `faint`. Row: 38 dp, radius 8 dp, padding 10 dp, gap 10 dp; selected: `sel` fill and a 2 dp `selBar` from 9 dp under the top to 9 dp above the bottom, radius 1 dp. Session: `c.sideDot` mark (8 dp; running ring 10 dp; needs in `Needs`), title sans 13.5 sp `text`, the badge (mono 11 sp `muted`, 1 dp `line`, radius 5 dp, padding 0/6 dp, 18 dp tall), then at the right the pill (sans semibold 11 sp, radius 9 dp, padding 1/8 dp) and on the selected row only "go to session ↵" sans 12 sp `faint` before it. Skill: mono 13.5 sp, the first `Dim` bytes in `faint`, the rest in `go`; description sans 12.5 sp `muted` right-aligned in at most 56% of the row, 16 dp from the name. Action: a 22 dp radius-6 tile in `raised` with a 1 dp `line` and the `Icon` drawn 14 dp in `muted` (the slot is kept, the tile drawn only with an icon, Ruling 9), the name sans 13.5 sp `text`, keycaps at the right (Task 5's keycap). Browse and more: a 22 dp radius-6 tile in `tileBG` (`mix(bg, Accent, 0.12)`) with mono semibold 12 sp "/" (browse) or "+" (more) in `go`; Browse text "Browse all " in `muted` then "N skills" semibold in `go`; more rows all `muted`. Every text cut with `fitSide`/`fitSideWeight` to its room; nothing outside `r`.

- [ ] **Step 1: Write the failing tests:**
  - `TestPaletteRowsAndHeaders` (adapt): items over three groups → `hits.headers` are the three heads in order; each row rect 38 dp tall, inset 6 dp from the box.
  - `TestPaletteListScrolls` (adapt to 38 dp rows, 420 dp list), `TestPaletteLongTextStaysInside` (adapt: a 300-rune title, a 200-rune description and a 12-part shortcut stay inside the row; every `hits.badges` rect inside its row).
  - `TestSelectedRowShowsGo`: the same session row drawn selected and not: a selected-only seam (`palHits.goShown int`) is 1 and 0.
  - `TestPaletteDrawsInEveryTheme` (adapt): one item of every `Kind` and every pill `State`, in all five themes, no panic, rows inside the box.
  - `TestPluginHeaderHasTile`: a DEVBOX group draws a tile (a `palHits.tiles` count seam) and YOURS draws none.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'PaletteRows|PaletteList|PaletteLong|SelectedRow|PaletteDraws|PluginHeader' -v` → FAIL.
- [ ] **Step 3: Implement** `palRow` and `palHeader`.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw palette rows with pills, badges, keycaps and plugin tiles`

### Task 7: Captures, contrast sweep and the frame cost

**Files:**
- Create: `internal/ui/palette_render_test.go`
- Modify: `internal/ui/palette_alloc_test.go` (Task 1's test now guards the new code)

**Interfaces:**
- Consumes: Task 1's `palAllocScene`, `palStillAllocs`, constants; everything above.

- [ ] **Step 1: Captures** `TestPaletteRenders`, gated by `KAZE_PALETTE_RENDER_DIR` (pattern of `TestFindRenders` / `TestDockRenders`: `applyTheme`, `pageContext`, `headless.NewWindow`, PNG), 1000×700, the Task 1 scene with the mockup's session titles and skill names, all five themes × states `open` (empty), `typing` ("v3"), `slash` ("/"), `review` ("/review"), `none` ("zzz"), plus `narrow` (520 px, Midnight and Washi): `palette-<theme>-<state>.png`. Skipped without the env var. Run it with the env var pointing at the scratch folder and look at every image against the mockup; fix what differs, with a test where the difference is behaviour.
- [ ] **Step 2: Contrast sweep** `TestPaletteTextReadableAsDrawn`: for each theme, draw the `review` state and, through the `palInk` the drawing used, assert the pairs it actually drew (a `palHits.pairs []palPair{fg, bg; min float64}` seam recorded only when a test sets `palState.audit`) all meet their minimum. This catches a colour drawn on a surface Task 5 did not test.
- [ ] **Step 3: Frame cost:** run `TestPaletteFrameAllocs` → PASS on Windows: the new palette's still frame allocates no more than Task 1's numbers for the empty field and for "re". Add a third measure for "/" (Skills scope) with the bound `palAllocsEmpty` (Ruling 12). Log all three; they go into the HANDOFF entry. If it fails, cut allocations (cache strings built per frame, such as header counts and the empty line, on `paletteUI` when the text changes) rather than raise a constant.
- [ ] **Step 4: Run** `go test ./...` and `go vet ./...` → PASS; `GOOS=darwin go vet ./internal/skills/` → PASS.
- [ ] **Step 5: Commit** `test(ui): Capture the palette in every theme and hold its frame cost`

### Task 8: README, SCREENSHOTS note, HANDOFF

**Files:**
- Modify: `README.md` ("## The palette"), `docs/site-assets/SCREENSHOTS.md`, `docs/HANDOFF.md`

- [ ] **Step 1: README:** rewrite "The palette": opens on Waiting on you, Sessions, up to 3 recent skills and "Browse all N skills"; typing also finds Kaze actions and up to 3 skills; `/`, Tab or the switch opens Skills, grouped Yours, This project, then one group per plugin (4 shown, "N more in ..." for the rest); Enter / Shift+Enter on a skill as today; servers are not listed (use the status bar or the sidebar); keep the "Skills come from" paragraph, with "The last 3 skills you started show under Recent" replacing the "last 10 stay at the top" sentence. Plain language, no em dashes.
- [ ] **Step 2: SCREENSHOTS.md:** under `kaze-palette.png`, note that the palette changed in this branch and the shot needs retaking (sessions first; and a second shot of the `/` Skills view if the lander wants one); no faked shot.
- [ ] **Step 3: HANDOFF.md:** a "Where things stand" entry at the top: the redesign (branch `feat/palette`, this plan, the spec, the mockup), Task 7's three allocation numbers before and after, the rulings below the owner may want to revisit (numbers 5, 6, 7, 9, 10, 11), and the owner's checks by hand: open, `/`, Tab, a plugin group's "more" row, Shift+Enter, all five themes, a narrow window. Update "Last updated".
- [ ] **Step 4: Run** `go test ./...` → PASS (docs only, nothing should change).
- [ ] **Step 5: Commit** `docs: Describe the new palette and note its screenshot`

---

## Rulings (gaps the spec left, decided here)

1. **The `/` stays in the editor text** (the scope is derived from it, spec 3.1); the "Skills" chip is drawn before the editor as well, as in the mockup (its own "/" dropped after the final review).
2. **"CapHover-style" raised fill** is `mix(ToastBG, ToastFG, 0.08)`, because Midnight's `CapHover` equals its `ToastBG` (the find bar made the same call). The box itself stays `ToastBG`.
3. **Leading spaces** before `/` still mean Skills (`palQuery` already trims).
4. **Plugin groups** are case-sensitive by name (two spellings are two plugins) and ordered case-insensitively.
5. **Browse row's header** is "Recent skills" when recent rows show, else "Skills".
6. **"Other"** folds at 4 like a plugin group and opens with "N more in Other".
7. **Pills** also cover "done" (`Done` tint) and "shell" (idle style); the row drops today's " · /command" text (the mockup has none), and matching on the command stays.
8. **Header counts** show on skill groups only (Skills-scope groups other than Recent, and the Sessions-scope "Skills" header, whose count is all matches, not the 3 shown). No letter spacing on headers (Gio labels have no tracking).
9. **Action icons:** `new_tab` plus, `new_branch_session` branch, `open_settings` cog, `toggle_sidebar` panel, `find` search, `agents_board` agents, "Run workspace" workspaces; others get no tile, their text keeps the slot's alignment.
10. **Clicking the chip** leaves Skills (a removable filter); the switch parts set the scope they name.
11. **Switch hidden** when the box is narrower than 440 dp; Tab keeps working.
12. **Frame-cost bound** is asserted on Windows only (where Task 1 measured today); other platforms log, because Gio's allocation counts can differ by OS.
13. **Editor font** is Manrope 16 sp (mockup); `palTop` stays 60 dp.
