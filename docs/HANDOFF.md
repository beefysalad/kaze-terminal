# kaze handoff

Last updated: 2026-10-28, after Plan 1 shipped. Read this first when you pick up kaze on another machine.

## Where things stand

**Plan 1 (terminal core with tabs) is done**, reviewed task by task plus a whole-branch review, and on `main`.

What works (tested on Windows):

- Tabs (Ctrl+Shift+T / W, Ctrl+Tab), each with its own shell (pwsh on Windows, zsh on macOS) and `KAZE_TAB_ID`.
- Full xterm-style emulation for Claude Code, git, vim, less: colors (256 + truecolor), alt screen, scroll regions, bracketed paste, mouse modes, focus events (1004), synchronized output (2026).
- Fast renderer: CPU-drawn grid in 8 strips, only changed rows redrawn, nothing drawn when idle.
- Japanese: wide characters, bundled Noto Sans JP subset, system font fallback for rare kanji.
- Symbols Claude Code prints (`⏺ ⎿ ✻`, spinners, emoji in one color) via system symbol fonts. Box-drawing is drawn by kaze so borders join.
- Readable text on colored backgrounds (minimum-contrast nudge).
- Mouse selection (drag, double-click word, triple-click line, Shift+drag in mouse-mode apps), Ctrl+Shift+C / V, optional `copy_on_select`.
- Status bar: kaze's private memory and tab count.
- Config file with warnings for bad values.

Measured (Windows, `docs/benchmarks/2026-10-28-milestone-1.md`):

| Goal | Result | Met |
|---|---|---|
| Claude Code streaming frames < 16.7ms | p99 5.1ms | yes |
| Smooth scroll through 10,000 lines | no late frames | yes |
| Idle CPU ~0 | 0.0% with 4 tabs | yes |
| Memory doesn't grow when idle | ~147MB, flat; +0.5MB per idle tab, ~+30MB per tab with full history | yes |
| Keypress to glyph < 10ms | ~25ms typical; kaze's own part ~8ms p50, ~24ms p99 | **no** |
| Launch to prompt < 500ms | ~1.3s with pwsh (kaze's first frame ~0.45s) | **no** |

## Next

1. **Mac milestone** (checklist below): build and use kaze on macOS for the first time.
2. **Plan 3**: attention alerts + chime + kaze's own title bar + folder/branch in the status bar. Decisions are in spec section 6.8; mockup in `docs/design/titlebar-and-status.html`. Start with a short plan (`docs/superpowers/plans/`), then build.
3. **Plan 2**: subagent child tabs (spec 6.3).
4. **Plan 4**: Claude usage in the status bar (spec 6.5).
5. Speed task: tighten kaze's own keypress p99 and first-frame time.
6. Later (spec 7 and owner ideas): browser button, notes-vault graph, smart Ctrl+C (copy when text is selected, else interrupt), kaze-native autocomplete.

## Mac milestone checklist

kaze has **never been built on macOS**. Expect small fixes.

1. Install Go (version in `go.mod`) and the Xcode Command Line Tools: `xcode-select --install`.
2. Clone and build: `git clone https://github.com/patrick-gogo/kaze-terminal && cd kaze-terminal && go build -o kaze ./cmd/kaze && ./kaze`.
3. Run the tests: `go test ./...` (several packages could only be type-checked for darwin from Windows: `internal/ui`, `internal/render`, `internal/bench`, `cmd/kaze`).
4. Check by hand, in a kaze tab:
   - zsh starts; `echo $TERM $COLORTERM $TERM_PROGRAM` prints `xterm-256color truecolor kaze`.
   - `claude` renders with colors, its input box borders join, symbols show.
   - Japanese file names: macOS stores names in NFD, and kaze currently **drops combining marks**, so `ls` of Japanese names may lose dakuten (が → か). Known gap, fix it here.
   - Rare kanji draw (Hiragino fallback paths in `internal/render/fontpaths_darwin.go` are unverified).
   - Cmd+T / Cmd+W / Cmd+C / Cmd+V work.
   - **Option key:** kaze treats Option as Alt unconditionally. On layouts that type `@ { | [ ]` with Option this breaks; add a config toggle.
   - Trackpad/mouse wheel: scroll speed may be off (unit not verified on macOS).
   - Status bar memory on macOS uses Go's runtime numbers, not process RSS; consider `task_info`.
5. Benchmarks: `bench/run.sh` (it uses a history-free shell and kaze drops real input in bench mode, so it can't type into your sessions). Add a "macOS" column to the milestone doc.

## Decisions so far (why the code looks the way it does)

- **Go + Gio**, one codebase for Windows and macOS. Beating Warp is not a goal; "fast, not laggy, fits the workflow" is. Memory has no hard cap but must not grow when idle.
- **Renderer:** a CPU-drawn image of the grid, split into 8 horizontal strips; only strips with changed rows are re-uploaded. Chosen by two spikes (`docs/spike-results.md`): Gio's text path and per-glyph ops were far too slow.
- **Synchronized output:** frames are held while mode 2026 is on, up to 150ms, but never when nothing has been drawn yet (tab switch).
- **Parser** treats `:` like `;` but records which params were colon-separated, so SGR `4:3` (curly underline) and `38:2::r:g:b` work.
- **Sessions don't survive kaze closing** (same as Warp). Shells live in the kaze process behind `ptyx.Session`.
- **Shell env:** each tab gets `KAZE_TAB_ID`, `TERM=xterm-256color`, `COLORTERM=truecolor`, `TERM_PROGRAM=kaze`; inherited `TERM_PROGRAM`/`WT_*` from the launching terminal are dropped.
- **Title bar:** Plan 1 kept the native one; Plan 3 replaces it (spec 6.8).
- **Copy is Ctrl+Shift+C** (Ctrl+C must stay "interrupt"). Smart Ctrl+C is a later idea, not wanted yet.
- **Ctrl+Backspace** sends 0x08 (word delete in PSReadLine).
- **Claude Code features left unanswered** (kitty keyboard protocol, XTVERSION, some OSCs): Claude falls back fine. Side effect: Shift+Enter submits; use Alt+Enter for a newline.
- **Fonts:** JetBrains Mono + Noto Sans JP subset bundled (OFL); missing symbols and rare kanji come from system fonts, loaded lazily, memoized, malformed fonts skipped.
- **Low-contrast text** is nudged just enough to reach 4.5:1, not forced to white (keeps dim text dim).
- **Benchmarks** measure keypress latency inside kaze (`KAZE_BENCH`), never with global input.

## Known gaps (deferred, not blocking)

- Combining marks dropped (NFD on macOS, see checklist).
- macOS: Option-as-Alt toggle, wheel units, RSS reading, Hiragino paths unverified.
- Scrollback stores 20 bytes per cell; a full 10,000-line history costs ~30MB per tab ("stored compactly" from spec 6.1 not done yet).
- Ctrl+Alt+letter is dropped on a US Windows layout.
- Status bar lacks CPU and the per-tab process tree (spec 6.5, Plan 4).
- `font_family` is accepted but not supported yet (kaze warns).
- Glyph cache is safe only with one window / one UI goroutine (add a mutex before multi-window).
- Closing a tab whose ConPTY close times out can keep that screen in memory until ConPTY lets go.
- Smaller items: see the deferred list in the final review notes, if you have them; otherwise grep tests for `TODO`.

## Optional shell setup

Autocomplete in pwsh (history suggestions + Tab menu) is shell-level, not kaze. In `$PROFILE`:

```powershell
if ($Host.Name -eq 'ConsoleHost' -and -not [Console]::IsOutputRedirected -and (Get-Module -ListAvailable PSReadLine)) {
    Set-PSReadLineOption -PredictionSource History -PredictionViewStyle ListView
    Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete
}
```

macOS equivalent: the `zsh-autosuggestions` plugin.
