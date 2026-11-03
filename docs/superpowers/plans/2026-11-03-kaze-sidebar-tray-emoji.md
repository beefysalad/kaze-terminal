# Sidebar Agent Tray, Project Rail, Title Symbols and Colour Emoji Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the title bar's empty box (#124), give the sidebar a slim, safe-click agent tray and a project rail (#125), and draw emoji in colour on Windows and macOS (#127), in one release (0.25).

**Architecture:** #124 is one shared status-mark helper in `internal/ui` plus a sans-text rule that drops runes the UI face would draw as a box. #125 replaces `Tracker.Tree`'s sidebar use with a new `Tracker.Tray` (state order, cap, hidden counts), adds per-session tray state (expanded, folded) to the retained sidebar cache, and redraws the agent lines, chip, footer, fold control, tooltip and project rail inside the existing cached `layoutSidebar` ops. #127 adds a small bounds-checked `internal/colorfont` package (table directory, COLR v0, CPAL, sbix, cmap) and an emoji face in `internal/render` whose glyphs are cached once as an RGBA image beside today's alpha masks and blended over the cell background.

**Tech Stack:** Go 1.27, Gio v0.10.3, `golang.org/x/image` v0.47.0 (`font/sfnt`, `vector`, `draw`), `image/png`.

**Spec:** `docs/superpowers/specs/2026-11-03-kaze-sidebar-tray-emoji-design.md` (look: `docs/design/sidebar-agents-mockup.html`, column "Combined: B + chip" and "2. Rail")

## Global Constraints

- Strict TDD: every test is seen failing before its code exists. RED tests are bounded: fixed sizes, no unbounded loops, `time.After` on any wait, fuzz seeds run as plain unit tests (no `-fuzz` in CI).
- No global keyboard or mouse input. UI tests drive `input.Router` queues (`sideClick`, `pointer.Event`), never SendInput.
- **Running tests on Windows (Smart App Control):** create the temp dir once (`mkdir -p /c/Users/john/Desktop/gotmp`), then every `go test` in this plan means
  `GOTMPDIR='C:\Users\john\Desktop\gotmp' go test -ldflags "-X main.r=$RANDOM$RANDOM" <args>` (Bash) or
  `$env:GOTMPDIR='C:\Users\john\Desktop\gotmp'; go test -ldflags "-X main.r=$(Get-Random)" <args>` (PowerShell).
  If "An Application Control policy has blocked this file" appears, rerun with a new random value. Never turn Smart App Control off.
- Zero allocations on unchanged sidebar frames: `TestSidebarCache*` keep passing; tray, chip, fold, tooltip and rail live inside the cached ops and `sideDrawKey`.
- No new timers or idle wakes: hover and fold redraw only through `sideRedraw()` on a change; no `op.InvalidateCmd` from the tray or tooltip.
- Emoji colour is decided once per glyph and cached; frames whose glyphs are cached cost the same as today. Glyph cache cap stays `glyphCap = 4096` glyphs.
- Malformed font tables never panic: colour is disabled for that font and the monochrome path is used. sbix PNG decode is limited by `png.DecodeConfig` to the strike's size.
- Tray numbers (spec 3): line 22 dp, gap above 3 dp, tree line 30 dp from the sidebar's left edge, 3 lines collapsed, 8 lines expanded, footer is not one of the 8.
- Sidebar text contrast ≥ 4.5:1 on its background in all five themes (`theme.All()`).
- U+FE0F, ZWJ sequences, skin tones, flags, COLRv1 and Linux colour emoji are out of scope.
- Commits: `type(scope): Verb ...`, **no AI attribution lines** (CLAUDE.md overrides any default). Work on `feat/sidebar-tray-emoji`; never push to `main`; the owner merges; one release 0.25 afterwards (separate release PR, not in this plan).

## Review Focus

1. **A title that is only a status mark** (`✳`, `◐`, a braille frame): the row and crumb fall back to the folder name, never an empty title or the bare symbol. (Task 1 test `TestRuledStatusOnlyTitleFallsBack`.)
2. **A shell title starting with ASCII punctuation** (`~/code/kaze`, `+x`): kept intact; only non-ASCII status symbols are stripped. (Task 1 test `TestDropStatusMarkKeepsASCII`.)
3. **A truncated or hostile emoji font** (COLR offsets past the table, layer index past the layer array, unsorted base records, PNG claiming 60000×60000): no panic, no big allocation, monochrome result. (Task 3 `TestParseCOLRRejects`, Task 4 `TestSbixPNGTooLarge`, Task 6 `TestBrokenColourTablesDrawMonochrome`.)
4. **Tray state outliving its session**: a closed session's expanded/fold entries are pruned, so a reused tab id starts open and collapsed. (Task 9 test `TestTrayStatePrunedWithSession`.)
5. **Two quick single clicks on different agent lines** are not a double-click (different keys), and two clicks on one line far apart in time are not either; neither opens a panel. (Task 12 test `TestAgentLineDoubleClickNeedsSameLine`.)

---

### Task 1: One status-mark helper (#124)

**Files:**
- Create: `internal/ui/statusmark.go`, `internal/ui/statusmark_test.go`
- Modify: `internal/ui/resources.go:157-162` (remove `dropStatusMark` there), `internal/ui/sidebar_model.go:345-350` (`ruled`)

**Interfaces:**
- Produces:
  - `func isStatusRune(r rune) bool`: non-ASCII and (`unicode.In(r, unicode.So, unicode.Sm)` or in U+2300–23FF, U+25A0–25FF, U+2700–27BF, U+2800–28FF). `✳ ✻ ✽ ⏺` are covered by these.
  - `func dropStatusMark(s string) string`: strips a leading run of `unicode.IsSpace` or `isStatusRune` runes; if nothing but spaces would remain, returns `s` unchanged.
  - `func onlyStatus(s string) bool`: true when `s` has no rune that is neither space nor status rune.

- [ ] **Step 1: Write the failing tests** in `statusmark_test.go`:
  - `TestDropStatusMark`: table `{"◐ Epic","Epic"}, {"⠂ Epic","Epic"}, {"✳ Epic","Epic"}, {"✳ ◐ Epic","Epic"}, {"◓◑ Epic V3-3347 tickets","Epic V3-3347 tickets"}, {"Epic","Epic"}, {"✳ ◐","✳ ◐"}, {"",""}`.
  - `TestDropStatusMarkKeepsASCII`: `"~/code/kaze"`, `"+x"`, `"<draft>"` come back unchanged.
  - `TestRuledStatusOnlyTitleFallsBack`: with the existing `sideFixture()`/`sideIn` helpers, an agent session (`agentState` with `claude: true`) titled `"✳ ◐"` in folder `/code/repo/api` gets row title `api` (the folder rule), and titled `"◐ Epic"` gets `Epic`.
  - `TestResourceNameDropsSpinner`: with `win, _ := sideWindow(t, true)`, `win.tabResourceName(&tabs.Tab{Title: "◐ Epic"})` == `"Epic"` (today `◐`, a So rune, is already stripped here; `"⌘ Epic"` (So) and `"∑ Epic"` (Sm) must be too).
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestDropStatusMark|TestRuledStatusOnly|TestResourceNameDrops'` → FAIL (`ruled` keeps `◐`; Sm runes survive `dropStatusMark`).
- [ ] **Step 3: Implement** the three functions; `ruled` becomes `title := dropStatusMark(t.Title); if !onlyStatus(title) && !looksLikePath(title) { return title }`. The crumb needs no change: `crumb()` takes its title from `sessionName`, which uses `ruled`.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `fix(ui): Strip any leading status symbol from session titles`

### Task 2: No missing-glyph boxes in UI-font text (#124)

**Files:**
- Modify: `internal/ui/tabbar.go` (`newChrome`, `fit`, `sans`), `internal/ui/titlebar_draw.go` (`fitSideW` already calls `sans`)
- Test: `internal/ui/tabfonts_test.go`

**Interfaces:**
- Produces:
  - `func (c *chrome) fitSans(s string) string`: `c.fit(s)` and then drops every non-ASCII rune with `unicode.In(r, unicode.So, unicode.Sm)` that the bundled Manrope Regular does not map (`sfnt` `GlyphIndex == 0`, font parsed once from `fonts.SansRegular`; answers memoized in a `map[rune]bool` cleared at `maxCovered`).
  - `sans` calls `fitSans` instead of `fit`.
  - test helper `func shapedNotdef(c *chrome, s string, f font.Font) bool` in the test file: lays `s` out with `c.shaper.LayoutString` and reports whether any glyph's font GID is 0, decoding `text.Glyph.ID` the way `gioui.org/text/shaper.go` `splitGlyphID` does (the GID is the low `gidbits = 64 - facebits - sizebits` bits; copy those two constants from that file, they are unexported).

- [ ] **Step 1: Write the failing test** `TestUIFontsNeverDrawBoxes`: for each rune in `◐ ◓ ⠂ ✳ ⏺ ⎇ ⌄`, for both `c.sans` (`font.Font{Typeface: sansFace}`, via `fitSans`) and `c.label` (`chromeFace`, via `fit`): the rune is either removed by the fit function or `shapedNotdef(c, fitted, face) == false`. Also `TestFitSansKeepsJapanese`: `c.fitSans("日本 Epic") == "日本 Epic"`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestUIFontsNeverDrawBoxes|TestFitSansKeepsJapanese'` → FAIL for `◐` under `sans` (the box seen in the title bar). Log which face Gio picked so the commit message can say why.
- [ ] **Step 3: Implement** `fitSans` as above.
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (title bar, settings and crumb tests included).
- [ ] **Step 5: Commit** `fix(ui): Drop symbols the UI font would draw as an empty box`

### Task 3: Colour font tables: directory, maxp, COLR v0, CPAL (#127)

**Files:**
- Create: `internal/colorfont/tables.go`, `colr.go`, `cpal.go`, `colorfont_test.go`, `testfont_test.go` (builders for synthetic tables)

**Interfaces:**
- Produces (package `colorfont`, no imports beyond the standard library):
  - `type Record struct{ Offset, Length uint32 }`; `type Directory map[string]Record`.
  - `func ReadDirectory(r io.ReaderAt, size int64, font int) (Directory, error)`: handles a plain sfnt and a `ttcf` collection (font index into its offsets); every offset+length must lie inside `size`; at most 64 tables.
  - `func (d Directory) Table(r io.ReaderAt, tag string, max int) ([]byte, error)`: `ErrMissing` when absent, `ErrTooBig` when `Length > max`.
  - `func NumGlyphs(maxp []byte) (int, error)`.
  - `type Layer struct{ Glyph, Palette uint16 }`; `const TextColor = 0xFFFF`.
  - `type COLR struct{ /* sorted base records, layer records */ }`; `func ParseCOLR(b []byte, numGlyphs int) (*COLR, error)` reads only the v0 header fields (works for v1 tables too); `func (c *COLR) Layers(gid uint16) []Layer` (binary search; returns a sub-slice, no allocation; nil when absent).
  - `type Palette []color.NRGBA`; `func ParseCPAL(b []byte) (Palette, error)` returns palette 0 (CPAL stores BGRA).
  - Errors: `ErrMissing`, `ErrTooBig`, `ErrMalformed`.

- [ ] **Step 1: Write the failing tests**:
  - `TestReadDirectory` (synthetic sfnt with `maxp`, `COLR`, `CPAL`; and the same wrapped in a `ttcf` header with two fonts, index 1).
  - `TestParseCOLR`: base records for gids 5 and 9, layers `{10,0},{11,1}` and `{12,0xFFFF}`; `Layers(5)` == `[{10 0} {11 1}]`, `Layers(9)` == `[{12 65535}]`, `Layers(6)` == nil; `testing.AllocsPerRun(100, func(){ c.Layers(5) }) == 0`.
  - `TestParseCPAL`: two entries BGRA `00 00 FF FF`, `00 FF 00 80` → `{R:255,A:255}`, `{G:255,A:128}`.
  - `TestParseCOLRRejects`: each of: table shorter than its header; base offset past the end; `firstLayer+numLayers > numLayerRecords`; layer glyph ≥ numGlyphs; base gids not ascending; numBaseGlyphRecords 0xFFFF with a 14-byte table → `ErrMalformed`, no panic.
  - `TestParseCPALRejects`: color record offset past the end; `numPaletteEntries` larger than the records present.
  - `FuzzParseCOLR` and `FuzzParseCPAL` with the valid tables as seeds: never panic (seeds run under plain `go test`).
  - `TestSegoeEmojiCOLR` (skip unless Windows and `%WINDIR%\Fonts\seguiemj.ttf` exists): directory has COLR and CPAL; `ParseCOLR` gives ≥ 3000 base records; `ParseCPAL` ≥ 1 colour.
- [ ] **Step 2: Run** `go test ./internal/colorfont/` → FAIL (package does not exist).
- [ ] **Step 3: Implement** the package. All reads go through one bounds-checked `u16(b, off)`/`u32(b, off)` pair returning `ok=false` instead of indexing out of range.
- [ ] **Step 4: Run** `go test ./internal/colorfont/` → PASS.
- [ ] **Step 5: Commit** `feat(colorfont): Read COLR and CPAL colour tables safely`

### Task 4: Colour font tables: sbix, cmap and bounded PNG (#127)

**Files:**
- Create: `internal/colorfont/sbix.go`, `cmap.go`, `sbix_test.go`

**Interfaces:**
- Produces:
  - `type Cmap struct{ /* format 4 and 12 subtables */ }`; `func ParseCmap(b []byte) (Cmap, error)` (prefers platform 3 encoding 10, then 0/4, then 3/1); `func (c Cmap) Glyph(r rune) uint16`. Used for fonts `sfnt` cannot parse (Apple Color Emoji may have no `glyf`).
  - `type Sbix struct{ /* strike ppems and offsets, numGlyphs */ }`; `func ParseSbix(r io.ReaderAt, rec Record, numGlyphs int) (*Sbix, error)` reads the header and strike offsets only (≤ 32 strikes).
  - `func (s *Sbix) Nearest(ppem int) (strike, strikePPEM int)`.
  - `func (s *Sbix) Has(r io.ReaderAt, strike int, gid uint16) bool` (non-empty `png ` record).
  - `func (s *Sbix) PNG(r io.ReaderAt, strike int, gid uint16) ([]byte, error)`: graphic type must be `png `; data ≤ `maxGlyphData = 512 << 10`; follows one `dupe` hop at most.
  - `func DecodePNG(data []byte, maxSide int) (image.Image, error)`: `png.DecodeConfig` first; `ErrTooBig` when either side > `maxSide`.

- [ ] **Step 1: Write the failing tests**: `TestParseCmapFormat12` (U+1F504 → 77, U+2705 → 12, U+0041 → 0), `TestParseCmapFormat4`, `TestSbixNearestStrike` (strikes 20, 32, 40, 64, 96, 160: `Nearest(19)`→20, `Nearest(36)`→32 or 40 by distance with ties to the larger, `Nearest(500)`→160), `TestSbixPNG` (a synthetic sbix holding a real 4×4 PNG for gid 3 and a `dupe` from gid 4 to 3: both decode to 4×4; gid 5 empty → `Has == false`), `TestSbixPNGTooLarge` (a PNG header claiming 60000×60000: `DecodePNG(data, 256)` → `ErrTooBig` before any pixel allocation; a data length field of 2 GiB → `ErrTooBig`), `TestSbixRejects` (strike offset past the table; offset table not ascending; a `dupe` cycle), `FuzzParseSbix`, `FuzzParseCmap`.
- [ ] **Step 2: Run** `go test ./internal/colorfont/ -run 'Cmap|Sbix'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/colorfont/` → PASS.
- [ ] **Step 5: Commit** `feat(colorfont): Read sbix bitmaps and cmap with size limits`

### Task 5: Emoji face and fallback order (#127)

**Files:**
- Create: `internal/render/emoji.go`, `internal/render/emoji_tables.go` (generated), `internal/render/gen_emoji.go` (`//go:build ignore`), `internal/render/emoji_test.go`
- Modify: `internal/render/fonts.go` (`faceID`, `fontSet`, `faceFor`, `drawFallback`), `internal/render/glyphs.go` (`get`), `fontpaths_windows.go`, `fontpaths_darwin.go`, `fontpaths_other.go`

**Interfaces:**
- Consumes: `colorfont.ReadDirectory`, `NumGlyphs`, `ParseCOLR`, `ParseCPAL`, `ParseSbix`, `ParseCmap` (Tasks 3–4).
- Produces:
  - `func isEmojiPresentation(r rune) bool` and `func isTextEmoji(r rune) bool` (Emoji=Yes, Emoji_Presentation=No, r > 0x7F), from range tables generated by `go run gen_emoji.go` out of Unicode 15.1 `emoji-data.txt` (URL and version in the generated header).
  - `func emojiFontPath() string`: Windows `%WINDIR%\Fonts\seguiemj.ttf`; darwin `/System/Library/Fonts/Apple Color Emoji.ttc`; others `""`.
  - `type emojiFont struct { sf *sfnt.Font; colr *colorfont.COLR; pal colorfont.Palette; sbix *colorfont.Sbix; cmap colorfont.Cmap; src io.ReaderAt }` with `func (e *emojiFont) glyph(r rune) uint16` (via `sf` when non-nil, else `cmap`) and `func (e *emojiFont) outline() bool` (`sf != nil`).
  - `type emojiSource struct { mu sync.Mutex; path string; done bool; f *emojiFont; open func(path string) *emojiFont }`; `var emojiFonts = &emojiSource{path: emojiFontPath()}`; `func (s *emojiSource) font() *emojiFont` loads at most once; `open == nil` means `loadEmojiFont`, which recovers from panics and leaves any table that fails to parse nil.
  - `faceEmoji` (colour when the font has colour for the glyph, else its outline) and `faceEmojiText` (always the outline, one colour), inserted before `faceSystem`.
  - `fontSet.emoji *emojiSource` (set from `emojiFonts` in `newFontSet`).
  - `func (fs *fontSet) drawOutline(faceAt func(px float64) xfont.Face, r rune, w int) *image.Alpha`: today's `drawFallback` body; `drawFallback(i, r, w)` calls it with `fs.sysFace`. `func (fs *fontSet) emojiFace(px float64) xfont.Face` caches faces of `emojiFont.sf` per px (cap `sysFaceCap`).

Order in `faceFor` (spec 7): emoji-presentation rune that the emoji font has and can draw (outline, or a non-empty sbix record in the nearest strike) → `faceEmoji`; then mono; CJK; text-default emoji that the emoji font has **and** `outline()` → `faceEmojiText`; then the system list unchanged (Segoe UI Emoji stays in the Windows list for other runes; update its comment); then `faceMissing`. This task draws `faceEmoji` from the outline too; colour arrives in Task 6.

- [ ] **Step 1: Write the failing tests**:
  - `TestEmojiTables`: `isEmojiPresentation` true for `✅ ❌ 🔄 🟢 🔴 🟡 ⏳ 🚀 📝 ⌚`, false for `⚠ ✔ ❤ © ⏺ A ✳`; `isTextEmoji` true for `⚠ ✔ ❤ ⏺ ©`, false for `✅ 🔄 # 7 ✳`.
  - `TestCheckMarkUsesEmojiFace` (skip unless Windows with seguiemj): `get('✅', false, true).face == faceEmoji` (today Segoe UI Symbol); `get('🔄', false, true).face == faceEmoji`; the mask is two cells wide and has ink.
  - `TestTextEmojiUsesEmojiOutline` (same skip): `get('⚠', false, false).face == faceEmojiText`; `get('✻', false, false).face` is unchanged (`faceSystem`).
  - `TestNoEmojiFontKeepsOldOrder`: with `emojiFonts.open` swapped (restore in `t.Cleanup`, new size so the shared cache is fresh) to return nil, `✅` resolves through the system list as before (`faceFor` never returns an emoji face).
  - Existing `TestFallbackDrawsClaudeSymbols`, `TestFallbackDrawsClaudeSymbolsOnMac` and `TestSystemFontParsedOnlyOnMiss` keep passing.
- [ ] **Step 2: Run** `go test ./internal/render/ -run 'Emoji|CheckMark|TextEmoji'` → FAIL.
- [ ] **Step 3: Implement** the tables, source, face IDs, order and the `get` branches (`faceEmoji`/`faceEmojiText` → `drawOutline(fs.emojiFace, r, w)`).
- [ ] **Step 4: Run** `go test ./internal/render/` → PASS.
- [ ] **Step 5: Commit** `feat(render): Send emoji to the emoji font before Segoe UI Symbol`

### Task 6: COLR colour glyphs in the glyph cache (#127, Windows)

**Files:**
- Modify: `internal/render/glyphs.go` (`glyph`, `get`), `internal/render/emoji.go`, `internal/render/renderer.go` (`drawCell`, new `blendColor`)
- Test: `internal/render/emoji_test.go`, `internal/render/renderer_test.go`

**Interfaces:**
- Consumes: `emojiFont` (Task 5), `COLR.Layers`, `Palette`, `colorfont.TextColor`.
- Produces:
  - `glyph.color *image.RGBA` (premultiplied, the cell span `w × cellH`); `mask` is nil when `color` is set; `empty` reflects the colour image's alpha.
  - `func (fs *fontSet) drawCOLR(e *emojiFont, gid uint16, layers []colorfont.Layer, w int) *image.RGBA`: nil on any error or panic (the caller then uses the outline).
  - `const textColorNeutral = 0x9a9a9a` (palette index 0xFFFF; builder's call, recorded in the audit).
  - `func blendColor(img *image.RGBA, x, y, w, h int, c *image.RGBA)`: premultiplied over: `dst = src + dst*(255-a)/255` per channel; skips `a == 0`.

Algorithm for `drawCOLR`: ppem starts at `fs.px`; take the union of the layers' `sfnt.GlyphBounds` at that ppem; if it exceeds `w × cellH`, scale ppem by `0.95 * min(w/gw, cellH/gh)` and recompute; centre the union horizontally and vertically in the span. For each layer: `sf.LoadGlyph(&buf, gid, ppem, nil)`, rasterize the segments with `vector.Rasterizer` (size `w × cellH`, translated by the centring offset) into one reused `*image.Alpha`, then `draw.DrawMask(dst, dst.Bounds(), image.NewUniform(col), image.Point{}, mask, image.Point{}, draw.Over)` with `col` = `pal[idx]` or `textColorNeutral` for 0xFFFF (or out-of-range indices).

In `drawCell`: after `fillRect(bg)`, `if g.color != nil { blendColor(...) } else if !g.empty { blendMask(...) }`; underline and strike still use `fg`.

- [ ] **Step 1: Write the failing tests**:
  - `TestCheckMarkIsColour` (Windows with seguiemj): `g := get('✅', false, true)`; `g.color != nil`, `g.color.Rect.Dx() == 2*cellW`, at least two distinct opaque colours among pixels with alpha 255, and one of them has `G > R && G > B` (the green box).
  - `TestColourGlyphIgnoresFG`: harness 10×2; print `\x1b[31m✅\x1b[0m` on row 0 and `✅` on row 1; every pixel of the two cell spans is equal (`pixelAt`).
  - `TestSelectionAndInverseShowBehindEmoji`: print `\x1b[7m✅\x1b[0m`; the span's corner pixel (transparent in the emoji) equals the inverse background (`p.FG`); with a `Selection` over the cell, it equals `p.Selection`.
  - `TestBrokenColourTablesDrawMonochrome`: `emojiFonts.open` returns the real font with `colr` replaced by `ParseCOLR` of a corrupt table (nil) → `get('✅')` has `color == nil` and an ink mask; and a font whose layer glyph fails `LoadGlyph` → `drawCOLR` returns nil, no panic.
  - `TestColourGlyphMemory`: 200 distinct emoji-presentation runes from the table at size 14 → `t.Logf` the total `len(color.Pix)` bytes (the audit's number) and assert < 4 MiB; `GlyphsCached` == 200.
  - `TestColourGlyphRasterizedOnce`: like `TestGlyphRasterizedOnce` with 1000 `✅`: one glyph cached, and a second frame with no change redraws 0 rows.
- [ ] **Step 2: Run** `go test ./internal/render/ -run 'Colour|BrokenColour|SelectionAndInverse'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/render/` → PASS.
- [ ] **Step 5: Commit** `feat(render): Draw Segoe UI Emoji in colour from its COLR layers`

### Task 7: sbix colour glyphs (#127, macOS)

**Files:**
- Modify: `internal/render/emoji.go`, `internal/render/glyphs.go`
- Test: `internal/render/emoji_test.go`, `internal/render/emoji_darwin_test.go`

**Interfaces:**
- Consumes: `Sbix.Nearest`, `Sbix.Has`, `Sbix.PNG`, `colorfont.DecodePNG` (Task 4).
- Produces: `func (fs *fontSet) drawSbix(e *emojiFont, gid uint16, w int) *image.RGBA`: strike nearest to `fs.cellH`; `DecodePNG(data, 2*strikePPEM)`; scale with `xdraw.CatmullRom.Scale` into the largest rectangle with the bitmap's aspect that fits `w × cellH`, centred; premultiplied RGBA. nil on any error. `get` tries `drawCOLR` when `e.colr` has layers, else `drawSbix` when `e.sbix != nil`, else the outline (and if there is no outline either, the glyph is blank, never .notdef).

- [ ] **Step 1: Write the failing tests**:
  - `TestSbixGlyphScaledIntoSpan` (all platforms): `emojiFonts.open` returns an `emojiFont` built from Task 4's synthetic sbix (a red 64×64 PNG for U+1F534 via a synthetic cmap, `sf == nil`); `get('🔴', false, true)` has `face == faceEmoji`, `color` of `2*cellW × cellH`, centre pixel red with alpha 255, and a corner pixel alpha 0 (aspect kept).
  - `TestSbixHugePNGDrawsBlank`: the PNG claims 60000×60000 → glyph has no colour and is blank; no panic.
  - `TestAppleColorEmojiIsColour` (darwin only, `emoji_darwin_test.go`; runs in macOS CI): `get('✅', false, true).color != nil` with ≥ 2 distinct opaque colours; `get('⏺', false, false)` still comes from the system list (unchanged, `face >= faceSystem`).
- [ ] **Step 2: Run** `go test ./internal/render/ -run 'Sbix'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/render/` → PASS here; the darwin test runs on the PR's macOS CI job.
- [ ] **Step 5: Commit** `feat(render): Draw Apple Color Emoji from its sbix bitmaps`

### Task 8: Tracker tray order and cap (#125)

**Files:**
- Modify: `internal/subagents/subagents.go`
- Test: `internal/subagents/subagents_test.go`

**Interfaces:**
- Produces:
  - `type TrayCounts struct{ Waiting, Running, Done int }` with `func (c TrayCounts) Total() int`.
  - `func (t *Tracker) Tray(tab, limit int) (shown []*Agent, hidden TrayCounts)`: the same members as `Tree` (`InTree`, and finished ones only while another runs), flat, ordered Waiting, then Running, then Done, newest `Started` first within each (ties: later in start order first); the first `limit` are `shown`, the rest counted in `hidden`. `Tree` stays for the dock's ↑/↓ and `waitingAgent`.

- [ ] **Step 1: Write the failing tests** `TestTrayOrderAndCap`: five agents started t0..t4 in tab 1: a0 running, a1 waiting, a2 done (a3 still running), a3 running, a4 waiting → `Tray(1, 3)` shows `a4, a1, a3` and hidden `{Waiting:0, Running:1, Done:1}`; `Tray(1, 8)` shows all five in order `a4 a1 a3 a0 a2`, hidden zero. `TestTrayDropsFinishedWhenNoneRun`: after every agent stops, `Tray(1, 3)` is empty (same rule as `TestFinishedLeaveTreeWhenNoneRun`).
- [ ] **Step 2: Run** `go test ./internal/subagents/ -run TestTray` → FAIL (undefined).
- [ ] **Step 3: Implement** with `slices.SortStableFunc`.
- [ ] **Step 4: Run** `go test ./internal/subagents/` → PASS.
- [ ] **Step 5: Commit** `feat(subagents): Order the sidebar tray waiting first and cap it`

### Task 9: Tray model, per-session state and texts (#125)

**Files:**
- Modify: `internal/ui/sidebar_model.go` (`sideRow`, `sideInput.Agents`, `buildSide`, `stateWord`), `internal/ui/sidebar_agents.go` (`sideAgents`, texts), `internal/ui/sidebar_window.go` (side state fields), `internal/ui/sidebar_cache.go` (`cachedSide` dirty check), `internal/ui/window.go:677` (prune next to `pruneDock`)
- Test: `internal/ui/sidebar_agents_test.go` (rewrite `TestBuildSideAddsAgents`), `internal/ui/sidebar_cache_test.go`

**Interfaces:**
- Consumes: `Tracker.Tray`, `TrayCounts` (Task 8).
- Produces:
  - `const trayCap, trayMax = 3, 8`.
  - `type sideTray struct { Shown []sideAgent; Hidden subagents.TrayCounts; Total, Waiting, Active int; Expanded, Folded bool }`.
  - `sideInput.Agents func(tabs.TabID) sideTray` (was three results).
  - `sideRow`: keep `Agents` (lines to draw, empty when folded) and `AgentCount` (active, for groups); replace `MoreAgents` with `Hidden subagents.TrayCounts`; add `TrayTotal, TrayWaiting int; TrayExpanded, TrayFolded bool`.
  - Side state (on `win.side`): `trayOpen, trayFolded map[tabs.TabID]bool` (expanded; folded). `cachedSide` marks dirty when either differs from its clone (`maps.Equal`, like `folded`). `func (win *window) pruneTray(list []*tabs.Tab)` deletes ids no longer open.
  - `func (win *window) sideAgents(id tabs.TabID) sideTray`: `limit = trayCap`, or `trayMax` when `trayOpen[id]`; folded → `Shown` nil, `Hidden` zero but `Total/Waiting` kept.
  - `func trayFold(total int, folded bool) string` → `"20 agents ⌄"` / `"20 agents ›"` / `"1 agent ⌄"` (caret constants `trayOpenCaret = "⌄"`, `trayShutCaret = "›"`).
  - `func trayChip(h subagents.TrayCounts) (lead, needs, rest string)` → `("+17 more", "", " · 15 running, 2 done ⌄")`; with waiting `("+5 more", " · 1 needs you", ", 3 running, 1 done ⌄")`, `"2 need you"` for more than one; zero parts are left out.
  - `func trayFooter(beyond int) (fewer, board string)` → `("Show fewer", "")`, or `("Show fewer", " · 13 more on the board")`.
  - `sideRow.stateWord()`: when `TrayTotal > 0` it returns `trayFold(...)` (the fold control replaces "needs you"/"done"/"N agents"; the row's dot keeps the mark); otherwise unchanged.

- [ ] **Step 1: Write the failing tests**:
  - `TestBuildSideAddsAgents` rewritten for `sideTray`: row 1 gets `Agents` 1, `Hidden{Running:2}`, `TrayTotal 3`, `AgentCount 3`; group `AgentCount` 4.
  - `TestTrayTexts`: the exact strings above, including `trayChip(TrayCounts{Waiting:2, Running:1})` == `("+3 more", " · 2 need you", ", 1 running ⌄")`.
  - `TestTrayCaretsDraw`: `render.BundledCovers` is true for every rune of `trayOpenCaret` and `trayShutCaret` (if `⌄` is not covered, switch the constant to `▾` and note it in the commit).
  - `TestTrayStateLimits`: 12 agents in tab a: default `len(Agents) == 3`, `Hidden.Total() == 9`; `trayOpen[a]` → 8 and 4; `trayFolded[a]` → 0 lines, `TrayTotal 12`.
  - `TestTrayStateRebuildsModel`: flipping `trayOpen` or `trayFolded` raises `win.side.cache.builds` by one; an unchanged frame does not.
  - `TestTrayStatePrunedWithSession`: set both maps for tab a, close it, run a frame: neither map has a.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestBuildSideAddsAgents|TestTray'` → FAIL.
- [ ] **Step 3: Implement**; update `agentHit`/`TestMoreAgentsOpensFilteredBoard` compile fixes only as far as `MoreAgents` is gone (drawing comes in Task 11).
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS (drawing still uses the old two-line layout reading `Agents` and a temporary `Hidden.Total()` for "+N more").
- [ ] **Step 5: Commit** `feat(ui): Keep per-session tray state for subagents`

### Task 10: Project rail and sidebar contrast (#125)

**Files:**
- Modify: `internal/ui/sidebar.go` (`sideGroupHeader`, `sideList`, `sideHits`, remove `activeProjectColors`/`activeProjectCountColor`), `internal/ui/tabbar.go` (`newChrome` faces, `labelW`), `internal/ui/sidebar_agents.go`
- Test: `internal/ui/active_project_test.go` (replace `TestActiveProjectHeaderTextContrast`), `internal/ui/sidebar_test.go`

**Interfaces:**
- Produces:
  - `type sideInk struct{ muted, needs, kind, chipLead, chipBG, tipFG, tipBG color.NRGBA }`; `func (c *chrome) sideInk() sideInk`: `muted = render.Readable(th.Muted, th.Chrome)`, `needs = render.Readable(th.Needs, th.Chrome)`, `kind = render.Readable(c.memberColor("magenta"), th.Chrome)`, `chipBG = mix(th.Chrome, th.Accent, 0.08)`, `chipLead = render.Readable(mix(th.Accent, th.ActiveFG, 0.35), chipBG)`, `tipBG = th.ToastBG`, `tipFG = render.Readable(th.ToastFG, th.ToastBG)`. Every `th.Muted` in `sidebar.go` and `sidebar_agents.go` becomes `ink.muted` (theme values stay; the nudge is local to the sidebar).
  - Chrome faces gain `{fonts.MonoBold, chromeFace, font.Bold}`; `func (c *chrome) labelW(gtx, s, size, col, w font.Weight) (op.CallOp, image.Point)`; `label` calls it with `font.Normal`; `fitSide` gains a weight through a private `fitSideWeight`.
  - `func (c *chrome) groupNameStyle(active bool) (color.NRGBA, font.Weight)` → `(th.ActiveFG, font.Bold)` when active, else `(th.ActiveFG, font.Normal)`.
  - `sideHits.rail image.Rectangle`: x from `h.list.Min.X + 2dp` to `+4dp`; y from the active group's header top to the bottom of its last row span (docs and tray included; header only when folded); intersected with `h.list`; drawn in `th.Accent` with radius 1 dp. Empty when no group is active or it is scrolled out.

- [ ] **Step 1: Write the failing tests**:
  - `TestSidebarTextContrast`: for each of `theme.All()`, `contrast(ink.muted, th.Chrome)`, `contrast(ink.needs, th.Chrome)`, `contrast(ink.kind, th.Chrome)`, `contrast(th.Folder, th.Chrome)`, `contrast(ink.chipLead, ink.chipBG)`, `contrast(ink.muted, ink.chipBG)`, `contrast(ink.needs, ink.chipBG)`, `contrast(ink.tipFG, ink.tipBG)` are all ≥ 4.5 (Sumi's raw Muted is 4.14 today; Washi's is lower).
  - `TestActiveProjectRail` (`sideDraw` with two groups, the second active with two rows, one having two tray lines): `h.rail.Min.Y == header top`, `h.rail.Max.Y == bottom of its last row span`, `h.rail.Dx() == 2dp`; folding it gives a rail exactly the header's height; `activeRepo ""` → `h.rail.Empty()`.
  - `TestActiveProjectNameStyle`: `groupNameStyle(true)` is `(ActiveFG, font.Bold)`, `groupNameStyle(false)` weight Normal, in all five themes.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestSidebarTextContrast|TestActiveProject'` → FAIL.
- [ ] **Step 3: Implement**; the active header gets no fill or outline; the count stays `ink.muted`, or `ink.needs` for "N waiting".
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS, including `TestSidebarCache*`.
- [ ] **Step 5: Commit** `feat(ui): Mark the project you are in with a rail, not a red box`

### Task 11: Draw the tray (#125)

**Files:**
- Modify: `internal/ui/sidebar_agents.go` (replace `agentLines`, `rowSpan`, constants), `internal/ui/sidebar.go` (`row` closure: fold control and its hit), `internal/ui/sidebar_window.go` (`revealScroll` already uses `rowSpan`)
- Test: `internal/ui/sidebar_agents_test.go` (replace `TestSidebarDrawsAgentLines`), `internal/ui/sidebar_cache_test.go`, `internal/ui/dock_entry_test.go` (`agentHit`), `internal/ui/board_test.go`

**Interfaces:**
- Consumes: `sideRow` tray fields, `trayChip`, `trayFooter`, `trayFold` (Task 9); `sideInk` (Task 10).
- Produces:
  - `const sideAgentH, sideTrayGap, sideTrayPad, sideChipH = 22, 3, 4, 22` (dp); `sideMoreH = 20` stays for the "finished worktrees" line only.
  - `func trayHeight(gtx layout.Context, r sideRow) int`: 0 with no lines and no chip/footer, else `gap + n*line + chip + pad`; `rowSpan` adds it.
  - `type agentHitKind uint8` with `hitAgentLine, hitAgentChip, hitAgentFewer, hitAgentBoard`.
  - `sideAgentHit{ key; tab; kind agentHitKind; open bool; rect, btn image.Rectangle; tip string }` (`more` removed; `btn` is the `›` box; `tip` is the line's `Action`).
  - `sideRowHit.fold image.Rectangle`: the fold control in the state slot, padded by `hitPad`.
  - `func (c *chrome) trayLines(gtx layout.Context, h *sideHits, r sideRow, row image.Rectangle, y int, open subagents.Key, ink sideInk)`.

Look (mockup `.tray`/`.trow`/`.morechip`): tree line 1 px in `th.Border` at `row.Min.X + 22dp` (30 dp from the sidebar edge) from the first line's top to the last line's middle, with a 7 dp tick to each line. Line: mark (`stateDot`, running ring) at tree + 10 dp, type 11 sp `ink.kind`, task 11 sp `th.Folder` (`ink.needs` waiting, `ink.muted` done), elapsed 10 sp `ink.muted`, then `›` 12 sp `ink.muted` centred in a 16 dp box at the right end (`btn` = that box plus 4 dp, full line height). Open line: `th.Active` fill with right corners 6 dp and a 2 dp `th.Accent` bar at its left. Chip and footer: a pill (radius 9 dp, `ink.chipBG`, 1 px `th.Border` outline) on its own 22 dp line, text 11 sp: chip `lead` in `ink.chipLead`, `needs` in `ink.needs`, `rest` in `ink.muted`; footer "Show fewer" (`hitAgentFewer`) and " · N more on the board" (`hitAgentBoard`, `ink.muted`) as two hits. Fold control: the row's state word drawn from `stateWord()`, in `th.Accent`, or `ink.needs` when `r.Mark == markNeeds` or (`TrayFolded` and `TrayWaiting > 0`).

- [ ] **Step 1: Write the failing tests**:
  - `TestSidebarDrawsTray` (replaces `TestSidebarDrawsAgentLines`): row with 3 lines and `Hidden{Running:15, Done:2}`; hits are 3 `hitAgentLine` each `22dp` tall, then one `hitAgentChip`; first line's top == row bottom + 3 dp; each line's `btn` lies inside its `rect` at the right edge; all hits lie between the two rows' rects (as in the old test).
  - `TestSidebarDrawsTrayFooter`: `TrayExpanded`, 8 lines, `Hidden.Total() 13` → 8 lines, then `hitAgentFewer` and `hitAgentBoard` on one line, board hit right of fewer; with `Hidden` zero → only `hitAgentFewer`.
  - `TestSidebarFoldedTrayHasNoLines`: `TrayFolded` → no agent hits; `h.rows[0].fold` non-empty; the row span equals `sideRowH`.
  - `TestRevealScrollCountsAgentLines` updated to the new heights.
  - `TestSidebarCacheTrayAllocatesNothing`: a session with 12 agents (via `startAgent`), one expanded session and one folded: after warm-up, `testing.AllocsPerRun(100, draw) == 0` and `builds`/`draws` unchanged.
  - `agentHit` matches `kind == hitAgentLine`; `TestMoreAgentsOpensFilteredBoard` builds its hit with `kind: hitAgentBoard`.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestSidebarDrawsTray|TestSidebarFolded|TestRevealScroll|TestSidebarCacheTray'` → FAIL.
- [ ] **Step 3: Implement** the drawing above. In `sidePress`, map the new kinds to today's behaviour so this task stays green on its own: `hitAgentBoard` → `openBoard`, `hitAgentLine` → `showAgent(tab, key, true)`; chip, fewer and fold are wired in Task 12.
- [ ] **Step 4: Run** the same command → PASS; `go test ./internal/ui/ -run TestSidebarCache` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Draw subagents as a slim tray under their session`

### Task 12: Safe click, `›`, double-click, chip, Show fewer, fold (#125)

**Files:**
- Modify: `internal/ui/sidebar_window.go` (`sidePress`, side state), `internal/ui/panes_agent.go` (unchanged `showAgent`; reuse)
- Test: `internal/ui/dock_entry_test.go`, `internal/ui/sidebar_tray_input_test.go` (new), `internal/ui/board_test.go`

**Interfaces:**
- Consumes: hit kinds and `sideRowHit.fold` (Task 11); `trayOpen`, `trayFolded` (Task 9); `showAgent(tab, k, toggle)`, `openBoard(tab)`, `floatMember`, `selectTab`.
- Produces:
  - `func (win *window) clickRow(id tabs.TabID)`: today's row default branch (a workspace member → `floatMember(id)`, else `selectTab(id)`); the row branch of `sidePress` calls it.
  - `type agentClick struct{ key subagents.Key; at time.Duration }`; `win.side.lastAgent agentClick`.
  - test helper `func sideClickAt(win *window, r *input.Router, p image.Point, b pointer.Buttons, at time.Duration)` (press with `Time: at`).

Behaviour in `sidePress`, agent hits (primary only): `hitAgentLine` → if `p.In(a.btn)`: `showAgent(a.tab, a.key, true)`; else if `lastAgent.key == a.key && at >= lastAgent.at && at-lastAgent.at < doubleClick`: `showAgent(a.tab, a.key, false)` (opens; a double-click never closes); else `clickRow(a.tab)` and remember `lastAgent`. `hitAgentChip` → `trayOpen[a.tab] = true`. `hitAgentFewer` → `delete(trayOpen, a.tab)`. `hitAgentBoard` → `openBoard(a.tab)`. Row hits: `primary && p.In(r.fold)` → flip `trayFolded[r.id]`, nothing selected. Any other press clears `lastAgent`.

- [ ] **Step 1: Write the failing tests** (two separate clicks use `sideClickAt` with times 0 and `2*doubleClick`):
  - `TestAgentLineShowsSession`: b shown, click a's agent line (away from `btn`) → `mgr.Active().ID == a.ID`, `dockOf(a.ID) == nil`.
  - `TestAgentButtonTogglesPanel` (replaces `TestSidebarAgentTogglesDock`): click `btn` → panel open on k; click `btn` again (later time) → closed.
  - `TestAgentLineDoubleClickOpensPanel`: two presses on the line at 0 and 100 ms → panel open on k; a third double-click keeps it open.
  - `TestAgentLineDoubleClickNeedsSameLine`: press line k at 0, line j at 100 ms → no panel; press line k at 0 and k at 2 s → no panel.
  - `TestAgentLineMatchesRowClick`: with `paneWindow`, clicking a's agent line and clicking a's row leave the same active tab, float set and dock state (both go through `clickRow`; members are not session rows, so `clickRow`'s member branch is reached only from `lineAt`'s member lines, as today).
  - `TestTrayChipExpandsAndFewerCollapses`: 12 agents; click chip → 8 lines and a fewer hit; click fewer → 3 lines and a chip.
  - `TestTrayBoardLinkOpensFilteredBoard` (rewritten `TestMoreAgentsOpensFilteredBoard`): `win.board.open && win.board.tab == id`.
  - `TestTrayFoldControl`: click `rowHit(...).fold` → `trayFolded[a]`, no tray hits, active tab unchanged; click again → open.
  - `TestToastGoOpensWaitingAgent` and `TestBoardCardOpensDock` unchanged and passing.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestAgentLine|TestAgentButton|TestTray|TestToastGo|TestBoardCard'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Make an agent line show its session and open the panel from ›`

### Task 13: Hover tooltip with the current step (#125)

**Files:**
- Modify: `internal/ui/sidebar.go` (`sideState.hoverAgent`, `layoutSidebar` draws the tip last, `sideHits.tip`), `internal/ui/sidebar_cache.go` (`sideDrawKey.hoverAgent`), `internal/ui/sidebar_window.go` (`sidePointer`, `drawSide` key)
- Test: `internal/ui/sidebar_tray_input_test.go`

**Interfaces:**
- Consumes: `sideAgentHit.tip`, `sideInk.tipFG/tipBG` (Tasks 10–11).
- Produces: `sideState.hoverAgent subagents.Key`; `sideHits.tip image.Rectangle` (empty when none).

Behaviour: on `pointer.Move/Enter`, the hovered `hitAgentLine` (anywhere on the line, `›` included) sets `hoverAgent`; `Leave` or another target clears it; `sideRedraw()` only when it changed. Drawn when `hoverAgent` matches a line with a non-empty `tip`: one line, 11 sp `ink.tipFG` on `ink.tipBG`, 1 px `th.ToastLine` outline, radius 6 dp, padding 6 dp; x from the tree line to `h.list.Max.X - 8dp`, text cut with `fitSide`; directly below the line, or above it when below would pass `h.list.Max.Y`. No delay, no timer.

- [ ] **Step 1: Write the failing tests**:
  - `TestTrayTooltipShowsStep`: agent with a step `Grep auth`; queue a Move to its line → frame → `win.side.hits.tip` non-empty, below the line; `win.side.redraws` rose by 1; three more frames with no input → `redraws` unchanged and no `InvalidateCmd` (`sidebar` asks for nothing).
  - `TestTrayTooltipFlipsAbove`: line at the list's bottom → `tip.Max.Y <= line.Min.Y`.
  - `TestTrayTooltipGoneOnLeave`: Leave → `hits.tip.Empty()`.
  - `TestSidebarCacheTooltipAllocatesNothing`: with the tooltip shown, unchanged draws allocate 0.
- [ ] **Step 2: Run** `go test ./internal/ui/ -run 'TestTrayTooltip|TestSidebarCacheTooltip'` → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/ui/` → PASS.
- [ ] **Step 5: Commit** `feat(ui): Show an agent's current step on hover`

### Task 14: Benchmarks, captures, audit, README and HANDOFF

**Files:**
- Create: `internal/render/emoji_bench_test.go`, `internal/ui/sidebar_tray_render_test.go`, `docs/audits/2026-11-03-sidebar-tray-emoji.md`, `docs/audits/sidebar-tray-emoji/*.png`
- Modify: `bench/flood/main.go` (+ `main_test.go`), `bench/run.ps1`, `bench/run.sh`, `bench/README.md`, `README.md` (subagents section, emoji note, Roadmap), `docs/HANDOFF.md` ("Where things stand" entry; line 383 "emoji in one color" → colour on Windows and macOS)

**Interfaces:**
- Produces: flood `-emoji` flag: `func emojiLine(b []byte, i int) []byte` writes a status-table row like `| 0042 | Shared 3D Secure helper | ✅ Done |`, cycling 20 emoji (`✅ ❌ 🔄 🟢 🔴 🟡 ⏳ 🚀 📝 ⚠ ✔ ❤` and others), `\r\n`-terminated, bounded by `-lines` (max 1,000,000). Scenario `emoji` in both scripts: `KAZE_BENCH_TYPE = "& '$flood' -emoji -lines 10100"`, `KAZE_BENCH_SCROLL = '10000'`.

- [ ] **Step 1: Write the failing test** `TestEmojiLine` in `bench/flood/main_test.go`: `emojiLine(nil, 7)` contains `|`, ends in `\r\n`, holds one emoji-presentation rune, and 100 lines cover ≥ 15 distinct emoji. Run `go test ./bench/flood/` → FAIL; implement; → PASS.
- [ ] **Step 2: Write** `BenchmarkEmojiRows` (render harness 120×40; each op prints 40 emoji table rows and lays out a frame) and `BenchmarkEmojiRowsMonochrome` (same with `emojiFonts.open` returning the font with `colr`/`sbix` nil, the "before" path), plus `BenchmarkEmojiRowsCached` (same text reprinted: glyphs already cached). Run `go test ./internal/render/ -run '^$' -bench 'BenchmarkEmojiRows' -benchmem -count 5` and record ns/op and allocs. Expected: `Cached` equals `Monochrome` within noise; the colour cost appears only on first sight of a glyph.
- [ ] **Step 3: App-level numbers:** `pwsh -File bench/run.ps1 -Only keys,emoji -Out <dir>` on this branch, and `-Only keys,flood` on a `main` worktree build (`git worktree add ..\kaze-before main`, run from there) for "before". Record keypress p50/p99 and frame-time percentiles in the audit. The harness drops real input; it sends none.
- [ ] **Step 4: Headless captures:** `TestSidebarTrayRenders` gated by `KAZE_TRAY_RENDER_DIR` (pattern of `TestSidebarRedesignRenders`): Sumi and Midnight, states default (3 lines + chip), expanded (8 + footer), folded, tooltip, and the project rail; plus `TestEmojiRenders` (in `internal/render`, gated by `KAZE_EMOJI_RENDER_DIR`) writing the status-table rows to PNG. Save under `docs/audits/sidebar-tray-emoji/`, inspect them against the mockup, and label them development captures, not product screenshots.
- [ ] **Step 5: Full check:** `go test ./...`, `go vet ./...`, `GOOS=darwin go vet ./internal/render/ ./internal/colorfont/` (cgo permitting) and the PR's macOS CI for the sbix test.
- [ ] **Step 6: Write the audit** `docs/audits/2026-11-03-sidebar-tray-emoji.md`: test list, benchmark tables (before/after), memory for 200 emoji (from `TestColourGlyphMemory`), the 0xFFFF neutral, the contrast numbers per theme, the captures, the flagged builder's calls ("Show fewer" at any count; U+FE0F follow-up issue) and the rulings below. Update README (tray, safe click, `›`, chip, fold, rail, colour emoji, Roadmap) and HANDOFF, and note that product screenshots need retaking per `docs/site-assets/SCREENSHOTS.md`.
- [ ] **Step 7: Commit** `docs(sidebar): Record the agent tray, rail and colour emoji audit`

## Rulings made while planning

Recorded here so reviewers and the owner can overturn them; each is cheap to change in its own task.

1. Tray is flat in state order (spec 3); nested agents lose the 12 dp depth indent (Task 8/11).
2. Tray membership is `Tree`'s rule unchanged (finished agents only while another runs) (Task 8).
3. "N agents" counts the whole tray, finished ones included, matching the mockup's 20 = 18 active + 2 done (Task 9).
4. When a tray exists, the fold control takes the state slot in place of "needs you"/"done"; the dot keeps the mark and the control turns amber for a needs-you row or a folded tray with a waiting agent (Task 9/11).
5. Clicking the fold control does not select the session (like a group chevron) (Task 12).
6. `›` toggles; a double-click only opens (Task 12).
7. Expanded with 3 or fewer agents left shows no footer (Task 11).
8. Chip says "1 needs you", "2 need you" (Task 9).
9. ASCII is never stripped as a status mark (Task 1).
10. A status-only title counts as no title in `ruled` (folder name wins), while `dropStatusMark` itself never returns empty (Task 1).
11. COLR/CPAL/sbix/cmap live in a new `internal/colorfont` package (Task 3/4).
12. Palette index 0xFFFF draws a fixed neutral `#9a9a9a` (Task 6).
13. Text-default emoji (`⚠ ✔ ❤ ⏺ ©`) use Segoe UI Emoji's outline on Windows after mono and CJK; macOS keeps today's order for them (Task 5).
14. Emoji-presentation runes go to the emoji face before the bundled mono font too (Task 5).
15. Segoe UI Emoji stays in the Windows system list for other runes; the emoji face is a separate handle (Task 5).
16. Contrast is a sidebar-local `render.Readable` nudge, not a theme change (Task 10).
17. The bold project name uses the bundled JetBrains Mono Bold added to the chrome faces (Task 10).
18. sbix fonts use the package's own cmap reader (Task 4).
19. Emoji are centred vertically and horizontally in their span (Task 6/7).
20. The tooltip sits below the line, above when there is no room, and is omitted when the step is empty (Task 13).
