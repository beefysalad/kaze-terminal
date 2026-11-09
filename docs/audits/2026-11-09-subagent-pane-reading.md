# Subagent pane reading

Branch `feat/pane-reading`, from `origin/main` at bdc2442 (release 0.37). Design:
`docs/superpowers/specs/2026-11-09-kaze-subagent-pane-reading-design.md`. Mockup:
`docs/design/pane-reading-mockup.html`, option B.

In Readable mode, in the docked panel and the board preview, edits open on their own with line
numbers, a +/- marker, full-width green and red rows and code colours. Quiet commands fold to one
line. Failed commands and failed edits open.

## Checks

| Check | Result |
|---|---|
| `go vet ./...` | clean |
| `go test ./...` (Windows, fresh run) | every package ok, `internal/ui` 46 s |
| `gofmt -l internal/ui` | clean |

## Frame cost

Limits from the design: a still frame with edits open at most 120 allocations and 400 µs; folded
within 5% of today. All timings are best-of-batch (200 frames per batch) on a shared, busy Windows
machine, so they move by tens of percent between runs.

| Scene | Allocations | Time | Limit |
|---|---|---|---|
| Edits open, 1000x600 (`TestOpenDiffsFrameTime`, 3 runs) | 73 | 290, 271, 216 µs | 120 / 400 µs |
| Edits open, 1400x1000, tall panel, about 30 diff rows (`diffScene`, 5 runs) | 73 | 330, 385, 313, 383, 336 µs | 120 / 400 µs |
| Folded, 21 steps, 1000x600 (3 runs) | 74 | 519, 249, 233 µs (the first run was a busy one) | 77.7 allocations (74 + 5%) |
| Folded, 21 steps, eb17e90 against the finished work, median of 5 alternating runs (Task 5 report) | 74 both | 212 µs before, 211 µs after | within 5% |

The big window is the one to watch. The 1400x1000 scene sat at 313 to 385 µs in this run. Task 5
measured the same scene at 357 and 421 µs, so on a loaded machine it can pass the 400 µs line. The
extra cost over folded there is about 150 µs, thought to be the input router and GPU walking the
replayed ops, since each row is several labels. Cutting it would mean fewer colours per row. The
1000x600 frame the limits were set on stays under 400 µs in every run. This is a real-session
check for the owner, not a pass or fail here.

## App size

Release-style build from CLAUDE.md (`-trimpath -ldflags "-s -w -H=windowsgui"`), built in a temp
folder, not over the installed copy.

| Build | Bytes |
|---|---|
| `origin/main` (bdc2442) | 19,346,944 |
| `feat/pane-reading` | 19,388,928 |
| Difference | +41,984 (+42 KB, +0.2%) |

CLAUDE.md quotes about 14 MB for this build. Both builds here came out near 19 MB (the repo has
grown since that note), so compare the two, not the quote.

## Captures

Made headlessly with `TestPaneReadingRenders` (new, opt-in, `KAZE_DOCK_RENDER_DIR`), which draws
through the Gio headless GPU path. No input was sent to any window and no Kaze was launched. 1400x860,
docked panel, Midnight and Washi:

- `pane-reading/reading-midnight.png`
- `pane-reading/reading-washi.png`

The run is a message, two folded reads, an applied Go edit, a quiet command that passed, a second
edit, a failed edit and a failed command.

What I looked at, against option B of the mockup, by opening each PNG:

- Edits are open without a click, no box behind them, rows run the full width of the panel.
- Line numbers sit in a right-aligned column, then the marker, then the code. The numbers show the
  real jump (41 to 46, then 118 and 119).
- Delete rows are red and add rows green, with `+3 -2` and `+1 -1` at the right of the header, as in
  B. Context lines have no background.
- Code colours show keywords, function names, strings and comments, in both themes. Washi's pale
  rows read clearly and the text stays dark on them.
- The passed command is one line with `done`. The failed edit shows `failed` and its error line.
  The failed command is open, with its command shown (cut off at the bottom of the frame).
- Reads fold to "Read 2 files", as in B.

Seen and not fixed (product code is out of scope for this task):

- Spacing around coloured words is slightly uneven in places, for example extra space after `func`
  and around `:=` and `nil`. Each colour span is its own label, so a boundary can show a small gap.
  The row columns still line up at the start of each line. Worth a look in a real session.
- `!=` draws as the JetBrains Mono ligature `≠`.
- The Midnight and Washi frames show a terminal that is empty, because the fixture has no output.

What I did not inspect: a real subagent session, the board preview in these two themes (the existing
`TestReadableRenders` fixture has placeholder edits with no numbers or code, so its board and panel
frames do not show this work; it now also renders Washi), a Mac, or any other theme. Contrast of the code and number
colours is tested in all ten themes by `TestReadInkContrast` (4.5:1 on the panel and on add and
delete rows).

## Owner checks

See `docs/HANDOFF.md`. In short: a real run, odd spacing around colours, a 40-line edit and its
"N more lines", failed edit and command, fold and reopen, and scrolling on a large window.
