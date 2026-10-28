# kaze Plan 1: Terminal Core with Tabs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A fast, cross-platform (Windows + macOS) terminal with tabs that can run Claude Code all day, meeting the design's performance targets.

**Architecture:** Each tab owns a PTY session, a VT parser and a screen model, connected by one reader goroutine per tab. The screen model tracks dirty lines; a Gio renderer draws only dirty lines using a glyph cache, and only when something changed. Everything kaze-specific (subagent tabs, alerts, status bar) is out of this plan and builds on the `tabs` and `screen` packages later.

**Tech Stack:** Go 1.23+, Gio (`gioui.org`), `github.com/aymanbagabas/go-pty` (ConPTY on Windows, Unix pty on macOS), `github.com/mattn/go-runewidth`, `github.com/BurntSushi/toml`.

**Spec:** `docs/superpowers/specs/2026-10-28-kaze-terminal-design.md`

**Follow-up plans (not in this one):** Plan 2 subagent child tabs (spec 6.3), Plan 3 attention alerts (6.4), Plan 4 status bar (6.5). Later: browser button, brain graph (spec 7).

## Global Constraints

- Platforms: Windows and macOS from one codebase. No Linux-specific work. Every task's tests must pass on Windows; platform-specific code uses `_windows.go` / `_darwin.go` / `_unix.go` build suffixes.
- Module path: `github.com/patrick-gogo/kaze-terminal`. Binary: `kaze`.
- Performance goals (spec section 5, revised 2026-10-28): keypress to glyph under 10ms; Claude Code streaming frames under 16.7ms with no flicker or tearing; launch to ready prompt under 500ms; smooth scroll through 10,000 lines; idle CPU effectively zero; memory has no hard cap but is measured and must not grow with idle tabs. Go and Gio are fixed (Patrick's ruling); beating Warp is not a goal.
- Rendering rules (spec 6.2): redraw only dirty lines; rasterize each glyph once and reuse; no frame when nothing changed; draw at most once per display refresh.
- Sessions do not survive kaze closing or crashing. Shells live in the kaze process, behind the `ptyx.Session` interface.
- Japanese: display and paste only. No IME composition.
- Look: "Midnight" (spec 6.7). `render.DefaultPalette()` and the tab bar / status bar colors use the spec 6.7 token values exactly; no other colors are hard-coded.
- Default shell: `pwsh` on Windows (fall back to `powershell.exe` if `pwsh` is not on PATH), `$SHELL` or `/bin/zsh` on macOS.
- Each tab's shell gets the environment variable `KAZE_TAB_ID=<id>` (Plan 3 depends on this name).
- No settings UI. Config at `<os.UserConfigDir()>/kaze/config.toml`.
- Commits: `type(scope): Verb ...` (e.g. `feat(screen): Add dirty line tracking`). Commit as `patrick-gogo <283561876+patrick-gogo@users.noreply.github.com>`. No AI attribution lines.

## Review Focus

1. **UTF-8 split across PTY reads.** A Japanese character whose bytes arrive in two reads must render as one character, not two replacement glyphs. Test in Task 4.
2. **Wide character at the last column.** A two-column character that doesn't fit on the line wraps to the next line whole, instead of being cut in half. Test in Task 2.
3. **Shell exits** (`exit`, or the shell crashes). The tab shows "[process exited]" and stays open until closed. It must not hang or take down other tabs. Test in Task 9.
4. **Window resize while output is streaming.** No panic and no out-of-range cursor. The PTY is told the new size. Test in Task 9.
5. **Huge paste** (1MB of text). The UI doesn't freeze; writes go to the PTY from a goroutine, wrapped in bracketed-paste markers when that mode is on. Test in Task 11.

---

### Task 0: Gio speed spike (go/no-go gate)

The spec says Gio must prove it meets the targets before features are built on it (spec 11.3), testing both Gio's normal text path and the glyph-atlas fallback (spec 10.1). This task is a measurement, not product code.

**Files:**
- Create: `spike/gio/main.go`, `spike/gio/README.md`
- Create: `docs/spike-results.md`

- [ ] **Step 1: Install tooling.** `winget install GoLang.Go`, then confirm `go version` reports 1.23 or newer. `go mod init github.com/patrick-gogo/kaze-terminal` at the repo root.
- [ ] **Step 2: Build the spike.** A Gio window showing a 200x60 grid of mixed ASCII and Japanese text (`日本語テスト` repeated) in a monospace font with a CJK fallback font. Two modes picked by flag `-mode=text|atlas`:
  - `text`: Gio `widget.Label` / `text.Shaper` per line, caching shaped lines per line content.
  - `atlas`: rasterize each glyph once into an `image.RGBA` atlas, draw cells as `paint.ImageOp` sub-images.
  Keystroke changes one cell. Flag `-stream` rewrites 5 random lines every 2ms to imitate Claude streaming.
- [ ] **Step 3: Measure on Windows** and record in `docs/spike-results.md` for both modes: frame time for a one-cell change (p50/p99, from `time.Now()` around the frame), frame time under `-stream`, process private memory after 10s idle (Task Manager "Memory" column or `Get-Process kaze | Select WorkingSet64,PrivateMemorySize64`), idle CPU over 30s, visible tearing yes/no.
- [ ] **Step 4: Decide.** Pass if either mode gets one-cell frame time p99 under 4ms, memory under 40MB with one grid, idle CPU ~0, and no visible tearing under `-stream`. Record which mode wins; Task 10 uses it. If neither passes, **stop and report to Patrick** (spec 10.1: reopening the language is his call).
- [ ] **Step 5: Commit**

```bash
git add spike docs/spike-results.md go.mod go.sum
git commit -m "chore(spike): Measure Gio text and atlas rendering paths"
```

macOS numbers for the spike are taken at the first milestone run on the MacBook (spec 10.3), not here.

**Outcome (2026-10-28):** neither mode passed (see `docs/spike-results.md`). Patrick ruled: keep Go and Gio, drop the memory cap and the Warp bar. Task 0b picks the drawing method instead of gating.

---

### Task 0b: Second spike: one CPU-drawn grid image, changed rows only

Not a go/no-go gate. It picks the drawing method Task 10 uses.

**Files:**
- Modify: `spike/gio/main.go` (add `-mode=image`)
- Modify: `docs/spike-results.md` (add a section for the new mode and the decision)

- [ ] **Step 1: Add `-mode=image`.** Keep one `*image.RGBA` the size of the whole grid. Rasterize glyphs once into a glyph cache (reuse the atlas code), and on each change draw only the changed cells' glyphs into the grid image on the CPU. Hand Gio the whole grid as ONE `paint.ImageOp` per frame (a single paint op), so Gio has one texture to show instead of thousands of ops. If Gio re-uploads the full texture on every change, record that cost; also try splitting the grid into horizontal strips (e.g. 8 strips, one `ImageOp` each, only changed strips get a new image) and record both.
- [ ] **Step 2: Measure on Windows** with the same commands as Task 0: one-cell frame time p50/p99 (`-autotype`), `-stream` frame time p50/p99 and late frames (over 16.7ms), private memory after 10s idle, idle CPU over 30s.
- [ ] **Step 3: Decide.** Pick the fastest mode across `text`, `atlas`, `image` (and the strip variant) by: stream p99 first (goal under 16.7ms), then one-cell p99 (goal under 10ms). Record the winner and whether each goal was met. Goals not met are recorded, not blocking.
- [ ] **Step 4: Commit** `chore(spike): Measure single-image grid rendering`

---

### Task 1: Screen grid with dirty line tracking

**Files:**
- Create: `internal/screen/cell.go`, `internal/screen/screen.go`
- Test: `internal/screen/screen_test.go`

**Interfaces:**
- Produces:
  - `type Color uint32` with `const DefaultColor Color = 1 << 31`; indexed colors 0-255 stored as the value, truecolor as `1<<30 | r<<16 | g<<8 | b`. Helpers `Indexed(n uint8) Color`, `RGB(r, g, b uint8) Color`.
  - `type Attr uint8` with `AttrBold, AttrItalic, AttrUnderline, AttrInverse, AttrDim, AttrStrike`.
  - `type Cell struct { Rune rune; Width uint8; FG, BG Color; Attrs Attr }`. `Width` is 1, 2 (lead half of wide char) or 0 (continuation half).
  - `type Screen struct` with `New(cols, rows int) *Screen`, `Size() (cols, rows int)`, `Cell(col, row int) Cell`, `SetCell(col, row int, c Cell)`, `Cursor() (col, row int)`, `SetCursor(col, row int)` (clamped), `TakeDirty() []int` (rows changed since last call, ascending, then clears), `MarkAllDirty()`.

- [ ] **Step 1: Write the failing tests**

```go
func TestNewScreenIsBlank(t *testing.T) {
	s := screen.New(80, 24)
	c, r := s.Size()
	if c != 80 || r != 24 { t.Fatalf("size %dx%d", c, r) }
	if got := s.Cell(0, 0); got.Rune != ' ' || got.Width != 1 || got.FG != screen.DefaultColor {
		t.Fatalf("blank cell = %+v", got)
	}
}

func TestDirtyRowsReportedOnceThenCleared(t *testing.T) {
	s := screen.New(80, 24)
	s.TakeDirty()
	s.SetCell(3, 5, screen.Cell{Rune: 'a', Width: 1})
	s.SetCell(4, 5, screen.Cell{Rune: 'b', Width: 1})
	s.SetCell(0, 2, screen.Cell{Rune: 'c', Width: 1})
	if got := s.TakeDirty(); !reflect.DeepEqual(got, []int{2, 5}) { t.Fatalf("dirty = %v", got) }
	if got := s.TakeDirty(); len(got) != 0 { t.Fatalf("second take = %v", got) }
}

func TestSettingSameCellIsNotDirty(t *testing.T) {
	s := screen.New(80, 24)
	s.TakeDirty()
	s.SetCell(0, 0, s.Cell(0, 0))
	if got := s.TakeDirty(); len(got) != 0 { t.Fatalf("dirty = %v", got) }
}

func TestCursorIsClamped(t *testing.T) {
	s := screen.New(80, 24)
	s.SetCursor(500, -3)
	if c, r := s.Cursor(); c != 79 || r != 0 { t.Fatalf("cursor %d,%d", c, r) }
}
```

- [ ] **Step 2: Run** `go test ./internal/screen/...`. Expected: FAIL (package does not exist).
- [ ] **Step 3: Implement** in `cell.go` and `screen.go`. Store cells as one flat `[]Cell` (row-major) plus a `[]bool` dirty slice per row; no per-cell allocation.
- [ ] **Step 4: Run** `go test ./internal/screen/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(screen): Add cell grid with dirty line tracking`

---

### Task 2: Printing text, wide characters and auto-wrap

**Files:**
- Modify: `internal/screen/screen.go`
- Create: `internal/screen/print.go`
- Test: `internal/screen/print_test.go`

**Interfaces:**
- Consumes: Task 1 `Screen`, `Cell`.
- Produces: `(*Screen).Print(r rune)` (writes at the cursor using the current pen, advances, wraps); `(*Screen).Pen() *Cell` (current FG/BG/Attrs template, Rune ignored); `(*Screen).SetAutoWrap(on bool)` (default on); `(*Screen).LineFeed()`, `(*Screen).CarriageReturn()`. `LineFeed` at the bottom scrolls the screen up by one (Task 3 hooks scrollback into this).

- [ ] **Step 1: Write the failing tests**

```go
func TestPrintAdvancesCursor(t *testing.T) {
	s := screen.New(10, 3)
	for _, r := range "hi" { s.Print(r) }
	if c, _ := s.Cursor(); c != 2 { t.Fatalf("col %d", c) }
	if s.Cell(1, 0).Rune != 'i' { t.Fatal("wrong cell") }
}

func TestWideCharOccupiesTwoCells(t *testing.T) {
	s := screen.New(10, 3)
	s.Print('日')
	if got := s.Cell(0, 0); got.Rune != '日' || got.Width != 2 { t.Fatalf("lead %+v", got) }
	if got := s.Cell(1, 0); got.Width != 0 { t.Fatalf("continuation %+v", got) }
	if c, _ := s.Cursor(); c != 2 { t.Fatalf("col %d", c) }
}

func TestWideCharAtLastColumnWrapsWhole(t *testing.T) {
	s := screen.New(5, 3)
	for _, r := range "abcd" { s.Print(r) }
	s.Print('日')
	if s.Cell(4, 0).Rune != ' ' { t.Fatalf("last col should be blank, got %q", s.Cell(4, 0).Rune) }
	if s.Cell(0, 1).Rune != '日' { t.Fatal("wide char should start next line") }
}

func TestOverwritingHalfOfWideCharClearsBothHalves(t *testing.T) {
	s := screen.New(10, 3)
	s.Print('日')
	s.SetCursor(1, 0)
	s.Print('x')
	if got := s.Cell(0, 0); got.Rune != ' ' || got.Width != 1 { t.Fatalf("lead not cleared: %+v", got) }
	if s.Cell(1, 0).Rune != 'x' { t.Fatal("x not written") }
}

func TestPrintAtBottomRightWrapsAndScrolls(t *testing.T) {
	s := screen.New(3, 2)
	for _, r := range "abcdefg" { s.Print(r) }
	if s.Cell(0, 0).Rune != 'd' || s.Cell(0, 1).Rune != 'g' { t.Fatal("did not scroll") }
}
```

- [ ] **Step 2: Run** `go test ./internal/screen/...`. Expected: FAIL.
- [ ] **Step 3: Implement.** Width via `runewidth.RuneWidth` with `runewidth.DefaultCondition.EastAsianWidth = false` (ambiguous-width chars are 1 column, matching Claude Code's assumptions). Zero-width runes (combining marks) are dropped in v1. Use the xterm "pending wrap" rule: printing in the last column sets a wrap-pending flag, and the wrap happens on the next printable rune.
- [ ] **Step 4: Run** `go test ./internal/screen/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(screen): Print text with wide characters and auto-wrap`

---

### Task 3: Scrollback with a hard cap

**Files:**
- Create: `internal/screen/scrollback.go`
- Modify: `internal/screen/screen.go` (scroll-up pushes the top line into scrollback when the scroll region is the full screen and the main screen is active)
- Test: `internal/screen/scrollback_test.go`

**Interfaces:**
- Produces: `NewWithScrollback(cols, rows, maxLines int) *Screen` (`New` uses 10000); `(*Screen).ScrollbackLen() int`; `(*Screen).ScrollbackLine(i int) []Cell` (0 = oldest; returned slice must not be modified); `(*Screen).SetViewOffset(n int)` (lines scrolled back from the bottom, clamped to `[0, ScrollbackLen()]`, marks all rows dirty); `(*Screen).ViewOffset() int`; `(*Screen).VisibleCell(col, row int) Cell` (what the renderer draws, taking the view offset into account).

- [ ] **Step 1: Write the failing tests**

```go
func TestScrolledLinesGoToScrollback(t *testing.T) {
	s := screen.NewWithScrollback(3, 2, 100)
	for _, r := range "aaabbbccc" { s.Print(r) }
	if s.ScrollbackLen() != 1 || s.ScrollbackLine(0)[0].Rune != 'a' { t.Fatal("expected 'aaa' in scrollback") }
}

func TestScrollbackIsCapped(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 5)
	for i := 0; i < 20; i++ { s.Print(rune('a' + i)); s.LineFeed(); s.CarriageReturn() }
	if s.ScrollbackLen() != 5 { t.Fatalf("len %d", s.ScrollbackLen()) }
}

func TestViewOffsetShowsOlderLines(t *testing.T) {
	s := screen.NewWithScrollback(1, 1, 100)
	for _, r := range "abc" { s.Print(r); s.LineFeed(); s.CarriageReturn() }
	s.SetViewOffset(2)
	if s.VisibleCell(0, 0).Rune != 'b' { t.Fatalf("got %q", s.VisibleCell(0, 0).Rune) }
}
```

- [ ] **Step 2: Run** `go test ./internal/screen/...`. Expected: FAIL.
- [ ] **Step 3: Implement** a ring buffer of fixed-width `[]Cell` lines. Lines are stored at the width they had when scrolled off; `ScrollbackLine` returns them as-is and `VisibleCell` pads short lines with blank cells. No reflow on resize in v1.
- [ ] **Step 4: Run** `go test ./internal/screen/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(screen): Add capped scrollback and view offset`

---

### Task 4: VT parser state machine

**Files:**
- Create: `internal/vt/parser.go`, `internal/vt/handler.go`
- Test: `internal/vt/parser_test.go`

**Interfaces:**
- Produces:

```go
type Handler interface {
	Print(r rune)
	Execute(b byte)                                          // C0 controls: BEL, BS, HT, LF, VT, FF, CR
	CSI(params []int, intermediates []byte, private byte, final byte) // private is '?', '>', '<', '=' or 0; missing params are -1
	ESC(intermediates []byte, final byte)
	OSC(data []byte)                                         // without the leading "ESC ]" and terminator
}
type Parser struct{ /* ... */ }
func NewParser(h Handler) *Parser
func (p *Parser) Feed(data []byte)
```

- [ ] **Step 1: Write the failing tests** using a recording `Handler` that appends each call to a `[]string` (e.g. `"print 日"`, `"csi ? [1049] h"`, `"exec 0a"`, `"osc 0;title"`).

```go
func TestUTF8SplitAcrossFeeds(t *testing.T) {
	rec := &recorder{}
	p := vt.NewParser(rec)
	b := []byte("日")
	p.Feed(b[:1]); p.Feed(b[1:])
	if !reflect.DeepEqual(rec.calls, []string{"print 日"}) { t.Fatalf("%v", rec.calls) }
}

func TestCSIWithPrivateMarkerAndParams(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte("\x1b[?1049h\x1b[38;2;10;20;30m\x1b[H"))
	want := []string{"csi ? [1049] h", "csi [38 2 10 20 30] m", "csi [-1] H"}
	if !reflect.DeepEqual(rec.calls, want) { t.Fatalf("%v", rec.calls) }
}

func TestCSISplitAcrossFeeds(t *testing.T) {
	rec := &recorder{}
	p := vt.NewParser(rec)
	p.Feed([]byte("\x1b[3")); p.Feed([]byte("1m"))
	if !reflect.DeepEqual(rec.calls, []string{"csi [31] m"}) { t.Fatalf("%v", rec.calls) }
}

func TestOSCTerminatedByBELAndST(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte("\x1b]0;one\x07\x1b]2;two\x1b\\"))
	if !reflect.DeepEqual(rec.calls, []string{"osc 0;one", "osc 2;two"}) { t.Fatalf("%v", rec.calls) }
}

func TestInvalidUTF8BecomesReplacementChar(t *testing.T) {
	rec := &recorder{}
	vt.NewParser(rec).Feed([]byte{0xff, 'a'})
	if !reflect.DeepEqual(rec.calls, []string{"print �", "print a"}) { t.Fatalf("%v", rec.calls) }
}
```

- [ ] **Step 2: Run** `go test ./internal/vt/...`. Expected: FAIL.
- [ ] **Step 3: Implement** the ground / escape / escape-intermediate / csi-entry / csi-param / csi-intermediate / csi-ignore / osc-string / dcs-ignore states from the Paul Williams DEC parser model. DCS and SOS/PM/APC are consumed and ignored. Cap params at 32 and OSC data at 64KB (extra is dropped). `Feed` must not allocate per byte; reuse the param and OSC buffers.
- [ ] **Step 4: Run** `go test ./internal/vt/...`. Expected: PASS. Also add `BenchmarkFeedASCII` and `BenchmarkFeedMixed` (1MB input) and record ns/op in the commit message body.
- [ ] **Step 5: Commit** `feat(vt): Add escape sequence parser`

---

### Task 5: Cursor movement, erase, scroll regions and line editing

**Files:**
- Create: `internal/term/apply.go` (a `vt.Handler` that drives a `*screen.Screen`)
- Modify: `internal/screen/screen.go` (add the operations below)
- Test: `internal/term/apply_test.go`

**Interfaces:**
- Consumes: `vt.Handler`, `vt.Parser`, `screen.Screen`.
- Produces: `type Applier struct` with `NewApplier(s *screen.Screen, reply io.Writer) *Applier` implementing `vt.Handler`. `reply` receives terminal responses (Task 7). New `Screen` methods: `EraseInDisplay(mode int)`, `EraseInLine(mode int)`, `SetScrollRegion(top, bottom int)`, `ScrollUp(n int)`, `ScrollDown(n int)`, `InsertLines(n int)`, `DeleteLines(n int)`, `InsertChars(n int)`, `DeleteChars(n int)`, `EraseChars(n int)`, `ReverseIndex()`, `SaveCursor()`, `RestoreCursor()`, `TabForward(n int)`.

- [ ] **Step 1: Write the failing tests.** Helper `run(cols, rows int, input string) *screen.Screen` feeds input through `vt.NewParser(term.NewApplier(s, io.Discard))`, and `row(s, r) string` returns the row's runes, trimmed of trailing spaces.

```go
func TestCUPIsOneBased(t *testing.T) {
	s := run(10, 5, "\x1b[3;4Hx")
	if row(s, 2) != "   x" { t.Fatalf("%q", row(s, 2)) }
}
func TestEraseInLineToEnd(t *testing.T) {
	s := run(10, 1, "abcdef\x1b[1;3H\x1b[K")
	if row(s, 0) != "ab" { t.Fatalf("%q", row(s, 0)) }
}
func TestEraseDisplayAll(t *testing.T) {
	s := run(5, 2, "abc\r\nde\x1b[2J")
	if row(s, 0) != "" || row(s, 1) != "" { t.Fatal("not cleared") }
}
func TestScrollRegionKeepsLinesOutsideIt(t *testing.T) {
	s := run(5, 4, "top\r\nA\r\nB\r\nbot\x1b[2;3r\x1b[3;1H\n")
	if row(s, 0) != "top" || row(s, 1) != "B" || row(s, 2) != "" || row(s, 3) != "bot" {
		t.Fatalf("%q %q %q %q", row(s, 0), row(s, 1), row(s, 2), row(s, 3))
	}
}
func TestBackspaceCarriageReturnTab(t *testing.T) {
	s := run(20, 1, "ab\bX\rZ\tT")
	if row(s, 0) != "ZX      T" { t.Fatalf("%q", row(s, 0)) }
}
func TestDeleteAndInsertChars(t *testing.T) {
	s := run(10, 1, "abcdef\x1b[1;2H\x1b[2P\x1b[1@")
	if row(s, 0) != "a def" { t.Fatalf("%q", row(s, 0)) }
}
```

- [ ] **Step 2: Run** `go test ./internal/...`. Expected: FAIL.
- [ ] **Step 3: Implement** CSI finals `A B C D E F G H f d J K L M P @ X S T r s u` plus `ESC 7`, `ESC 8`, `ESC D`, `ESC E`, `ESC M`, `ESC c` (full reset), and C0 `BS HT LF VT FF CR` (LF/VT/FF all do line feed; BEL is ignored for now). Missing or zero params default to 1 where xterm does.
- [ ] **Step 4: Run** `go test ./internal/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(term): Apply cursor, erase and scroll sequences to the screen`

---

### Task 6: Colors and text attributes (SGR)

**Files:**
- Modify: `internal/term/apply.go`
- Create: `internal/term/sgr.go`
- Test: `internal/term/sgr_test.go`

**Interfaces:**
- Consumes: `screen.Color`, `screen.Indexed`, `screen.RGB`, `(*Screen).Pen()`.

- [ ] **Step 1: Write the failing tests**

```go
func TestSGRBasicAndReset(t *testing.T) {
	s := run(10, 1, "\x1b[1;31ma\x1b[0mb")
	a, b := s.Cell(0, 0), s.Cell(1, 0)
	if a.FG != screen.Indexed(1) || a.Attrs&screen.AttrBold == 0 { t.Fatalf("a %+v", a) }
	if b.FG != screen.DefaultColor || b.Attrs != 0 { t.Fatalf("b %+v", b) }
}
func TestSGR256AndTruecolor(t *testing.T) {
	s := run(10, 1, "\x1b[38;5;208ma\x1b[48;2;1;2;3mb")
	if s.Cell(0, 0).FG != screen.Indexed(208) { t.Fatal("256") }
	if s.Cell(1, 0).BG != screen.RGB(1, 2, 3) { t.Fatal("truecolor") }
}
func TestSGRBrightColors(t *testing.T) {
	s := run(10, 1, "\x1b[92ma")
	if s.Cell(0, 0).FG != screen.Indexed(10) { t.Fatal("bright green") }
}
func TestSGRColonSubparamsDoNotCrash(t *testing.T) {
	run(10, 1, "\x1b[4:3m\x1b[58:2::1:2:3ma") // curly underline, underline color: accepted, may be ignored
}
```

- [ ] **Step 2: Run** `go test ./internal/term/...`. Expected: FAIL.
- [ ] **Step 3: Implement** SGR 0, 1, 2, 3, 4, 7, 9, 22, 23, 24, 27, 29, 30-37, 38, 39, 40-47, 48, 49, 90-97, 100-107. The parser (Task 4) treats `:` like `;` for params; unknown codes are ignored.
- [ ] **Step 4: Run** `go test ./internal/term/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(term): Support colors and text attributes`

---

### Task 7: Terminal modes and replies

**Files:**
- Create: `internal/term/modes.go`
- Modify: `internal/term/apply.go`, `internal/screen/screen.go` (alternate screen buffer)
- Test: `internal/term/modes_test.go`

**Interfaces:**
- Produces:
  - `type Modes struct { AppCursorKeys, BracketedPaste, CursorVisible, MouseX10, MouseButton, MouseAny, MouseSGR, Synchronized, AltScreen bool }` and `(*Applier).Modes() Modes`.
  - `(*Applier).SyncSince() time.Time` (when synchronized output last started; zero if off).
  - `(*Applier).Title() string` (from OSC 0/2).
  - `(*Screen).UseAltScreen(on bool)` (switching clears the alt screen and marks all rows dirty; alt screen has no scrollback).

- [ ] **Step 1: Write the failing tests**

```go
func TestAltScreenSwitchKeepsMainContent(t *testing.T) {
	s := run(10, 2, "main\x1b[?1049hALT\x1b[?1049l")
	if row(s, 0) != "main" { t.Fatalf("%q", row(s, 0)) }
}
func TestModesToggle(t *testing.T) {
	a := applierFor(run2(10, 2, "\x1b[?1h\x1b[?2004h\x1b[?25l\x1b[?1002h\x1b[?1006h\x1b[?2026h"))
	m := a.Modes()
	if !m.AppCursorKeys || !m.BracketedPaste || m.CursorVisible || !m.MouseButton || !m.MouseSGR || !m.Synchronized {
		t.Fatalf("%+v", m)
	}
}
func TestDeviceStatusReportRepliesCursorPosition(t *testing.T) {
	var buf bytes.Buffer
	s := screen.New(10, 5)
	vt.NewParser(term.NewApplier(s, &buf)).Feed([]byte("\x1b[2;3H\x1b[6n"))
	if buf.String() != "\x1b[2;3R" { t.Fatalf("%q", buf.String()) }
}
func TestPrimaryDeviceAttributesReply(t *testing.T) {
	var buf bytes.Buffer
	vt.NewParser(term.NewApplier(screen.New(10, 5), &buf)).Feed([]byte("\x1b[c"))
	if buf.String() != "\x1b[?62;22c" { t.Fatalf("%q", buf.String()) }
}
func TestSynchronizedModeQueryReply(t *testing.T) {
	var buf bytes.Buffer
	vt.NewParser(term.NewApplier(screen.New(10, 5), &buf)).Feed([]byte("\x1b[?2026$p"))
	if buf.String() != "\x1b[?2026;2$y" { t.Fatalf("%q", buf.String()) }
}
func TestOSCTitle(t *testing.T) {
	a := applierFor(run2(10, 1, "\x1b]0;claude\x07"))
	if a.Title() != "claude" { t.Fatalf("%q", a.Title()) }
}
```

(`run2` returns the applier as well as the screen; `applierFor` picks it out.)

- [ ] **Step 2: Run** `go test ./internal/...`. Expected: FAIL.
- [ ] **Step 3: Implement** DEC private modes 1, 25, 47, 1047, 1049, 1000, 1002, 1003, 1006, 2004, 2026; DSR 5 and 6; DA1; DECRQM for 2026 (reply `2` when reset, `1` when set). Before writing this, run Claude Code inside Windows Terminal with output logged (`script`-style capture through a tiny Go `go-pty` recorder in `tools/capture/`) and note in the commit body which of these modes Claude Code actually uses (spec 11.1).
- [ ] **Step 4: Run** `go test ./internal/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(term): Support modes, alternate screen and status replies`

---

### Task 8: Cross-platform PTY session

**Files:**
- Create: `internal/ptyx/session.go`, `internal/ptyx/shell_windows.go`, `internal/ptyx/shell_unix.go`
- Test: `internal/ptyx/session_test.go`

**Interfaces:**
- Produces:

```go
type Session interface {
	io.ReadWriter
	Resize(cols, rows int) error
	Close() error
	Wait() (exitCode int, err error)
}
func Start(argv []string, cols, rows int, env []string, dir string) (Session, error)
func DefaultShell() []string // pwsh / powershell.exe on Windows; $SHELL or /bin/zsh -l on macOS
```

- [ ] **Step 1: Write the failing tests**

```go
func TestEchoThroughPTY(t *testing.T) {
	argv := []string{"sh", "-c", "echo kaze-ok"}
	if runtime.GOOS == "windows" { argv = []string{"cmd.exe", "/c", "echo kaze-ok"} }
	s, err := ptyx.Start(argv, 80, 24, os.Environ(), "")
	if err != nil { t.Fatal(err) }
	defer s.Close()
	out := readUntil(t, s, "kaze-ok", 5*time.Second)
	if !strings.Contains(out, "kaze-ok") { t.Fatalf("%q", out) }
}
func TestEnvIsPassedThrough(t *testing.T) {
	argv := []string{"sh", "-c", "echo $KAZE_TAB_ID"}
	if runtime.GOOS == "windows" { argv = []string{"cmd.exe", "/c", "echo %KAZE_TAB_ID%"} }
	s, err := ptyx.Start(argv, 80, 24, append(os.Environ(), "KAZE_TAB_ID=7"), "")
	if err != nil { t.Fatal(err) }
	defer s.Close()
	readUntil(t, s, "7", 5*time.Second)
}
func TestWaitReturnsAfterExit(t *testing.T) { /* start a command that exits 3; Wait returns 3 within 5s */ }
func TestDefaultShellIsNotEmpty(t *testing.T) { if len(ptyx.DefaultShell()) == 0 { t.Fatal("empty") } }
```

- [ ] **Step 2: Run** `go test ./internal/ptyx/...`. Expected: FAIL.
- [ ] **Step 3: Implement** on top of `github.com/aymanbagabas/go-pty`. All library calls stay inside this package; nothing else imports `go-pty`.
- [ ] **Step 4: Run** `go test ./internal/ptyx/...`. Expected: PASS on Windows.
- [ ] **Step 5: Commit** `feat(ptyx): Start shells in a pseudo-console on Windows and macOS`

---

### Task 9: Terminal session (PTY + parser + screen)

**Files:**
- Create: `internal/term/session.go`
- Test: `internal/term/session_test.go`

**Interfaces:**
- Consumes: `ptyx.Session`, `vt.Parser`, `Applier`, `screen.Screen`.
- Produces:

```go
type Session struct{ /* ... */ }
func NewSession(p ptyx.Session, cols, rows int, onChange func()) *Session
func (s *Session) Lock()          // renderer holds this while reading the screen
func (s *Session) Unlock()
func (s *Session) Screen() *screen.Screen // only while locked
func (s *Session) Applier() *Applier      // only while locked
func (s *Session) Write(p []byte)         // non-blocking: queued to a writer goroutine
func (s *Session) Resize(cols, rows int)
func (s *Session) Exited() (bool, int)
func (s *Session) Close()
```

`onChange` is called (from the reader goroutine) after each batch of output is applied; the UI uses it to request a frame. When the process exits, the session prints `\r\n[process exited <code>]` to the screen and calls `onChange`.

- [ ] **Step 1: Write the failing tests** with a fake `ptyx.Session` (an `io.Pipe` pair plus recorded `Resize` calls):

```go
func TestOutputReachesScreenAndCallsOnChange(t *testing.T) { /* fake writes "hi"; onChange fires; row 0 == "hi" */ }
func TestRepliesAreWrittenBackToPTY(t *testing.T) { /* fake writes "\x1b[6n"; fake receives "\x1b[1;1R" */ }
func TestResizeDuringStreamingDoesNotPanic(t *testing.T) {
	// goroutine writes 10,000 lines of mixed ASCII/Japanese to the fake;
	// main goroutine calls Resize with random sizes between 1x1 and 300x100 200 times;
	// assert no panic, final cursor within bounds, fake recorded the last Resize.
}
func TestProcessExitShowsMarkerAndKeepsSessionReadable(t *testing.T) {
	// fake closes with exit code 0; screen shows "[process exited 0]"; Exited() == (true, 0); Write is a no-op.
}
func TestOtherSessionsUnaffectedByOneExiting(t *testing.T) { /* two sessions; one exits; the other still applies output */ }
```

- [ ] **Step 2: Run** `go test -race ./internal/term/...`. Expected: FAIL.
- [ ] **Step 3: Implement.** Reader goroutine reads up to 64KB per read, then applies under the lock. Writes go through a buffered channel to a writer goroutine so a slow PTY never blocks the UI. `Resize` clamps to at least 1x1, resizes the screen (truncating or padding, no reflow) under the lock, then calls `ptyx.Session.Resize`.
- [ ] **Step 4: Run** `go test -race ./internal/term/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(term): Connect the shell, parser and screen in one session`

---

### Task 10: Gio renderer (dirty lines, glyph cache, frame pacing)

**Files:**
- Create: `internal/render/renderer.go`, `internal/render/glyphs.go`, `internal/render/fonts.go`, `internal/render/palette.go`
- Create: `assets/fonts/` (bundled monospace font + CJK fallback, both under open licenses, e.g. JetBrains Mono and Noto Sans Mono CJK JP subset; license files included)
- Test: `internal/render/renderer_test.go`

**Interfaces:**
- Consumes: `term.Session`, `screen.Screen`, `Modes`, `SyncSince`, the winning mode recorded in `docs/spike-results.md` by Task 0b.
- Produces:

```go
type Renderer struct{ /* ... */ }
func NewRenderer(fontSize float32, palette Palette) *Renderer
func (r *Renderer) CellSize() (w, h int) // pixels, for computing cols/rows from window size
func (r *Renderer) Layout(gtx layout.Context, s *term.Session, sel *Selection) layout.Dimensions
func (r *Renderer) Stats() Stats // FramesDrawn, RowsRedrawn, GlyphsCached; used by tests and the bench harness
type Palette struct { FG, BG, Cursor, Selection color.NRGBA; ANSI [16]color.NRGBA }
func DefaultPalette() Palette
```

`Selection` is defined in Task 12; until then pass `nil`.

- [ ] **Step 1: Write the failing tests** (Gio supports headless rendering via `gioui.org/gpu/headless`; if that is unavailable on the CI machine, these tests use `Stats()` with a no-op op list):

```go
func TestOnlyDirtyRowsAreRedrawn(t *testing.T) {
	// 80x24 session, first frame draws 24 rows; write one char; second frame: Stats().RowsRedrawn delta == 1
}
func TestNoChangeMeansNoRowsRedrawn(t *testing.T) { /* two frames with no output: second redraws 0 rows */ }
func TestGlyphRasterizedOnce(t *testing.T) { /* print 'a' 1000 times across frames: GlyphsCached for 'a' stays 1 */ }
func TestSynchronizedOutputHoldsFrame(t *testing.T) {
	// set mode 2026, write output: frame redraws 0 rows; after 150ms with mode still set, frame draws (timeout)
	// reset mode: next frame draws the dirty rows
}
func TestJapaneseUsesFallbackFont(t *testing.T) { /* glyph for '日' comes from the CJK face, not .notdef */ }
```

- [ ] **Step 2: Run** `go test ./internal/render/...`. Expected: FAIL.
- [ ] **Step 3: Implement** using Task 0b's winning drawing method; only dirty rows are redrawn (re-recorded ops for the op-based modes, re-rasterized rows for the image mode), clean rows reuse what was drawn before. Inverse, bold (bright on the 8 basic colors), underline, strike and dim are drawn here. Cursor: block when focused, outline when not, hidden when `CursorVisible` is false. Synchronized output: skip re-recording while `Synchronized` is set, up to 150ms after `SyncSince()`.
- [ ] **Step 4: Run** `go test ./internal/render/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(render): Draw only changed rows with a glyph cache`

---

### Task 11: Keyboard, mouse wheel and paste input

**Files:**
- Create: `internal/input/keys.go`, `internal/input/paste.go`, `internal/input/mouse.go`
- Test: `internal/input/keys_test.go`, `internal/input/paste_test.go`, `internal/input/mouse_test.go`

**Interfaces:**
- Consumes: `term.Modes`.
- Produces:

```go
func EncodeKey(name key.Name, mods key.Modifiers, m term.Modes) []byte   // nil when kaze handles the key itself
func EncodeText(s string) []byte
func EncodePaste(s string, m term.Modes) []byte
func EncodeWheel(lines int, col, row int, m term.Modes) []byte           // nil when not in mouse mode: caller scrolls the view
func EncodeMouse(button int, press bool, mods key.Modifiers, col, row int, motion bool, m term.Modes) []byte // nil when the mode doesn't report this event
```

- [ ] **Step 1: Write the failing tests**

```go
func TestArrowsNormalAndAppMode(t *testing.T) {
	if got := input.EncodeKey(key.NameUpArrow, 0, term.Modes{}); string(got) != "\x1b[A" { t.Fatalf("%q", got) }
	if got := input.EncodeKey(key.NameUpArrow, 0, term.Modes{AppCursorKeys: true}); string(got) != "\x1bOA" { t.Fatalf("%q", got) }
}
func TestCtrlLetters(t *testing.T) {
	if got := input.EncodeKey("C", key.ModCtrl, term.Modes{}); string(got) != "\x03" { t.Fatalf("%q", got) }
}
func TestAltPrefixesEscape(t *testing.T) {
	if got := input.EncodeKey("B", key.ModAlt, term.Modes{}); string(got) != "\x1bb" { t.Fatalf("%q", got) }
}
func TestEnterTabBackspaceEscape(t *testing.T) { /* "\r", "\t", "\x7f", "\x1b"; Shift+Tab -> "\x1b[Z" */ }
func TestFunctionAndNavKeys(t *testing.T) { /* F1 "\x1bOP", F5 "\x1b[15~", Home "\x1b[H", PageUp "\x1b[5~", Delete "\x1b[3~", Ctrl+Left "\x1b[1;5D" */ }
func TestBracketedPasteWraps(t *testing.T) {
	got := input.EncodePaste("hi", term.Modes{BracketedPaste: true})
	if string(got) != "\x1b[200~hi\x1b[201~" { t.Fatalf("%q", got) }
}
func TestPasteNormalizesNewlinesAndStripsEndMarker(t *testing.T) {
	got := input.EncodePaste("a\r\nb\x1b[201~c", term.Modes{BracketedPaste: true})
	if string(got) != "\x1b[200~a\rbc\x1b[201~" { t.Fatalf("%q", got) }
}
func TestHugePasteIsWrittenWithoutBlocking(t *testing.T) {
	// 1MB paste through term.Session.Write to a fake PTY that reads slowly: Write returns in under 5ms
}
func TestWheelInSGRMouseMode(t *testing.T) {
	got := input.EncodeWheel(1, 4, 2, term.Modes{MouseButton: true, MouseSGR: true})
	if string(got) != "\x1b[<64;5;3M" { t.Fatalf("%q", got) }
}
func TestMouseClickSGRPressAndRelease(t *testing.T) {
	m := term.Modes{MouseButton: true, MouseSGR: true}
	if got := input.EncodeMouse(0, true, 0, 9, 4, false, m); string(got) != "\x1b[<0;10;5M" { t.Fatalf("%q", got) }
	if got := input.EncodeMouse(0, false, 0, 9, 4, false, m); string(got) != "\x1b[<0;10;5m" { t.Fatalf("%q", got) }
}
func TestMouseMotionOnlyReportedInMatchingMode(t *testing.T) {
	// motion with no button held: nil under MouseButton, reported under MouseAny
}
func TestMouseLegacyEncodingWithoutSGR(t *testing.T) { /* MouseX10 click at 0,0 -> "\x1b[M !!" */ }
```

- [ ] **Step 2: Run** `go test ./internal/input/...`. Expected: FAIL.
- [ ] **Step 3: Implement** xterm encodings. Platform shortcuts kaze keeps for itself (copy, paste, new/close/next tab) return nil; those bindings come from config (Task 14). Copy is Ctrl+Shift+C / Cmd+C, paste Ctrl+Shift+V / Cmd+V, so plain Ctrl+C always reaches the shell.
- [ ] **Step 4: Run** `go test ./internal/input/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(input): Encode keys, paste and mouse wheel for the shell`

---

### Task 12: Mouse selection and copy

**Files:**
- Create: `internal/render/selection.go`
- Test: `internal/render/selection_test.go`

**Interfaces:**
- Produces:

```go
type Point struct{ Col, Row int } // Row counts from the oldest scrollback line, so selections survive scrolling
type Selection struct{ Start, End Point; Active bool }
func (s Selection) Contains(p Point) bool
func SelectedText(scr *screen.Screen, sel Selection) string
func WordAt(scr *screen.Screen, p Point) Selection   // double-click
func LineAt(scr *screen.Screen, p Point) Selection   // triple-click
```

- [ ] **Step 1: Write the failing tests**

```go
func TestSelectedTextAcrossLinesTrimsTrailingSpaces(t *testing.T) { /* "ab  " / "cd": text == "ab\ncd" */ }
func TestWideCharIsSelectedWhole(t *testing.T) { /* select only the continuation half of '日': text == "日" */ }
func TestWordAtStopsAtSpaces(t *testing.T) { /* "git status here": WordAt col 5 -> "status" */ }
func TestSelectionIncludesScrollback(t *testing.T) { /* select from a scrollback line into the screen */ }
```

- [ ] **Step 2: Run** `go test ./internal/render/...`. Expected: FAIL.
- [ ] **Step 3: Implement.** Drag selects; Shift+drag selects even when the app has mouse mode on. Copy writes to the system clipboard through Gio's clipboard op. Rows inside the selection are marked dirty when the selection changes.
- [ ] **Step 4: Run** `go test ./internal/render/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(render): Select text with the mouse and copy it`

---

### Task 13: Tabs and the main window

**Files:**
- Create: `internal/tabs/manager.go`, `internal/ui/window.go`, `internal/ui/tabbar.go`, `cmd/kaze/main.go`
- Test: `internal/tabs/manager_test.go`

**Interfaces:**
- Consumes: `ptyx.Start`, `ptyx.DefaultShell`, `term.NewSession`, `render.Renderer`, `input.*`, `config.Config` (Task 14; until it lands, use hard-coded defaults with the same field names).
- Produces (Plans 2 and 3 build on these):

```go
type TabID int
type Tab struct { ID TabID; Session *term.Session; Title string; Exited bool }
type Manager struct{ /* ... */ }
func NewManager(start func(env []string) (ptyx.Session, error), cols, rows int, onChange func()) *Manager
func (m *Manager) New() (*Tab, error)       // sets KAZE_TAB_ID=<id> in the shell env
func (m *Manager) Close(id TabID)
func (m *Manager) Active() *Tab
func (m *Manager) SetActive(id TabID)
func (m *Manager) Next(); func (m *Manager) Prev()
func (m *Manager) Tabs() []*Tab             // display order
func (m *Manager) ResizeAll(cols, rows int)
```

- [ ] **Step 1: Write the failing tests** (fake `start` func):

```go
func TestNewTabSetsTabIDEnv(t *testing.T) { /* start func records env; contains "KAZE_TAB_ID=1" then "KAZE_TAB_ID=2" */ }
func TestClosingActiveTabActivatesNeighbor(t *testing.T) { /* tabs 1,2,3 active 2; Close(2) -> active 3; Close(3) -> active 1 */ }
func TestTabIDsAreNeverReused(t *testing.T) { /* New, Close, New: second ID is 2 */ }
func TestTitleFollowsOSC(t *testing.T) { /* session sets OSC title "claude": Tab.Title == "claude" */ }
func TestResizeAllResizesEveryPTY(t *testing.T) {}
func TestClosingLastTabClosesWindow(t *testing.T) { /* Manager reports empty; window code quits */ }
```

- [ ] **Step 2: Run** `go test ./internal/tabs/...`. Expected: FAIL.
- [ ] **Step 3: Implement** the manager, then the Gio window: tab bar on top (title, exited marker, click to switch, middle-click or × to close, + to open), the active tab's renderer below. Window resize computes cols/rows from `Renderer.CellSize()` and calls `ResizeAll`. Mouse wheel scrolls the view (`SetViewOffset`) unless the app is in mouse mode. Typing snaps the view back to the bottom. Only the active tab is drawn; background tabs keep applying output but request no frames. Default shortcuts: new tab Ctrl+Shift+T / Cmd+T, close Ctrl+Shift+W / Cmd+W, next/prev Ctrl+Tab / Ctrl+Shift+Tab.
- [ ] **Step 4: Run** `go test ./internal/tabs/...` then `go run ./cmd/kaze` and check by hand: 4 tabs open, pwsh works in each, `claude` starts and streams cleanly in one tab, Japanese text from `Get-Content` of a UTF-8 file displays correctly.
- [ ] **Step 5: Commit** `feat(ui): Add tabs and the main window`

---

### Task 14: Config file

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:

```go
type Config struct {
	FontSize       float32           `toml:"font_size"`        // default 14
	FontFamily     string            `toml:"font_family"`      // default "" = bundled font
	Shell          []string          `toml:"shell"`            // default ptyx.DefaultShell()
	ScrollbackLines int              `toml:"scrollback_lines"` // default 10000, max 100000
	Colors         map[string]string `toml:"colors"`           // "fg", "bg", "cursor", "selection", "color0".."color15": "#rrggbb"
	Keys           map[string]string `toml:"keys"`             // action -> shortcut, e.g. "new_tab" = "ctrl+shift+t"
}
func Path() string                     // <os.UserConfigDir()>/kaze/config.toml
func Load(path string) (Config, []string, error) // returns warnings for bad values instead of failing
func Default() Config
```

- [ ] **Step 1: Write the failing tests**

```go
func TestMissingFileGivesDefaults(t *testing.T) {}
func TestBadValuesFallBackWithWarning(t *testing.T) { /* font_size = -3, color "#zzz": defaults used, 2 warnings */ }
func TestScrollbackIsCapped(t *testing.T) { /* 5_000_000 -> 100000 with a warning */ }
func TestPartialFileKeepsOtherDefaults(t *testing.T) {}
```

- [ ] **Step 2: Run** `go test ./internal/config/...`. Expected: FAIL.
- [ ] **Step 3: Implement** with `github.com/BurntSushi/toml`. Wire into `cmd/kaze/main.go`; warnings are printed into the first tab's screen as a dim line so they are visible without a settings UI. Config is read at startup only (no live reload in v1).
- [ ] **Step 4: Run** `go test ./internal/config/...`. Expected: PASS.
- [ ] **Step 5: Commit** `feat(config): Load settings from config.toml`

---

### Task 15: Benchmark harness and first milestone run

**Files:**
- Create: `bench/run.ps1`, `bench/run.sh`, `bench/flood/main.go`, `bench/README.md`
- Create: `docs/benchmarks/2026-MM-DD-milestone-1.md` (date of the run)

**Interfaces:**
- Consumes: `render.Stats`, the `kaze` binary, Warp installed on the same machine.

- [ ] **Step 1: Write the flood tool** `bench/flood`: prints N lines of mixed ASCII/Japanese/SGR colors as fast as possible (`-lines 10000`), and a `-claude` mode that replays a captured Claude Code session (from Task 7's capture tool) at original speed.
- [ ] **Step 2: Write the harness** measuring, for kaze and Warp:
  - launch to first prompt: time from process start until the window's first frame with the prompt (kaze: `KAZE_BENCH=1` env makes kaze log the time of the first frame containing non-blank text; Warp: screen-capture timing),
  - memory: private bytes of the terminal process only, idle 30s with 4 tabs,
  - idle CPU: average over 30s with 4 idle tabs,
  - scroll: time to page through 10,000 lines of scrollback and dropped frames (`render.Stats`),
  - keypress latency: software screen-capture timing (send a key with `SendInput` on Windows, detect the glyph change by polling the window region), 200 samples, p50/p99.
- [ ] **Step 3: Run on Windows** and write `docs/benchmarks/<date>-milestone-1.md` with a table: goal, kaze, met yes/no, and Warp for reference only (spec section 5 and 10.3). Each goal missed is listed with what was measured.
- [ ] **Step 4: Commit** `chore(bench): Add benchmark harness and first milestone results`

The macOS run of the same harness is done on Patrick's MacBook as part of the milestone (spec 10.3).
