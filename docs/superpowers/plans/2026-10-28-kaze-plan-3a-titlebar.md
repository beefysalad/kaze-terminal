# kaze Plan 3a: kaze's own title bar (Windows) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On Windows, replace the OS title bar with kaze's own: tabs at the top edge, kaze-drawn minimize / maximize-restore / close, drag by empty bar space, native resize and snap.

**Architecture:** Run the Gio window undecorated (`app.Decorated(false)`) on Windows only. Gio's Windows backend already returns resize hit-tests at the window border and `HTCAPTION` for any area marked with `system.ActionInputOp(system.ActionMove)`, so dragging, double-click-to-maximize and Aero snap come from Windows itself. kaze computes the bar's regions (tabs, drag space, three caption buttons) in a pure function, draws the buttons in Midnight colors, and turns clicks into `app.Window.Perform` actions.

**Tech Stack:** Go 1.27, Gio v0.10.3 (`gioui.org/app`, `gioui.org/io/system`, `gioui.org/op/clip`, `gioui.org/op/paint`).

**Spec:** `docs/superpowers/specs/2026-10-28-kaze-terminal-design.md` section 6.8 (title bar) and 6.7 (Midnight tokens). Mockup: `docs/design/titlebar-and-status.html`.

## Global Constraints

- Windows only in this plan. On macOS keep `Decorated(true)` (native title bar): Gio hides the traffic lights when undecorated, and re-showing them inset needs darwin-only cgo that can't be tested here. That is the Mac milestone's job. Build-tag or `runtime.GOOS` gate; darwin must still compile.
- Colors (spec 6.7): bar `#0b0d12`; button glyph `#a7adbd`; button hover background `#1a1f2c` with glyph `#ffffff`; close hover background `#e5484d` with glyph `#ffffff`; pressed = hover background slightly darker (`#151925` / `#c93c40`).
- Button size: 46dp wide, full bar height (bar is 38dp today; keep it). Glyphs are 10dp line drawings, 1dp stroke: minimize = horizontal line; maximize = square outline; restore = two offset squares; close = X.
- Order right-to-left: close, maximize/restore, minimize. Nothing else may overlap the buttons; tabs shrink/scroll before overlapping them.
- Empty tab-bar space (after the `+` button, before the minimize button) is a move area. Tabs, tab close buttons and `+` are NOT move areas.
- Maximize icon follows the real window state from `app.ConfigEvent` (`Config.Mode == app.Maximized`), not a local toggle.
- No Windows 11 snap-layout flyout on hover (needs `HTMAXBUTTON`, which Gio doesn't expose). Win+Z, Win+arrows, edge drag and double-click on the bar must work.
- Idle CPU stays ~0: hover state changes request one frame; no animation loops.
- Safety: never inject global keyboard/mouse input (SendInput etc.) in tests or manual checks. RED tests bounded. Commits `type(scope): Verb ...`, no AI attribution.

## Review Focus

1. **Maximized window overflow:** undecorated windows on Windows can extend past the screen by the border width when maximized, clipping the tab bar and buttons. Expected: when maximized, kaze's content (bar + buttons) is fully visible. Pinned by Task 2's manual check and, if Gio reports insets, a layout test using them.
2. **Narrow windows:** with many tabs and a narrow window, tabs must never draw under the caption buttons and the buttons must stay clickable. Pinned in Task 1 (`TestRegionsNeverOverlapButtons`).
3. **DPI scaling:** at 150% scale the button and drag regions scale with `gtx.Dp`. Pinned in Task 1 (`TestRegionsScaleWithDp`).
4. **Click vs drag:** a press on a button that's released outside it must not fire the action (standard button behavior). Pinned in Task 1 (`TestReleaseOutsideButtonDoesNothing`).
5. **Double-click on a tab** must not maximize (only empty space is a move area). Pinned in Task 1 (`TestTabsAreNotMoveArea`).

---

### Task 1: Title-bar regions and caption-button logic (pure, testable)

**Files:**
- Create: `internal/ui/titlebar.go`
- Test: `internal/ui/titlebar_test.go`

**Interfaces:**
- Produces:

```go
type captionButton int
const (
	capNone captionButton = iota
	capMinimize
	capMaximize // shows "restore" when maximized
	capClose
)

// barLayout is where things go in the top bar, in pixels.
type barLayout struct {
	Tabs    image.Rectangle            // where the tab strip may draw (ends before Drag)
	Drag    image.Rectangle            // empty space that moves the window; may be empty
	Buttons [3]image.Rectangle         // indexed capMinimize-1, capMaximize-1, capClose-1
}

// layoutBar splits a bar of the given size. tabsWidth is how wide the tab strip
// wants to be (tabs + plus button); buttonW is 46dp in pixels. If custom is false
// (macOS / decorated), Buttons are empty and Drag is empty.
func layoutBar(size image.Point, tabsWidth, buttonW int, custom bool) barLayout

// captionAt returns which caption button contains p.
func (b barLayout) captionAt(p image.Point) captionButton

// captionState tracks hover/press for the three buttons across pointer events.
type captionState struct{ hover, pressed captionButton }

// pointer feeds one pointer event (kind, position) and returns the button whose
// action should fire now (capNone if none). Fires on release inside the pressed button.
func (s *captionState) pointer(kind pointer.Kind, p image.Point, b barLayout) captionButton

// actionFor maps a fired button to the window action, given the real maximized state.
func actionFor(c captionButton, maximized bool) system.Action
```

- [ ] **Step 1: Write the failing tests** in `internal/ui/titlebar_test.go`:

```go
func TestButtonsAreRightAlignedInOrder(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Buttons[2] != image.Rect(954, 0, 1000, 38) { t.Fatalf("close %v", b.Buttons[2]) }
	if b.Buttons[1] != image.Rect(908, 0, 954, 38) { t.Fatalf("max %v", b.Buttons[1]) }
	if b.Buttons[0] != image.Rect(862, 0, 908, 38) { t.Fatalf("min %v", b.Buttons[0]) }
}
func TestDragFillsSpaceBetweenTabsAndButtons(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Drag != image.Rect(300, 0, 862, 38) { t.Fatalf("drag %v", b.Drag) }
}
func TestRegionsNeverOverlapButtons(t *testing.T) {
	b := layoutBar(image.Pt(400, 38), 900, 46, true) // tabs want more than fits
	if b.Tabs.Max.X > b.Buttons[0].Min.X { t.Fatalf("tabs %v overlap min %v", b.Tabs, b.Buttons[0]) }
	if !b.Drag.Empty() { t.Fatalf("no room for drag, got %v", b.Drag) }
}
func TestRegionsScaleWithDp(t *testing.T) {
	b := layoutBar(image.Pt(1500, 57), 300, 69, true) // 150%
	if b.Buttons[2].Dx() != 69 || b.Buttons[2].Dy() != 57 { t.Fatalf("%v", b.Buttons[2]) }
}
func TestDecoratedHasNoButtonsOrDrag(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, false)
	if !b.Buttons[0].Empty() || !b.Drag.Empty() || b.Tabs.Max.X != 1000 { t.Fatalf("%+v", b) }
}
func TestTabsAreNotMoveArea(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	if b.Drag.Overlaps(b.Tabs) { t.Fatalf("drag %v overlaps tabs %v", b.Drag, b.Tabs) }
}
func TestClickFiresOnReleaseInside(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	var s captionState
	p := b.Buttons[0].Min.Add(image.Pt(5, 5))
	if s.pointer(pointer.Press, p, b) != capNone { t.Fatal("fired on press") }
	if s.pointer(pointer.Release, p, b) != capMinimize { t.Fatal("no fire on release") }
}
func TestReleaseOutsideButtonDoesNothing(t *testing.T) {
	b := layoutBar(image.Pt(1000, 38), 300, 46, true)
	var s captionState
	s.pointer(pointer.Press, b.Buttons[2].Min.Add(image.Pt(5, 5)), b)
	if s.pointer(pointer.Release, image.Pt(500, 20), b) != capNone { t.Fatal("fired outside") }
}
func TestMaximizeTogglesWithRealState(t *testing.T) {
	if actionFor(capMaximize, false) != system.ActionMaximize { t.Fatal("max") }
	if actionFor(capMaximize, true) != system.ActionUnmaximize { t.Fatal("restore") }
	if actionFor(capMinimize, true) != system.ActionMinimize || actionFor(capClose, false) != system.ActionClose { t.Fatal("min/close") }
}
```

- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Bar|Button|Drag|Regions|Click|Release|Maximize|Tabs|Decorated'`. Expected: FAIL (undefined: layoutBar).
- [ ] **Step 3: Implement** `internal/ui/titlebar.go` with the signatures above. Hover is tracked on `pointer.Move`/`Enter`/`Leave`; `pointer.Cancel` clears both states.
- [ ] **Step 4: Run** the same command. Expected: PASS. Then `go test ./internal/ui/` all pass.
- [ ] **Step 5: Commit** `feat(ui): Lay out kaze's own title bar regions`

---

### Task 2: Undecorated window, drawn caption buttons, drag area (Windows)

**Files:**
- Modify: `cmd/kaze/main.go` (window options), `internal/ui/window.go` (frame: bar layout, events, ConfigEvent maximized state), `internal/ui/tabbar.go` (tab strip limited to `barLayout.Tabs`; draw caption buttons)
- Create: `internal/ui/decor_windows.go`, `internal/ui/decor_other.go` (`const customTitleBar = true/false` by build tag)
- Test: `internal/ui/titlebar_test.go` (extend)

**Interfaces:**
- Consumes: Task 1's `layoutBar`, `captionState`, `actionFor`.
- Produces: `func (win *window) perform(a system.Action)` (calls `win.w.Perform(a)`; a func field `performFn` lets tests capture actions). `win.maximized bool` updated from every `app.ConfigEvent` (`e.Config.Mode == app.Maximized`).

- [ ] **Step 1: Write the failing tests:**

```go
func TestCaptionClickPerformsAction(t *testing.T) {
	// window with a fake performFn; feed press+release on the close button through barEvents' caption path
	// assert performFn received exactly system.ActionClose
}
func TestMaximizeButtonUsesConfigState(t *testing.T) {
	// set win.maximized = true via a ConfigEvent{Config{Mode: app.Maximized}} handler call;
	// click maximize; assert ActionUnmaximize
}
func TestTabStripStopsBeforeButtons(t *testing.T) {
	// many tabs, narrow width; after layoutTabBar, every tab hit rect and the + rect is left of Buttons[0].Min.X
}
```

- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'Caption|MaximizeButton|TabStrip'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - `main.go`: on Windows add `app.Decorated(false)` to the window options; elsewhere unchanged.
  - `frame`: compute `barLayout` with `layoutBar(image.Pt(size.X, barH), tabStripWidth, gtx.Dp(46), customTitleBar)`; lay the tab strip inside `Tabs`; push `clip.Rect(Drag)` + `system.ActionInputOp(system.ActionMove)` for the drag area; draw the three buttons (background per state, glyph lines with `clip.Stroke`) and route pointer events in their rects through `captionState`, calling `win.perform(actionFor(...))` on fire; request a frame on hover change only.
  - Keep keyboard shortcuts and everything else as is. Window title (`app.Title`) still updates (taskbar/Alt+Tab use it).
- [ ] **Step 4: Run** `go test ./internal/ui/` and `go vet ./...`; `GOOS=darwin go vet ./internal/ui/` is not possible (cgo), so make sure `decor_other.go` has the same symbols. Expected: PASS.
- [ ] **Step 5: Build and look (no input injection).** `go build -ldflags "-H=windowsgui -s -w -X main.r=<random>" -o <worktree-or-temp>\kaze-3a.exe ./cmd/kaze`, launch it **without activating** (`Start-Process` then don't type), screenshot the window normally and after `win.w.Perform(system.ActionMaximize)` triggered by a temporary env hook `KAZE_DEBUG_MAXIMIZE=1` (remove before commit, or keep behind the existing bench flag if trivial). Check: no OS title bar; buttons right-aligned; when maximized nothing is clipped at the top/right edge (Review Focus 1). Close what you start. Record findings; the owner does the interactive checks (drag, double-click, snap, resize, hover) by hand.
- [ ] **Step 6: Commit** `feat(ui): Draw kaze's own title bar on Windows`

---

## Owner's hand checklist (after Task 2)

Drag by empty bar space · double-click empty space maximizes/restores · drag to screen edges snaps · Win+Z and Win+arrows work · resize from all edges and corners · hover colors (close turns red) · minimize/restore from the taskbar · Alt+F4 closes · maximized window isn't clipped.
