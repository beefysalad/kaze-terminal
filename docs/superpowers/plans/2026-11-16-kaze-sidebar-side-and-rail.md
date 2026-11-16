# Sidebar side and rail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the sidebar sit on the left or the right, and cycle it through full, rail and hidden with `toggle_sidebar`.

**Architecture:** Two new config keys feed one effective mode (`sideMode`) and a side flag on the window. `window.go` places the sidebar rect on the chosen side; the full sidebar mirrors its divider and accent line; the title bar's lead strip follows the side. A new `layoutRail` draws 52 dp of tiles from the same `sideModel` and returns the same `sideHits` shape (tiles as `rows`, bars as `members`), so the existing pointer code (`sidePointer`, `sidePress`, `lineAt`, `clickRow`, `closeTab`) handles the rail unchanged.

**Tech Stack:** Go 1.27, Gio. Tests with `go test`, headless GPU renders with `gioui.org/gpu/headless`.

**Spec:** `docs/superpowers/specs/2026-11-16-kaze-sidebar-side-and-rail-design.md`

## Global Constraints

- Default side `"left"`, default width `"full"`; nothing about the left layout changes for someone who never touches the settings.
- Full 260 dp (`sidebarW`), rail 52 dp, hidden 0. Narrow rule: window under 560 dp (`minSideWindow`) shows full as rail, unsaved; under 300 dp the rail hides too.
- `sidebar = true|false` in old configs still loads with no warning; Kaze writes only `sidebar_mode` and `sidebar_side`.
- One font (JetBrains Mono via `chromeFace`), Regular and Bold. No emoji, no Unicode-symbol icons: rail tiles use `agentLogo` and `stateDot`.
- Zero allocations on an unchanged sidebar frame, full or rail (`testing.AllocsPerRun == 0`). No ticker.
- Strict TDD, RED first; RED tests bounded (fixed sizes, no unbounded loops).
- Commits: `type(scope): Verb ...`, no AI attribution. Never push to `main`.
- Never inject global input; tests drive `win.route`, `sidePress` and frames.

## Review Focus

1. Toggling while a narrow window forces the rail: the saved mode must still cycle and save, and the effective view must follow (full saved, narrow, press: saved becomes rail, view stays rail). Test in Task 2.
2. Switching side or width while the filter or a rename editor has the keyboard: the editor must drop focus cleanly, no stuck keyboard. Test in Task 2.
3. Rail with Settings open and shown: gear tile present and highlighted, clicking a session tile leaves Settings. Test in Task 5.
4. Rail with more sessions than fit: wheel scrolls, Workspaces bars stay pinned, scroll clamps after a session closes. Test in Task 5.
5. Right side on Windows at the 560 dp edge in rail mode: the lead strip (toggle, gear, agents, caption buttons) never overlaps the crumb or search. Test in Task 4.

---

### Task 1: Config keys

**Files:**
- Modify: `internal/config/config.go` (struct near line 50, `Default` near 89, load switch near 245)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.SidebarSide string` (toml `sidebar_side`, `"left"|"right"`, default `"left"`), `Config.SidebarMode string` (toml `sidebar_mode`, `"full"|"rail"|"hidden"`, default `""` meaning "follow the old `Sidebar` bool"). `Config.Sidebar bool` stays.

- [ ] **Step 1: Write failing tests**

```go
func TestSidebarSideAndModeLoad(t *testing.T) {
	cfg, warns, err := Load(write(t, "sidebar_side = \"right\"\nsidebar_mode = \"rail\"\n"))
	if err != nil || len(warns) != 0 { t.Fatalf("err %v warns %v", err, warns) }
	if cfg.SidebarSide != "right" || cfg.SidebarMode != "rail" { t.Fatalf("got %q %q", cfg.SidebarSide, cfg.SidebarMode) }
}
func TestSidebarDefaultsKeepOldBool(t *testing.T) {
	cfg, warns, _ := Load(write(t, "sidebar = false\n"))
	if len(warns) != 0 || cfg.SidebarSide != "left" || cfg.SidebarMode != "" || cfg.Sidebar { t.Fatalf("%+v %v", cfg, warns) }
}
func TestSidebarBadValuesWarn(t *testing.T) {
	cfg, warns, _ := Load(write(t, "sidebar_side = \"top\"\nsidebar_mode = \"wide\"\n"))
	if len(warns) != 2 || cfg.SidebarSide != "left" || cfg.SidebarMode != "" { t.Fatalf("%+v %v", cfg, warns) }
}
```

- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/config/ -run Sidebar` fails to compile (no fields).
- [ ] **Step 3: Implement** — add the two fields with toml tags and comments; `Default()` sets `SidebarSide: "left"`; load cases use `loadChoice` with `[]string{"left","right"}` and `[]string{"full","rail","hidden"}`. Add both keys to the known-keys handling so they don't warn as unknown.
- [ ] **Step 4: Run, expect PASS** — `go test ./internal/config/`
- [ ] **Step 5: Commit** — `feat(config): Add sidebar_side and sidebar_mode`

---

### Task 2: Effective mode, the cycle, and the narrow rule

**Files:**
- Modify: `internal/ui/sidebar_window.go` (`toggleSidebar` line 156, `sideWidth` line 169, constants near 25)
- Modify: `internal/ui/sidebar_window_test.go:280-320` (the Ctrl+Shift+B and narrow tests now expect the cycle)
- Create: `internal/ui/sidebar_mode.go`, `internal/ui/sidebar_mode_test.go`

**Interfaces:**
- Consumes: Task 1 fields.
- Produces:
  - `type sideMode uint8` with `sideFull`, `sideRail`, `sideHidden`; `func (m sideMode) String() string` returns `"full"|"rail"|"hidden"`.
  - `func (win *window) savedSideMode() sideMode` — `SidebarMode` if set, else `Sidebar` true → full, false → hidden.
  - `func (win *window) sideRight() bool` — `SidebarSide == "right"`.
  - `func effectiveSideMode(saved sideMode, widthDp int) sideMode` — pure: under 300 → hidden; under 560 and saved full → rail; else saved.
  - `win.side.mode sideMode` — this frame's effective mode, set by `sideWidth`.
  - `func (win *window) setSideMode(m sideMode)` and `func (win *window) setSideRight(right bool)` — set config, `dropSideEdit()`, clear hover, `saveSetting("sidebar_mode", m.String())` / `saveSetting("sidebar_side", "left"|"right")`.
  - `toggleSidebar()` becomes: `setSideMode(next(savedSideMode()))` with order full → rail → hidden → full. The narrow no-op is removed.
  - `sideWidth` returns `gtx.Dp(sidebarW)`, `gtx.Dp(railW)` or 0 for the effective mode; `const railW = 52`, `const minRailWindow = 300`. `win.side.hidden` stays true only for hidden.

- [ ] **Step 1: Write failing tests** in `sidebar_mode_test.go`:
  - `TestEffectiveSideMode`: table — `(full,1000)→full`, `(full,500)→rail`, `(rail,500)→rail`, `(hidden,500)→hidden`, `(full,250)→hidden`, `(rail,299)→hidden`.
  - `TestSavedSideModeFallsBackToBool`: `SidebarMode ""` with `Sidebar` true → full, false → hidden; `SidebarMode "rail"` wins over `Sidebar false`.
  - `TestToggleCyclesAndSaves`: from full, three `toggleSidebar()` calls give rail, hidden, full; after each, the config file contains `sidebar_mode = "rail"`, `"hidden"`, `"full"`.
  - `TestToggleWhileNarrowCyclesSavedMode` (Review Focus 1): 500 px frame with saved full shows rail (`win.side.mode == sideRail`, grid x = 52+8); toggle saves rail and the view stays rail; toggle again saves hidden and grid x is 8.
  - `TestSideChangeDropsEditors` (Review Focus 2): with `win.side.filtering = true`, `setSideRight(true)` then a frame leaves `filtering` false; same with a rename in progress (`startRename`) and `setSideMode(sideRail)` → `win.side.renaming == 0`.
  - Update the two existing tests at `sidebar_window_test.go:280-320`: Ctrl+Shift+B now goes full → rail (grid x = 52+8, file has `sidebar_mode = "rail"`); the narrow test now expects the rail at 500 px (not hidden) and still no config write.
- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/ui/ -run 'SideMode|Toggle|Narrow|SideChange'`
- [ ] **Step 3: Implement** `sidebar_mode.go` and the edits above.
- [ ] **Step 4: Run, expect PASS** — `go test ./internal/ui/` (whole package: many tests set `Config.Sidebar` and must still pass via the fallback).
- [ ] **Step 5: Commit** — `feat(sidebar): Cycle the sidebar through full, rail and hidden`

(Until Task 5 lands, the rail mode draws nothing in its 52 dp: `drawSide` gets an empty-model branch. That is fine inside the branch.)

---

### Task 3: Right side for the window and the full sidebar

**Files:**
- Modify: `internal/ui/window.go:674-676` (sideRect, termRect)
- Modify: `internal/ui/sidebar.go` (`sideState` gets `right bool`; border fill at line 197; accent line at line 374; `h.list` at line 254)
- Modify: `internal/ui/sidebar_window.go` (`drawSide` sets `s.st.right`; `sideDrawKey` gets `right`)
- Modify: `internal/ui/sidebar_cache.go` (`sideDrawKey` field)
- Test: `internal/ui/sidebar_side_test.go` (create)

**Interfaces:**
- Consumes: `win.sideRight()`, `sideWidth` from Task 2.
- Produces: `sideState.right bool`; layout rule `sideRect = image.Rect(size.X-w, barH, size.X, bottom)` and `termRect = image.Rect(0, barH, size.X-w, bottom)` when right.

- [ ] **Step 1: Write failing tests**
  - `TestRightSidebarRects`: window 1200x700, right, full: `win.side.at.X == 1200-260`, `win.gridMin.X == 8`, `win.gridRect.Max.X == 1200-260-8`. Same with rail: `at.X == 1200-52`.
  - `TestRightSidebarMirrorsDividerAndAccent`: lay out `twoGroupsOneLoose()` with `sideState{shown: 1, right: true}` at 260x400 and render it with a new test helper `renderImage(t, ops, size) *image.RGBA` (the body of `writeRender` without the file). Assert pixel (0, 200) is `Border` and (259, 200) is not; the shown row's accent colour is at `x = rect.Max.X-1` at the row's vertical middle and not at `rect.Min.X`. Same test with `right: false` keeps today's pixels (border at 259, accent at `rect.Min.X`).
  - `TestRightSidebarClicksStillHit`: right, full; `clickAt` the centre of the second row's hit rect (from `win.side.hits.rows`, offset by `win.side.at`) shows that tab.
- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/ui/ -run RightSidebar`
- [ ] **Step 3: Implement** — rects in `window.go`; in `layoutSidebar` draw the 1 px border at x 0 when `st.right`, the accent at `rect.Max.X-2dp..rect.Max.X`, and the list rect from x 1 instead of ending at `size.X-1`. Add `right` to the cache key.
- [ ] **Step 4: Run, expect PASS** — `go test ./internal/ui/`, including `sidebar_cache_test.go` zero-alloc tests.
- [ ] **Step 5: Commit** — `feat(sidebar): Put the sidebar on the right`

---

### Task 4: Title bar follows the side and width

**Files:**
- Modify: `internal/ui/titlebar.go` (`titleIn`, `layoutTitle` lines 155-209)
- Modify: `internal/ui/titlebar_draw.go:93-105, 134` (inputs, lead fill and divider, toggle icon)
- Modify: `internal/ui/chrome_cache.go:56` (cache key compares the new fields)
- Modify: `internal/ui/settings_style.go:133, 237` (new icon `"panelRight"`: the `"panel"` path with the inner line at the mirrored x)
- Test: `internal/ui/titlelayout_test.go`

**Interfaces:**
- Consumes: `win.sideRight()`, `win.side.mode`.
- Produces: `titleIn.Right bool`, `titleIn.NoPlus bool` (true unless full). `Lead` is the strip over the sidebar on either side.

Layout for `Right`: left side is lights (Mac), mark, word (fits rule uses the hidden-sidebar branch), nothing else. Lead strip = `[size.X - lw, size.X)` with `lw = max(SideW, width of its buttons + pads)`. Inside it, right to left: Windows caption buttons, toggle, gear, agents, then `Plus` at the strip's left end + `LeadPad` when `!NoPlus`. Crumb and search share `[after word, Lead.Min.X)`. Hidden and right: the same buttons at the right, `Lead` empty-width strip with no fill (draw skips fill and divider when `SideW == 0`). For left, `NoPlus` drops the Plus and the strip is `max(SideW, Toggle.Max.X + LeadPad)`.

- [ ] **Step 1: Write failing tests** (pure `layoutTitle` with the dims the existing tests use):
  - `TestTitleRightFullMac`: `Size 1200x38, Mac, LightsW 70, SideW 260, Right`: `Lead == Rect(940,0,1200,38)`; `Gear`, `Agents`, `Toggle`, `Plus` all inside `Lead`; order by X: `Plus < Agents < Gear < Toggle`; `Mark.Min.X == 70+Gap`; `Search.Max.X <= 940`.
  - `TestTitleRightRailWindows`: `Mac false, SideW 52, Right, NoPlus`: `Plus` empty; `Lead.Min.X <= size.X - 52`; `Buttons[2].Max.X == size.X`; `Toggle.Max.X <= Buttons[0].Min.X`.
  - `TestTitleLeftRailHasNoPlus`: left, `SideW 52, NoPlus`: `Plus` empty, `Lead.Max.X >= Toggle.Max.X`.
  - `TestTitleRightNarrowNoOverlap` (Review Focus 5): Windows, width 560, rail, right: no pair among `Crumb, Search, Agents, Gear, Toggle, Buttons...` overlaps.
  - `TestTitleCacheSeesSide`: two `titleCache.layout` calls differing only in `Right` give different `Lead`.
- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/ui/ -run Title`
- [ ] **Step 3: Implement** in `layoutTitle`; in `titlebar_draw.go` pass `Right: win.sideRight(), NoPlus: win.side.mode != sideFull`, draw the divider at `Lead.Min.X` when right, and use icon `"panelRight"` when right.
- [ ] **Step 4: Run, expect PASS** — `go test ./internal/ui/`
- [ ] **Step 5: Commit** — `feat(titlebar): Move the sidebar's strip with the sidebar`

---

### Task 5: The rail

**Files:**
- Create: `internal/ui/sidebar_rail.go`, `internal/ui/sidebar_rail_test.go`
- Modify: `internal/ui/sidebar_window.go` (`drawSide` calls `layoutRail` when `s.mode == sideRail`; the cache key gets `mode`; `sidePointer`'s hover already uses `lineAt`)
- Modify: `internal/ui/sidebar_cache.go` (`sideDrawKey.mode sideMode`)

**Interfaces:**
- Consumes: `sideModel`, `sideState` (incl. `right`, `shown`, `hover`, `scroll`, `settings`, `settingsOn`, `docked`), `stateDot`, `agentLogo`, `fitSide`.
- Produces: `func (c *chrome) layoutRail(gtx layout.Context, m sideModel, st sideState, size image.Point) sideHits`. Hits: one `sideRowHit{id, rect}` per tile in `rows` (close, title, fold empty), one per bar in `members` (id = member ID, skipped when 0), `settings` = gear tile rect while `st.settings`, `list` = tile area, `scrollMax`.

Drawing (sizes in dp): `Chrome` fill, 1 px `Border` on the inner edge. Top pad 8. Per group: label (group `Name` cut to 44 dp, `Muted`, 9.5 sp) then 36x36 tiles centred, 4 gap; a 24x1 `Border` divider between groups; groups' `Folded` is ignored (tiles always drawn); `m.Loose` rows come last with no label. Tile: 18 dp `agentLogo(row.Agent)` centred (a shell row with no agent uses the same fallback the rows use), `stateDot` 11 dp in the bottom-right on a 14 dp `Chrome` disc. Shown tile (`id == st.shown && !st.settingsOn && st.doc == 0`, or `id == st.docked`): `Active` rounded fill (radius 8) and the 2 dp `Accent` line on the window edge (`right` flips it). Workspaces: pinned at the bottom, 4x16 bars with 3 gap per member of every run, stopped members in `CloseHover` red. Hover tooltip when `st.hover != 0`: a box beside the hovered tile toward the terminal, same style as `trayTip` (ToastBG fill, ToastLine stroke, radius 7), content `stateDot`, title, status word (`needs you` / `done` / `N agents` / member status). Gear tile at the very top while `st.settings`.

- [ ] **Step 1: Write failing tests** in `sidebar_rail_test.go` using `twoGroupsOneLoose()`:
  - `TestRailTilesInSidebarOrder`: `rows` ids equal the model order of all rows, folded group included.
  - `TestRailTileClickShowsAndMiddleCloses`: in a window (`sideWindow` + `setSideMode(sideRail)`), primary press at a tile shows it; middle press closes it.
  - `TestRailBarsClickMember`: a member bar's hit shows that member (reuse `memberWindow`).
  - `TestRailSettingsTile` (Review Focus 3): with Settings open, `hits.settings` is non-empty and above the first tile; clicking a session tile leaves Settings shown false.
  - `TestRailScrollsAndPinsBars` (Review Focus 4): 30 rows at height 400: `scrollMax > 0`; scroll to max, bar rects unchanged; close 20 tabs, next frame `st.scroll <= scrollMax`.
  - `TestRailHoverTooltip`: with `st.hover = 2`, `hits.tip` is non-empty, sits outside the tile toward the terminal (right of the rail for left, left of it for right).
  - `TestRailNoAllocsWhenUnchanged`: the `sidebar_cache_test.go` pattern with rail mode: `testing.AllocsPerRun(100, draw) == 0`.
- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/ui/ -run Rail`
- [ ] **Step 3: Implement** `layoutRail` and the `drawSide` branch. The tooltip must draw outside the 52 dp rect: draw it in a second call after `drawSide`, from `win.side.hits.tip`, over the terminal (the same way editors draw live over the cache).
- [ ] **Step 4: Run, expect PASS** — `go test ./internal/ui/` and `go vet ./...`
- [ ] **Step 5: Commit** — `feat(sidebar): Add the session rail`

---

### Task 6: Settings and palette

**Files:**
- Modify: `internal/ui/settings_appearance.go` (`appearanceRows`, `appearanceAct`)
- Modify: `internal/ui/commands.go:28`, `internal/ui/logic.go:43` (actions), `internal/ui/window.go:1346` area (handlers), `internal/ui/palette_window.go:175` (icons)
- Test: `internal/ui/settings_appearance_test.go`, `internal/ui/commands_test.go`

**Interfaces:**
- Consumes: `setSideMode`, `setSideRight`, `savedSideMode`, `sideRight`.
- Produces: rows after "Blinking cursor": `{heading: "Sidebar"}`, `{label: "Side", hint: "Which edge of the window the sidebar sits on.", ctrls: segmented id "sideside", options {"Left","Right"}}`, `{label: "Width", hint: "<bound toggle_sidebar key> cycles these.", ctrls: segmented id "sidemode", options {"Full","Rail","Hidden"}}`. Commands `{"sidebar_left", "Sidebar on the left", actSidebarLeft}`, `{"sidebar_right", "Sidebar on the right", actSidebarRight}`, both with the `"panel"` palette icon. These two get no default key binding and are not added to `[keys]`.

- [ ] **Step 1: Write failing tests**
  - `TestAppearanceSidebarRows`: rows contain the heading and both segmented controls with `selected` matching config (right → 1; rail → 1).
  - `TestAppearanceSidebarActs`: `appearanceAct(win, "sideside", 1)` sets right and writes `sidebar_side = "right"`; `appearanceAct(win, "sidemode", 2)` writes `sidebar_mode = "hidden"`.
  - `TestSidebarSideCommands`: running `actSidebarRight` then `actSidebarLeft` through the command path flips `sideRight()`.
  - The Width hint names the key: on macOS `Cmd+B`, else `Ctrl+Shift+B` (use the same key label helper the shortcuts page uses).
- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/ui/ -run 'AppearanceSidebar|SidebarSideCommands'`
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run, expect PASS** — `go test ./...` and `go vet ./...`
- [ ] **Step 5: Commit** — `feat(settings): Choose the sidebar's side and width`

---

### Task 7: Visual evidence and handoff

**Files:**
- Create: `internal/ui/sidebar_side_render_test.go`
- Modify: `docs/HANDOFF.md` (top "Last updated" line and a short paragraph under "Where things stand")

- [ ] **Step 1: Add opt-in render** `TestSidebarSideRenders`, skipped unless `KAZE_SIDE_RENDER_DIR` is set: for midnight and washi, full window frames (title bar + sidebar or rail + empty terminal) at 1100x540 for left/right x full/rail, saved as `<theme>-<side>-<mode>.png`. Use `writeRender`.
- [ ] **Step 2: Run it** — `KAZE_SIDE_RENDER_DIR=<scratchpad>/side go test ./internal/ui/ -run TestSidebarSideRenders`; open the PNGs and compare with `docs/design/sidebar-right-prototype.html`. Fix any mismatch in the owning task's code with a test first.
- [ ] **Step 3: Full gates** — `go test ./...`, `go vet ./...`, `go build -o kaze ./cmd/kaze`; Windows cross-build `GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o /dev/null ./cmd/kaze`.
- [ ] **Step 4: Update HANDOFF** — what was built, the owner checks still open (macOS title bar on the right by hand; Windows caption buttons with the right strip).
- [ ] **Step 5: Commit** — `docs(handoff): Record the sidebar side and rail work`

After this: push the branch, open a PR for the owner. The release (0.43, changelog, lander, README) follows the merge, per CLAUDE.md.
