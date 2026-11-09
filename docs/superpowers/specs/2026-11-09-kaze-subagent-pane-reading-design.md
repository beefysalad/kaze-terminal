# Reading a subagent's work: design

Status: the owner chose option B from the mockup and approved the description on 2026-11-09.
Mockup: `docs/design/pane-reading-mockup.html` (a fragment; the option B block is the target).
Code: `internal/ui/dock_readable.go`, `internal/ui/dock_readable_draw.go`,
`internal/subagents/transcript.go`.

## Why

The owner watches subagents work in Kaze's pane. Claude Code's own subagent view shows edits inline
with line numbers, green and red rows and code colours. Kaze's Readable view shows one collapsed row
per step, so the owner has to open each step to see anything, and an opened step is a cramped grey
box with no line numbers or code colours. Reading a run is tedious.

## Goal

A subagent's run can be read top to bottom without clicking: what it said, what code it changed, and
what went wrong.

## What it does (option B)

Applies to the Readable mode, wherever it is drawn: the docked pane and the agents board preview.

| Item | Default | Details |
|---|---|---|
| Agent messages | Shown | Coral bar, unchanged, the most prominent thing in the list |
| Edits (Edit, Write, MultiEdit) that were applied or are still running | Open | Inline diff, no box |
| Edits that failed or were rejected | Open | No diff, since nothing changed. A `failed` tag and the first lines of the error, shown like a failed command (builder's call, not confirmed by the owner) |
| NotebookEdit and apply_patch | One line | Header only, with no diff and no `+0 -0`. Neighbouring ones share a line, as before this work. One that failed stands alone and opens with its error, like a failed edit (builder's call, not confirmed by the owner) |
| Commands that succeeded or are still running | One line | Click opens the output in place, no box |
| Commands that failed | Open | Output shown in place. Click folds it |
| Reads and searches | One quiet line | A neighbouring run shares a line: `Searched the code · Read 2 files` |
| Other steps | One line | As today |

**The inline diff:**
- Header row: tool word and path, with `+N -M` at the right, as in the mockup.
- Each line: real file line number, `+` or `-` marker, code. Added and removed rows use the theme's
  `DiffAddBG` and `DiffDelBG` across the full row. Unchanged context lines have no background.
- No border, no box background. Rows sit directly in the list, slightly indented.
- At most 40 lines per edit, then a muted `N more lines` row. Today's cap is 24.
- Long lines clip at the pane's edge. No wrapping and no sideways scrolling.

**Folding:** clicking a step's header folds or opens it, and that choice stays for that step while
the pane lives, whatever its default was. It is not saved across restarts.

**Code colours:** a small built-in colouring, not a library, to keep Kaze small. Go, Python,
TypeScript and JavaScript (with TSX and JSX), shell, and JSON, chosen by file extension. Token kinds:
keyword, string, comment, number, function or type name. Any other file shows plain diff colours.
Colouring is per line and best effort: a multi-line string or comment may colour imperfectly, which
is acceptable. Command output is never coloured.

## Where the data comes from

Transcript user records carry a top-level `toolUseResult.structuredPatch`, a list of
`{oldStart, oldLines, newStart, newLines, lines}`, where each entry in `lines` starts with a space,
`+` or `-`. This was confirmed in real subagent transcripts on 2026-11-09. It is the source of the
real line numbers and of the context lines, and is paired to its step by the tool_use id, as results
are today. A record has one `toolUseResult`, so it is applied only when the record holds exactly one
`tool_result` block. With several, every step keeps today's diff and shows no line numbers.

A Write whose result has type `update` and a present but empty `structuredPatch` rewrote the file
with the same content: it shows no diff and `+0 -0`, as one row.

When it is missing (the step's result has not arrived yet, or an agent that does not write it), keep
today's diff from `old_string` and `new_string`, and show no line numbers. A diff must never appear
to have wrong numbers.

`toolUseResult` also holds `originalFile`, so a record for an edit to a big file can exceed
`MaxTranscriptLine` (256 KiB) and be skipped by the existing oversized-line handling. Such a step
keeps the fallback diff. The transcript reader's memory bounds must not be loosened to read those.

## Constraints

- **Lean over features.** No new dependency for colouring. No new polling or timers.
- Memory stays bounded: 200 steps, and the existing 256 KiB total for detail, where the oldest steps
  lose detail first. A step that has lost its detail shows as one line. The per-step detail cap may
  rise only as far as 40 lines needs.
- Colour a line once, when the step's item is built, and keep the result. Never tokenise while
  drawing.
- Measure against the numbers in `docs/audits/2026-11-04-battery-activity-names.md`: Readable at
  about 200 to 207 microseconds and 73 allocations per frame (1000x600, 21 steps). Limits, set with
  the owner on 2026-11-09: a still frame with edits open stays at or under 120 allocations and
  400 microseconds, and a pane with everything folded stays within 5% of today. Colours and the
  added and removed rows are not to be traded away to meet these: the owner's priority is seeing
  that it is code and what was added or removed, so a retained copy of each drawn row is expected.
- One font: JetBrains Mono, Regular and Bold only.
- Every token colour must stay readable on `DiffAddBG`, `DiffDelBG` and the panel background in all
  ten themes, at the contrast the existing theme tests require (4.5:1 for text).
- Strict test first. Never inject real input when testing the GUI.

## Not in scope

- The **Every step** mode stays as it is.
- Codex helpers and agents without a transcript keep their hook-based steps.
- The main terminal, the Report tab, and the markdown reader are unchanged.
- No new settings.
- Full language parsing, or colours for languages not listed.

## Would be a failure even if it works

- Scrolling the pane stutters or a still frame allocates noticeably more.
- Line numbers that do not match the real file.
- A huge edit or output floods the pane or grows memory.

## Left to the builder (calls made while writing this, none confirmed by the owner)

- Exact row heights, indent and spacing, as long as the result reads like the mockup.
- The tool word in the header. The mockup uses Claude Code's `Update` and `Bash(...)`; keeping
  Kaze's past-tense sentences (`Edited`, `Ran`) is equally fine if it reads better in the pane.
- Whether clicking the shared reads and searches line lists what is in it. Default: it opens the
  list, as a fold does today.
- How hunks of a multi-hunk edit are separated.
- How the token colours are defined per theme, as long as the contrast rule holds.
- Whether the mixed folding state is stored as an inverted set or per-step overrides.
