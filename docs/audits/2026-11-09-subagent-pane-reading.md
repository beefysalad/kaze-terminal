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
| Edits open, 1000x600 (3 runs after the column fix) | 73 | 220, 286, 218 µs | 120 / 400 µs |
| Edits open, 1400x1000, tall panel, about 30 diff rows (3 runs after the column fix) | 73 | 342, 339, 346 µs | 120 / 400 µs |
| Folded, 21 steps, 1000x600 (3 runs after the column fix) | 74 | 221, 218, 212 µs | 76.65 allocations and 217 µs (the spec's 73 and 207 µs, + 5%) |
| Folded, 21 steps, 1400x1000 (3 runs after the column fix) | 78 | 409, 358, 348 µs | the spec has no baseline at this size |
| Folded, 21 steps, eb17e90 against the finished work, median of 5 alternating runs (Task 5 report) | 74 both | 212 µs before, 211 µs after | shows this work added nothing |

The folded limit is measured against the spec's baseline: 73 allocations and 207 µs on a 1000x600
frame, so 76.65 allocations and about 217 µs. The folded still frame is 74 allocations, inside that
limit by count, and about 212 to 221 µs, which sits at the time limit and past it in two of three
runs, on a machine that moves by tens of percent. Against eb17e90, the commit just before inline
diffs, it was already 74 allocations and 212 µs, so the 1 allocation and most of the time were there
before this work. `TestOpenDiffsStayWithinAllocations` guards a limit of 77.7 from eb17e90's 74, one
step looser than the spec's 76.65.

The big window is the one to watch. The 1400x1000 scene sat at 339 to 346 µs in these runs. Task 5
measured the same scene at 357 and 421 µs, so on a loaded machine it can pass the 400 µs line. A
folded pane at that size costs about the same (348 to 409 µs), so most of the time there is the
big window itself rather than the open edits. The
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
- Code lines up. Each colour span is placed at its character column from one measured character
  width (a label's own width is rounded and leaves out trailing spaces, which once put false gaps
  around coloured words and started equal indents at different columns). Rows with the same indent
  start together, and a line of many colours ends where a plain line of the same length ends
  (`dock_diff_columns_test.go`). Japanese text in a line is best effort: what follows it moves past
  it, so that line's later columns no longer match its neighbours.

Seen and not fixed (product code is out of scope for this task):

- `!=` draws as the JetBrains Mono ligature `≠`.
- The Midnight and Washi frames show a terminal that is empty, because the fixture has no output.

What I did not inspect: a real subagent session, the board preview in these two themes (the existing
`TestReadableRenders` fixture has placeholder edits with no numbers or code, so its board and panel
frames do not show this work; it now also renders Washi), a Mac, or any other theme. Contrast of the code and number
colours is tested in all ten themes by `TestReadInkContrast` (4.5:1 on the panel and on add and
delete rows).

## Owner checks

See `docs/HANDOFF.md`. In short: a real run, a 40-line edit and its
"N more lines", failed edit and command, fold and reopen, and scrolling on a large window.
