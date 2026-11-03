# Sidebar agent tray, project rail, title symbols and colour emoji (#125, #124, #127)

Status: #125's design and mockup were approved by the owner on 2026-11-03 (mockup:
`docs/design/sidebar-agents-mockup.html`, PR #126). For #127 the owner chose real colour emoji
(option 1 in the issue) on 2026-11-03. #124 is a bug fix. One release covers all three.

Where this text and the mockup disagree on behaviour, this text wins; on look, the mockup wins.

## 1. Why

- **Misclicks (#125).** Subagent lines sit flush under their session row, two lines tall, and look
  like part of it. A click just below the branch line opens an agent's panel instead of showing
  the terminal. With many subagents the list also floods the sidebar.
- **Alarming highlight (#125).** The project you are in is a tinted, outlined box in the theme
  accent. On Sumi the accent is red, so it reads as an error.
- **Empty box in the title bar (#124).** Claude Code starts its window title with a status
  symbol. Kaze strips only `✳ ✻ ✽ ⏺` and braille, so `◐` (U+25D0 to U+25D3 spinner frames)
  stays in the title, and the title bar's UI font draws it as an empty box.
- **Broken-looking emoji (#127).** Kaze draws emoji in one colour. On Windows, `✅` comes from
  Segoe UI Symbol's thin outline and `🔄` from Segoe UI Emoji's monochrome outlines, so
  Claude's status tables look wrong next to Windows Terminal.

## 2. Goals, non-goals, anti-goals

Goals: the four fixes above, matching the approved mockup, on Windows and macOS.

Non-goals:
- Colour emoji on Linux (Noto's CBDT bitmaps). Linux keeps today's monochrome path.
- COLRv1 gradients and transforms. Segoe UI Emoji's COLRv0 layers are enough.
- Emoji sequences that need shaping: ZWJ families, skin-tone modifiers, flags. Each code point
  in a cell is drawn on its own, as today.

Anti-goals (a failure even if it works):
- Any per-frame cost on unchanged frames. The sidebar keeps zero allocations when nothing
  changed (`TestSidebarCache*`). Emoji colour is decided once per glyph and cached.
- New timers or idle wakes (hover tooltips, folds).
- A stray click that opens a panel when you meant the session.

## 3. #125 Sidebar agent tray

Mockup row "Combined: B + chip". Behaviour:

- **Tray layout.**
  - The tray sits under its session with a small gap above (about 3 dp) and is indented, with a
    thin tree line on the left (about 30 dp from the row's left edge).
  - Each agent takes **one line, about 22 dp**: state mark (the existing still ring, amber dot or
    green dot), type in magenta, task, elapsed time, and a small **`›`** button at the end.
  - The current step (today's second line) moves to a **hover tooltip** on the line. The tooltip
    is drawn only while the pointer rests on the line and asks for no frames when idle.
- **Safe click.**
  - A primary click anywhere on an agent line **except `›`** does exactly what clicking its
    session row does: it shows that session, with workspace members handled as today.
  - **`›`** opens the agent's panel, with today's showAgent toggle semantics.
  - A **double-click** on the line also opens the panel.
  - The line whose panel is open stays highlighted.
- **Cap and order.**
  - At most **3 lines** per session, ordered waiting first, then running, then done, newest first
    within each.
  - Past 3, a chip follows: **"+N more · R running, D done ⌄"**, which counts only the hidden
    agents and says "W need you" first, in amber, when any hidden agent waits.
  - Clicking the chip expands the tray to **at most 8 agent lines**, followed by one footer line,
    **"Show fewer"**, at any count (builder's call, flagged for the owner). Past 8 the footer reads
    **"Show fewer · N more on the board"**: "Show fewer" collapses back to 3, and "on the board"
    opens the agents board filtered to the session. The footer is not one of the 8 lines.
  - The hover tooltip shows as soon as the pointer is on the line, with no delay, because a delay
    would need a timer.
  - The expanded state is per session and is not saved across restarts.
- **Fold.**
  - The row's agent count becomes **"N agents ⌄"** when the tray is open and **"N agents ›"**
    when it is folded. Clicking it folds or unfolds the whole tray.
  - The fold is per session and remembered for the session's life (not saved to disk). The tray
    is **open by default**.
  - A folded session with a waiting agent shows the count in amber.
- **Unchanged.** The "needs you" toast and the board still open panels directly, and docked panel
  behaviour (0.24) does not change.
- Today's "+N more" line that opens the board is replaced by the chip above.

## 4. #125 Project rail

Mockup row "The project you are in", option 2:

- Remove the tinted fill and outline from the active project's header (`activeProjectColors`,
  `stroke`).
- Draw a **2 dp accent rail** at the sidebar's left inset. It runs from the top of the active
  project's header to the bottom of its last row, including the agent trays, and is clipped to the
  list's visible area.
- The project name is drawn bright (`ActiveFG`) and bold. The count stays muted, or amber for
  "N waiting".
- This applies in all five themes. The selected session keeps its own left bar. The "folded group
  names the shown session" rule is unchanged.

## 5. #125 Contrast

Sidebar text must be at least **4.5:1** on the sidebar background in all five themes. Sumi's
`Muted` `#77736b` on `Chrome` `#0c0c0c` is 4.14:1. Fix it in the theme or with a local nudge
(`render.Readable`), and add a test over the five themes for the tray's muted text, the chip and
the tooltip.

## 6. #124 Leading status symbols

- **One shared helper.** It strips a leading run of status-like runes plus the spaces after them:
  Unicode So and Sm symbols, geometric shapes U+25A0 to U+25FF, dingbats U+2700 to U+27BF, braille
  U+2800 to U+28FF, misc technical U+2300 to U+23FF, and the existing `✳ ✻ ✽ ⏺`.
- **Never empty.** If stripping would leave nothing, keep the original.
- **Used everywhere.** The sidebar's `ruled` title, the title-bar crumb and the resource card
  (`dropStatusMark`) all use it.
- **No empty boxes.** Chrome text drawn in the UI fonts (`sans`, `label`, `fitSide`) must never
  draw a missing-glyph box. Runes no loaded UI face can draw are dropped, the way `fit` drops `⎇`.
- **Tests:** `◐ Epic`, `⠂ Epic`, `✳ Epic`, `✳ ◐ Epic`, a plain `Epic`, and a title that is only
  symbols.

## 7. #127 Colour emoji

- **Which runes.**
  - Emoji-presentation code points (Unicode Emoji_Presentation, for example `✅ ❌ 🔄 🟢 ⏳ 🚀`)
    go to the colour emoji face first. On Windows that means **before** Segoe UI Symbol, so `✅`
    uses Segoe UI Emoji.
  - **U+FE0F is out of scope (builder's call, flagged for the owner).** The screen drops
    zero-width runes (`internal/screen/print.go`), so text-default emoji such as `⚠️ ✔️ ❤️` can't
    be told apart from their text form. They are drawn from the emoji font's outlines in one
    colour, at their normal width, so they look cleaner than Segoe UI Symbol's. Keeping U+FE0F
    (and deciding 1 or 2 cells) is a follow-up issue.
  - Other symbols keep today's order.
- **Windows (Segoe UI Emoji, COLR v0 + CPAL).**
  - Read the COLR base glyph and layer records and CPAL palette 0. For a base glyph, rasterize each
    layer glyph's outline (via `sfnt.LoadGlyph` by glyph index) as a coverage mask and composite
    it in its palette colour into an RGBA image of the cell span (usually 2 cells), centred and
    scaled like other glyphs.
  - Palette index 0xFFFF ("text colour") uses the theme FG at draw time, or, the builder's call,
    a fixed neutral, recorded in the audit.
  - A glyph with no COLR record falls back to today's monochrome path.
- **macOS (Apple Color Emoji, sbix).** Pick the strike nearest to the cell height and decode its
  PNG with `image/png`. Scale it with a good-quality filter (`golang.org/x/image/draw`, already a
  dependency) into the cell span.
- **Cache.**
  - The glyph cache gains an optional colour image beside the mask. Colour glyphs are copied
    as-is, alpha-blended over the cell background, and ignore FG.
  - Selection and inverse video still draw their background behind the emoji.
  - The cache cap and size eviction stay as they are, and the cap is measured in glyphs. Record
    the extra memory for 200 distinct emoji in the audit.
- **Bounds.** Table parsing is bounds-checked: a malformed font disables colour for that font
  and falls back to monochrome, never panics. The sbix PNG decode is limited to the strike's
  declared size (no unbounded decode).
- **Measure.** Keypress-to-glyph and a streaming benchmark with emoji-heavy output, before and
  after, using the existing bench mode. No change on frames whose glyphs are already cached.

## 8. Constraints

- Strict TDD with bounded RED tests, no global input, and tests on both platforms where code is
  platform-specific (the darwin sbix path runs in macOS CI).
- Commits `type(scope): Verb ...`, no AI attribution, branch and PR, the owner merges, then one
  release (0.25).

## 9. Left to the builder

- Exact dp sizes, following the mockup.
- Whether COLR parsing lives in `internal/render` or a small new package.
- The tooltip's look, matching existing chrome.
- How the double-click is detected (Gio's NumClicks).
