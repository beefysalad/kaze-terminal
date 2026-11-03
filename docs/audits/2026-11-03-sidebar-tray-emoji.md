# Sidebar agent tray, project rail, title symbols and colour emoji (#125, #124, #127): verification

Branch `feat/sidebar-tray-emoji`. Design: `docs/superpowers/specs/2026-11-03-kaze-sidebar-tray-emoji-design.md`.
Plan: `docs/superpowers/plans/2026-11-03-kaze-sidebar-tray-emoji.md`. Mockup: `docs/design/sidebar-agents-mockup.html` (PR #126).

## What changed

- **Agent tray (#125).** Subagents sit in a slim tray under their session behind a thin tree
  line: one 22 dp line each (mark, type, task, elapsed, `›`). At most 3 lines, waiting first,
  then a chip "+N more · R running, D done ⌄" ("W need you" in amber when a hidden agent waits).
  The chip expands to 8 lines and a footer, "Show fewer" (past 8: "· N more on the board"). The
  row's "N agents ⌄ / ›" folds the whole tray. The current step moved to a hover tooltip with no
  timer.
- **Safe click.** A click on an agent line does what a click on its session row does; only the
  20 dp `›` (or a double-click on the line) opens the agent's panel.
- **Project rail.** The active project's tinted, outlined header is gone; a 2 dp accent rail runs
  from its header to its last row and tray, and its name is bold.
- **Title symbols (#124).** One helper strips any leading run of status symbols (`◐`, braille,
  `✳ ✻ ✽ ⏺` and the rest of So/Sm, geometric shapes, dingbats, misc technical) from the sidebar
  title, the title-bar crumb and the resource card. UI fonts never draw a missing-glyph box.
- **Colour emoji (#127).** Emoji-presentation runes go to the system emoji font first: Segoe UI
  Emoji's COLR v0 layers and CPAL palette 0 on Windows, Apple Color Emoji's sbix PNGs on macOS.
  Each glyph is rasterized once into the existing glyph cache as an RGBA image.
- **Narrow chip (this audit's fix).** At 11 sp in the default 260 dp sidebar the chip
  "+17 more · 15 running, 2 done ⌄" was cut with "…" and lost its caret, and the footer read
  "Show fewer · 12 more on the …". A chip that does not fit now drops its counts first, then the
  "need you" part, and always keeps "+N more" and `⌄`. A footer that does not fit says
  "· 12 on the board".

## Tests

New or changed tests, all passing (`go test ./...` on Windows):

- `internal/colorfont`: `TestReadDirectory`, `TestReadDirectoryRejects`, `TestParseCOLR`,
  `TestParseCPAL`, `TestParseCOLRRejects`, `TestParseCPALRejects`, `TestSegoeEmojiCOLR`,
  `TestParseCmapFormat12`, `TestParseCmapFormat4`, `TestParseCmapRejects`, `TestSbixNearestStrike`,
  `TestSbixPNG`, `TestSbixPNGTooLarge`, `TestSbixRejects`.
- `internal/render`: `TestEmojiTables`, `TestCheckMarkUsesEmojiFace`, `TestTextEmojiKeepsOldFonts`,
  `TestNoEmojiFontKeepsOldOrder`, `TestCheckMarkIsColour`, `TestColourGlyphIgnoresFG`,
  `TestSelectionAndInverseShowBehindEmoji`, `TestBrokenColourTablesDrawMonochrome`,
  `TestColourGlyphMemory`, `TestColourGlyphRasterizedOnce`, `TestSbixGlyphScaledIntoSpan`,
  `TestSbixHugePNGDrawsBlank`, and on macOS `TestAppleColorEmojiIsColour`.
- `internal/subagents`: `TestTrayOrderAndCap`, `TestTrayDropsFinishedWhenNoneRun`.
- `internal/ui`: `TestSidebarTextContrast`, `TestActiveProjectRail`, `TestActiveProjectNameStyle`,
  `TestTrayBoardLinkOpensFilteredBoard`, `TestTrayTexts`, `TestTrayCaretsDraw`, `TestTrayStateLimits`,
  `TestTrayStateRebuildsModel`, `TestTrayStatePrunedWithSession`, `TestSidebarDrawsTray`,
  `TestSidebarDrawsTrayFooter`, `TestSidebarFoldedTrayHasNoLines`, `TestTrayChipKeepsCaretAtDefaultWidth`,
  `TestTrayFooterKeepsBoardLinkAtDefaultWidth`, `TestSidebarCacheTrayAllocatesNothing`,
  `TestSidebarCacheTooltipAllocatesNothing`, `TestAgentLineShowsSession`, `TestAgentButtonTogglesPanel`,
  `TestAgentLineDoubleClickOpensPanel`, `TestAgentLineDoubleClickNeedsSameLine`,
  `TestAgentLineMatchesRowClick`, `TestTrayChipExpandsAndFewerCollapses`, `TestTrayFoldControl`,
  `TestTrayTooltipShowsStep`, `TestTrayTooltipFlipsAbove`, `TestTrayTooltipGoneOnLeave`,
  `TestDropStatusMark`, `TestDropStatusMarkKeepsASCII`, `TestRuledStatusOnlyTitleFallsBack`,
  `TestResourceNameDropsSpinner`, `TestUIFontsNeverDrawBoxes`, `TestFitSansKeepsJapanese`.
- `bench/flood`: `TestEmojiLine`.

The narrow-chip fix was test first: `TestTrayChipKeepsCaretAtDefaultWidth` failed with
`chip drew "+17 more · 15 running, 2 don…"` and `"+17 more · 2 need you, 13 ru…"`, then passed
(`"+17 more ⌄"`, and `"+17 more · 2 need you ⌄"` with the amber part kept).
`TestTrayFooterKeepsBoardLinkAtDefaultWidth` failed with `"Show fewer · 12 more on the …"`, then passed.
The unchanged-sidebar tests (`TestSidebarCache*`) still allocate nothing.

## Measurements

Windows 11, 12th Gen Intel i7-1255U (the owner's laptop).

### Render benchmarks

`go test ./internal/render/ -run '^$' -bench BenchmarkEmojiRows -benchmem -count 5`. Each op
prints 40 status-table rows (`| 0042 | Shared 3D Secure helper | ✅ Done |`, 20 emoji cycling,
shifted by one per op so every emoji cell changes) on a 120x40 screen and lays out a frame.
Medians of 5:

| Benchmark | What it measures | ns/op | B/op | allocs/op |
|---|---|---|---|---|
| `EmojiRows` | colour, glyph cache emptied each op (first sight of every glyph) | 1,636,506 | 83,152 | 489 |
| `EmojiRowsMonochrome` | the old path: emoji font without COLR/sbix, cache emptied each op | 1,255,894 | 38,988 | 283 |
| `EmojiRowsCached` | colour, glyphs already cached (every later frame) | 401,000 (run 1), 457,415 (run 2) | 17,981 | 156 |
| `EmojiRowsMonochromeCached` | old path, glyphs cached | 399,486 | 18,250 | 156 |

The colour cost is only on first sight of a glyph (about 20 µs more per emoji, once). With glyphs
cached, colour and monochrome frames are the same within noise and allocate the same.

### Colour glyph memory

`TestColourGlyphMemory`: 200 distinct colour emoji at 14 px take **256,000 bytes** (250 KiB) of
RGBA images, well under the 4 MiB test limit. The glyph cache cap (in glyphs) is unchanged.

### App benchmarks (`bench/run.ps1`)

"Before" is `main` at `d2b9e30` (0.24) built in a separate worktree; "after" is this branch.
Single runs, so a few ms of p99 swing between runs is normal on this laptop. The harness sends no
real input. The `emoji` scenario on main used this branch's `flood -emoji` (main's flood has no
`-emoji`), so it shows the old monochrome drawing of the same text.

| Scenario | Number | Before (main) | After (run 1) | After (run 2) |
|---|---|---|---|---|
| keys (200 keys, 50 ms apart) | keypress p50 / p99 | 19.6 / 27.6 ms | 20.3 / 39.2 ms | 18.8 / 30.9 ms |
| keys | private memory | 140.5 MB | 144.1 MB | 143.5 MB |
| emoji (10,100 emoji rows, then scroll 10,000) | frame p50 / p99 | 3.72 / 12.7 ms | 3.08 / 12.1 ms | |
| emoji | scroll frame p50 / p99 | 4.10 / 8.49 ms | 3.53 / 9.03 ms | |
| emoji | private memory | 164.0 MB | 169.1 MB | |
| flood (10,100 colored lines, then scroll) | frame p50 / p99 | 4.00 / 9.16 ms | | 4.34 / 15.1 ms |
| flood | scroll frame p50 / p99 | 4.77 / 9.52 ms | | 4.83 / 8.36 ms |
| flood | private memory | 174.1 MB | | 176.8 MB |

Keypress p50 is unchanged. Keypress and flood p99 vary run to run (39.2 then 30.9 ms on the same
build); nothing in this branch runs per key. Emoji frames are no slower than main's monochrome
drawing. Private memory is 3 to 5 MB higher in these single runs; it was not broken down further.

## Contrast (sidebar text, at least 4.5:1)

`TestSidebarTextContrast` checks these in every theme; values from the same colours:

| Theme | Muted on sidebar | Amber on sidebar | Agent type | Chip lead on chip | Muted on chip | Amber on chip | Tooltip |
|---|---|---|---|---|---|---|---|
| Midnight | 6.10 | 10.86 | 9.22 | 8.57 | 5.62 | 10.00 | 13.56 |
| Sumi | 4.84 | 10.03 | 7.21 | 7.79 | 4.54 | 9.39 | 13.60 |
| Tide | 7.06 | 13.30 | 9.21 | 11.65 | 6.12 | 11.52 | 12.77 |
| Washi | 5.11 | 5.10 | 4.72 | 6.50 | 4.57 | 4.56 | 13.94 |
| Sakura | 6.49 | 12.29 | 8.96 | 10.39 | 5.77 | 10.94 | 13.15 |

Sumi and Washi failed before the nudge (Sumi's muted was 4.14:1 on the sidebar).

## Development captures

Headless renders, development evidence only, **not product screenshots**:

- `KAZE_TRAY_RENDER_DIR=<dir> go test ./internal/ui/ -run TestSidebarTrayRenders`: the real
  sidebar drawing in the Gio GPU (no native window, no input), four columns at the default 260 dp:
  default (3 lines and the chip), expanded (8 lines and the footer), folded, and the tooltip on the
  second line (with a hidden waiting agent, so the chip keeps "1 needs you"). The project rail
  marks the group in each.
- `KAZE_EMOJI_RENDER_DIR=<dir> go test ./internal/render/ -run TestEmojiRenders`: an emoji-heavy
  terminal sample (status table, `⏺ ✔ ⚠ ❤`, Claude's `✳` spinner, Japanese) as kaze's renderer
  draws it on the CPU.

| Capture | File |
|---|---|
| Tray and rail, Sumi | [tray-sumi.png](sidebar-tray-emoji/tray-sumi.png) |
| Tray and rail, Washi | [tray-washi.png](sidebar-tray-emoji/tray-washi.png) |
| Tray and rail, Midnight | [tray-midnight.png](sidebar-tray-emoji/tray-midnight.png) |
| Emoji sample (Windows, Segoe UI Emoji) | [emoji-sample.png](sidebar-tray-emoji/emoji-sample.png) |

Checked by eye against the mockup: the tray, chip, footer, fold control, tooltip and rail match;
the chip and footer are whole at 260 dp after this audit's fix. In the emoji sample `⏺` draws as
a solid dot from Segoe UI Symbol, as in 0.24 (re-rendered after the final-review fix below).

## Rulings

### Plan (recorded for the owner to overturn)

1. The tray is flat in state order; nested agents lose the 12 dp depth indent.
2. Tray membership is `Tree`'s rule unchanged (finished agents only while another runs).
3. "N agents" counts the whole tray, finished ones included (20 = 18 active + 2 done).
4. With a tray, the fold control takes the row's state slot; the dot keeps the mark and the
   control is amber for a needs-you row or a folded tray with a waiting agent.
5. Clicking the fold control does not select the session.
6. `›` toggles the panel; a double-click only opens it.
7. Expanded with 3 or fewer agents shows no footer.
8. The chip says "1 needs you", "2 need you".
9. ASCII is never stripped as a status mark.
10. A status-only title counts as no title in `ruled` (the folder name wins); `dropStatusMark`
    itself never returns empty.
11. COLR, CPAL, sbix and cmap live in a new `internal/colorfont` package.
12. Palette index 0xFFFF draws a fixed neutral `#9a9a9a`.
13. Text-default emoji (`⚠ ✔ ❤ ⏺ ©`) keep 0.24's fonts on both systems (mono, CJK, then the
    system list with Segoe UI Symbol first). Overturned in the final review: the earlier ruling sent
    them to Segoe UI Emoji's outline, which changed about 184 runes, `⏺` and `⏸` among them.
14. Emoji-presentation runes go to the emoji face before the bundled mono font too.
15. Segoe UI Emoji stays in the Windows system list for other runes; the emoji face is a separate handle.
16. Contrast is a sidebar-local `render.Readable` nudge, not a theme change.
17. The bold project name uses the bundled JetBrains Mono Bold added to the chrome faces.
18. sbix fonts use the package's own cmap reader.
19. Emoji are centred in their span.
20. The tooltip sits below the line, above when there is no room, and is left out when the step is empty.

Flagged builder's calls from the design: **"Show fewer" shows at any count past the cap**, and
**U+FE0F is out of scope** (text-default emoji with U+FE0F draw in one colour at their normal
width; keeping U+FE0F and deciding 1 or 2 cells is a follow-up issue to open).

### Lane A (colour emoji)

- Test table builders live in `internal/colorfont/colorfonttest`; `COLR.NumBase` was added.
- `✳` (U+2733) is excluded from the text-emoji set, so Claude's spinner never switches font (the set itself was removed in the final review).
- `⚠` stays on JetBrains Mono (it has the glyph); the text emoji `❤ ✔ ⏺` keep their 0.24 fonts
  (see the final review).
- sbix fonts never use outlines.
- 200 emoji take 256,000 bytes at 14 px; palette 0xFFFF draws neutral `#9a9a9a`.

### Lane B (sidebar)

- Fallback fonts are re-registered at Medium, SemiBold and Bold so Japanese and symbols don't box
  in UI text (instead of the plan's `fitSans`).
- Muted and amber sidebar text are nudged to 4.5:1 on both the chip background and the sidebar
  (Sumi and Washi failed before); `sideTheme()` is a copy per theme.
- The tree line ends at the chip.
- Chip and "Show fewer" clicks don't select the session.
- `TestSidebarAgentTogglesDock` is replaced by `TestAgentButtonTogglesPanel`.
- The "N agents ⌄" fold control replaces "needs you" in the row's state slot (the dot keeps the
  mark, the count is amber when waiting).
- The whole tray line selects the session; only the 20 dp `›` opens the panel.
- The tooltip's "no idle wake" is tested as an unchanged router wakeup.

### This audit

- A chip that does not fit drops its counts first, then "N need you", and never loses "+N more"
  or `⌄`. A footer that does not fit shortens "N more on the board" to "N on the board".
- The drawn chip and footer parts are kept in `sideHits.chip` so tests can read them.

## Final review fixes

1. **Text emoji keep their 0.24 fonts** (`fix(render)`). The step that sent text-default emoji to
   Segoe UI Emoji's monochrome outline before Segoe UI Symbol is gone, with its face and the
   text-emoji table (`gen_emoji.go` no longer writes it). `⏺` (every Claude Code reply) and `⏸`
   (plan mode) are Segoe UI Symbol again; `✅ ❌ 🔄 ⏳` still go to the colour emoji face first.
   `TestTextEmojiKeepsOldFonts` replaces `TestTextEmojiUsesEmojiOutline`. `blendMask` and
   `blendColor` have their own doc comments again.
2. **Fold control clear of the ×** (`fix(ui)`). On a hovered row the fold hit rect touched the ×'s,
   and a near-miss closed the session. The fold rect now ends at its text and the × drops its left
   pad when a fold is shown: an 8 dp dead zone (`TestTrayFoldClearOfClose`).
3. **Variation selectors are part of a status mark** (`fix(ui)`). U+FE0E, U+FE0F, U+200D and Sk
   (skin tones) no longer end the leading run, so a title of just `⚡️` falls back to the folder and
   `⚡️ Deploy` becomes `Deploy` (`TestStatusMarkTakesVariationSelectors`).
4. **Fold and open-line contrast** (`test(ui)`). `TestSidebarTextContrast` now also checks the fold
   control (accent, and amber) on the sidebar and on a selected row, and the open agent line's
   task, muted, amber and type text on `Active`. All pass without a nudge:

   | Theme | Fold on sidebar | Fold on selected | Amber fold on selected | Task on open line | Muted on open line | Type on open line |
   |---|---|---|---|---|---|---|
   | Midnight | 5.90 | 5.73 | 10.54 | 11.87 | 5.92 | 8.95 |
   | Sumi | 5.22 | 5.04 | 9.68 | 14.46 | 4.68 | 6.96 |
   | Tide | 11.53 | 11.06 | 12.75 | 13.73 | 6.77 | 8.83 |
   | Washi | 5.53 | 6.16 | 5.68 | 12.69 | 5.68 | 5.25 |
   | Sakura | 8.96 | 8.63 | 11.84 | 14.10 | 6.25 | 8.63 |

### Deferred minors

- Colour emoji ignore `AttrDim`.
- Colour glyph memory can reach about 28 MB at about 40 px with every emoji cached at two sizes
  (bounded by the cache).
- On Windows `seguiemj.ttf` is opened and parsed twice (emoji face and system fallback list).
- On macOS an sbix glyph that fails to decode draws blank instead of falling through to the next font.
- The tray tooltip covers the lines below it and is click-through.

## Checks

- `go test ./...` and `go vet ./...` pass on Windows (Smart App Control needed `GOTMPDIR` under
  the Desktop and fresh build ids).
- `GOOS=darwin go vet ./internal/render/ ./internal/colorfont/` passes. The sbix test against
  Apple Color Emoji (`TestAppleColorEmojiIsColour`) runs in the PR's macOS CI.

## Owner checks still open

- A real Claude session with many subagents: the tray order, the chip, expand and fold, and that a
  click on a line no longer opens a panel by mistake.
- Sumi's rail uses the theme accent, which is red; check that it reads as "you are here", not an error.
- Colour emoji in a real Claude status table on Windows, and Apple Color Emoji on a Mac.
- Open the U+FE0F follow-up issue.
- Product screenshots of the sidebar need retaking (`docs/site-assets/SCREENSHOTS.md`); the
  captures above are not product screenshots.
