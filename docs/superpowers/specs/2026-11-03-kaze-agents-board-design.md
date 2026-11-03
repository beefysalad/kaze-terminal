# Agents board redesign and sidebar follow-ups

Status: approved by the owner on 2026-11-03 from the clickable mockup
`docs/design/agents-board-mockup.html` ("looks good, please implement it now"). The sidebar highlight
choice comes from `docs/design/sidebar-active-highlight-mockup.html` (option 3, "Session bar only").
Where this text and the mockup disagree on behaviour, this text wins; on look, the mockup wins.

## 1. Why

- **The highlight looks bad.** 0.25's project rail sits right next to the selected session's left bar,
  so you get two parallel accent lines. On Sumi the accent is red, which reads as an error.
- **Safe click went too far.** Since 0.25 an agent line only shows its session, and the panel opens
  only from a small `›` that is easy to miss. The owner wants a plain click on an agent line to open
  its panel again. The slim tray with its gap is enough protection against misclicks.
- **The board is hard to use.**
  - Its three status columns (Running, Waiting on you, Done) leave "Waiting" empty most of the time.
  - Cards aren't grouped by the session they belong to.
  - Twenty agents become one long scroll, and nothing can be read without leaving the board.

## 2. Sidebar

- **Remove the project rail** (0.25's `hits.rail` and its drawing). The active project's header keeps
  its bright, bold name, with no fill, outline or rail. The selected session keeps its own 2 dp accent
  left bar and its row background, as before 0.23.
- **A click on an agent line opens its panel.** A second click on the same line closes it. This is the
  0.24 behaviour: `showAgent(tab, k, toggle=true)`.
  - Remove the `›` button and its hit area.
  - Remove the double-click rule and the `lastAgent` record.
  - Clicking the session row still shows the session.
  - The tray (3 lines, "+N more", fold, tooltip) is unchanged.
- **"+N more"** opens the board **filtered to that session** (section 3). It replaces 0.25's in-place
  expand to 8 lines and the "Show fewer" footer. Remove that footer and the expanded state.

## 3. The agents board

Opened as today: the Agents button, the status-bar pill, the palette, or `Ctrl+Shift+A` /
`Cmd+Shift+A`. It replaces the terminal area, as today. Two panes:

### 3.1 List (left)

- **Header.**
  - The title "Agents" with a summary, "11 agents in 3 sessions".
  - "Back to <session>" on the right, which closes the board as today.
- **Filter.**
  - A segmented control: **All, Needs you, Running, Done**. Each segment shows its count, and the
    Needs you count is amber when non-zero.
  - Beside it, a **search** field that matches type, task, session title and repo, case-insensitive.
    `/` focuses search when the board has the keyboard. `Esc` clears a non-empty search, otherwise it
    closes the board.
  - Opening the board from a session's "+N more" sets a **session filter**: a removable "<session> ×"
    token beside the search. It replaces today's "Showing X only. Show all".
- **Needs you card.** Pinned at the top under All, only when any agent is waiting. It has an amber tint
  and border. Each row adds the session title before its step, so you know where the agent is.
- **One card per session** that has matching agents, in sidebar order. Its header shows:
  - the agent logo and the session title;
  - the repo and branch, muted;
  - "Go to session ›", which shows the session and closes the board.
- **State meter** (the signature), under each card header. A 3 dp bar split in proportion into needs
  (amber), running (accent) and done (muted grey). It counts all of the session's agents, regardless of
  the filter.
- **Rows.**
  - Columns: state mark, type in magenta (fixed width), task, and elapsed time right-aligned ("took Nm"
    when done). Under the task, a second, quieter line with the current step (or the report's first
    line when done).
  - Order: running first, then done; waiting agents sit in the Needs you card under All.
  - Hover and selection get a tint; the selected row also gets a 2 dp accent left bar.
- **Finished agents fold** per session: "✓ N finished ›" expands them in place, and "Hide finished"
  folds them back. They are always shown under the Done filter.
  - The fold state lasts while Kaze runs and is not saved.
  - Keep today's ways to dismiss or clear finished agents (the × on a done row on hover, and "Clear"),
    placed in the card header or on the row. Builder's call.
- **Empty state:** "No agents match" plus "Try another filter, or clear the search." When there are no
  agents at all: "No subagents yet" plus a short line on how they appear. Keep today's hooks-missing hint.

### 3.2 Preview (right)

- It shows the selected agent:
  - an eyebrow with the state label (NEEDS YOU / RUNNING / DONE in its colour), the type pill and the
    elapsed time;
  - the task as a title;
  - "In <session> · <repo>";
  - the actions **Open in session** (primary), plus, when done, **Copy report** and **Open in reader**.
- **Body.**
  - Running or waiting: the Activity timeline (reuse the docked panel's timeline and markdown notes,
    `dockTimeline` / `mdLayout`). Waiting shows the approval box.
  - Done: the rendered report (reuse the panel's report layout).
  - The preview reads the transcript only while it is shown, with the same caps and polling rules as
    the panel (no polling when nothing can change).
- **Selection.**
  - Nothing selected: "Pick an agent" plus "Its steps, messages and report show here."
  - On opening, the first waiting agent is selected, if any.
  - A click selects a row. A double-click, or Open in session, shows the session with that agent's
    panel open (`showAgent` without toggle) and closes the board.
  - `↑`/`↓` move the selection when the board has the keyboard and the search field is not focused.
    `Enter` does Open in session.
- In a narrow window (under about 900 dp) the preview is hidden and a click opens the agent in its
  session, as today.

## 4. Look

Follow the mockup within Kaze's drawing:

- **Surfaces:** layered, with hairline borders. Cards are raised with a 10 dp radius.
- **Type:** Manrope for the title, labels, buttons and the filter; JetBrains Mono for agent data.
  Section labels in caps, small and letterspaced.
- **Colour:** all colours come from the theme (Chrome, Active, Border, Muted, Accent, Needs, Done), with
  local `render.Readable` nudges. Text is at least 4.5:1 in all five themes, including amber on the
  Needs you card's tint. Add contrast tests.
- **No timers or animation.**

## 5. Constraints

- Lean:
  - The board draws only while shown.
  - No allocations on unchanged frames where the current board already avoids them; measure board
    frames with 50 agents.
  - The sidebar keeps zero allocations on unchanged frames.
- Strict TDD with bounded RED tests and no global input. Commits `type(scope): Verb ...`, no AI
  attribution. Branch `feat/agents-board`, PR, the owner merges, then release 0.26.
- Update README (Subagents, the board) and HANDOFF, and add an audit note with captures in Sumi and
  Midnight.

## 6. Left to the builder

- Exact sizes and paddings, following the mockup.
- Where dismiss and clear live.
- How the search editor is wired (reuse the palette's or filter's editor pattern).
- File layout (e.g. split `board.go` into `board_list.go` / `board_preview.go`).
