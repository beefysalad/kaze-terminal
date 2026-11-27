# First-run tour, a calmer Workspaces list and Geist for the UI (design)

Date: 2026-11-26. Owner request: a short tutorial for friends who install Kaze, shown once, and a redesign of the
Settings Workspaces list, which gets crowded, and the UI font moves to Geist (the owner likes how herdr looks).
All three ship in **one PR** (owner ruling). Builder's calls are marked
**(call)**. Mockups (the owner's reference, private): https://claude.ai/artifact/Sr9HUr9nTQnNVyyRVvntVG, boards
"First-run tour" (Main) and "Settings: Workspaces" (Workspaces).

## Intent

The owner is sharing Kaze with friends. A friend who opens it sees a terminal and will not know why it is more than
the terminal they already use. The tour shows what Kaze adds around Claude Code and Codex, in about a minute, then
gets out of the way. Separately, the Workspaces page in Settings lists every workspace as a full card with every tab
and command, so with many workspaces it becomes a wall that truncates names and commands.

## Goals

- A friendly, skippable first-run tour that shows the real UI: spaces, the agents list, workspaces, the bottom bar,
  the Agents board, and turning on hooks.
- It shows **once**, on first install only. Nothing about it nags afterwards.
- It can be replayed on purpose (Settings and the command palette).
- The Workspaces list stays readable with 10+ workspaces and tabs that have several commands.

## Non-goals

- Not a heavy onboarding: no account, no questionnaire, no forced steps, nothing blocks using the shell.
- No tour for individual features after the first run, no tips, no what's-new popups.
- The workspace **editor** (draft with tab list and fields) is not redesigned in this PR.
- No telemetry on whether the tour was seen or finished.

## Anti-goals (a failure even if it technically works)

- Saying anything Kaze does not do. In particular no claim of agent-to-agent messaging or orchestration (not wired
  in, off by default), and no docked agents pane (it was replaced by the full-area Agents board).
- Showing the tour again after an upgrade, or to someone who has used Kaze for months.
- Wrong words. The product says **spaces**, never "sessions" (the config key `restore_sessions` is a name, not copy).
- Emojis or Unicode-symbol icons anywhere (repo rule). Fonts: Geist in the UI chrome, JetBrains Mono in the terminal,
  nothing else, and never mixed inside one piece of text.
- The tour opening on top of a running agent and swallowing keys the user meant for it.

## The tour

An overlay on the real window: a dimmed layer, a card, and a highlight ring on the real element being explained.
Chrome colors follow the active theme. Copy below is the mockup's, in the owner's words where they gave them.

| Card | Highlights | Says |
|---|---|---|
| Welcome | nothing | "Welcome to Kaze." Thank you for trying it. A quick look around, about a minute. Skip any time, shows once. Buttons: Skip tour, Show me around. **No product pitch** (owner: the welcome is a thank you). |
| 1 of 5 Spaces and agents | sidebar Spaces and Agents lists | Each project is a space with its own tabs. The Agents list sorts every Claude Code and Codex agent into Needs you, Working and Idle. Click one to jump to its tab. Kaze alerts you when something wants you. |
| 2 of 5 Workspaces | Workspaces list at the sidebar bottom | A workspace is a saved launch recipe: a few tabs, each in its folder with its commands typed for you. Make one in Settings, press Run. Running ones show here with status. |
| 3 of 5 Bottom bar | status bar | Folder and branch with lines changed, workspace members up, CPU and RAM for Kaze and what it runs, the model, then Claude usage: 5H is the 5 hour window, WK the week, CTX the context window of the tab in front. |
| 4 of 5 Agents board | the agents button, top right | Cmd+Shift+A (Ctrl+Shift+A on Windows) opens a full-area board of every subagent Claude Code and Codex start, by space, with what needs you first. Pick one to read its steps, messages and report. |
| 5 of 5 Live status (last) | none (dimmed) | Status, alerts and usage come from small hooks in Claude Code and Codex. Kaze can set them up. Nothing changes until you press Fix. Skip button reads "Do this later". |

- **Hooks card (kept last, owner decision).** It shows one line per agent with its real state: ready, or Fix. Fix
  reuses the existing Settings flow (`claudesetup.Fix` / the Codex equivalent through the same confirm dialog that
  lists exactly what will change). If an agent is not installed the line says so and offers nothing. Pressing Finish
  or Do this later never changes any file. **(call)** The card reads live state each time it is shown.
- **Step 4 (call):** the tour opens the Agents board while that card is up and returns to the previous view on Next,
  Back or Skip, so the card describes what is on screen. If a build cannot do that cleanly, ring the button instead.
- **Keys:** Esc skips, Enter or Right goes next, Left goes back. Mouse works everywhere. The tour takes keyboard
  focus only while it is up and hands it back to the terminal when it closes. **(call)**
- **Narrow windows:** the card anchors to the window and stays on screen; the ring is dropped if its target is hidden
  (for example the sidebar toggled off), and the card still reads fine without it. **(call)**
- Any other way of closing the card (window close, Skip, Finish) counts as seen.

## Once only

- A small state file `tour.json` beside `config.toml` and `sessions.json` (same directory, found the same way as
  `sessions.json`), holding `{"seen": true}`. **(call)** It is not in `config.toml`: users edit that file, and a
  settings toggle for "show the tour" would be one more thing to explain. Failing to read or write it must never
  show an error: if it cannot be written the tour simply may show again, and if it cannot be read it is treated as
  "do not show" so a broken profile never traps anyone in a tour. **(call)**
- First install means **none of `tour.json`, `config.toml` and `sessions.json` exist** when Kaze starts, checked
  before anything creates them. **(call)** If `tour.json` is absent but either of the others exists (an existing
  install upgrading), Kaze writes `seen` silently and shows nothing.
- `seen` is written when the tour closes by any route, and also the moment it opens, so a crash during the tour does
  not bring it back. **(call)**
- Not shown when Kaze starts in a mode where nobody is there to read it: bench mode (`KAZE_BENCH`), and when a
  workspace runs at startup or sessions are being restored. **(call)** In those cases `seen` stays false, so the
  tour appears the next plain launch. Open question for the builder: confirm this cannot starve a real first install.
- **Replay:** Settings (the About page, "Replay the tour") and a palette command "Show the tour". Replaying never
  touches `seen` and never offers the hooks Fix without the confirm dialog.

## Workspaces list

Replaces the per-workspace card in `listRows` and `memberRow` (`internal/ui/settings_workspaces.go`,
`settings_members.go`). Behaviour from the approved mockup:

- **Closed row, one line:** chevron, letter tile, name, "N tabs" (and "runs at startup" when set), one color dot per
  tab, state pill, Run (or "Go to tabs" when it is open or starting), and a "..." menu.
- **Open row:** click the row or chevron. The tabs appear under it: color bar, title, folder, first command, "+N more"
  when the tab has more, and the wait tag. Only one row is open at a time. **(call)** Edit and Delete are at the
  bottom of an open row.
- **"..." menu (call):** Edit, Delete, Run at startup. Delete keeps its existing confirmation.
- **Filter chips:** All, Open, Closed, with counts. "Open" means any state other than Closed.
- **Search (call):** a text box above the list, shown only with more than 5 workspaces, matching workspace name and
  tab titles.
- **Truncation:** names and folders truncate with an ellipsis and the full text is in the open row or the editor.
  Commands are never silently dropped: "+N more" is always a count of real ones.
- **Narrow pages:** keep today's `memberNarrow` behaviour for widths under 480 dp: the closed row drops the dots
  first, then the filter chips wrap.
- The Startup section below the list (start folder, "Restore sessions on launch", startup workspace) is unchanged.
- The editor, its drafts, Save and Cancel, and the loading error card are unchanged.

## UI font: Geist for the chrome, JetBrains Mono in the terminal

Owner decision (2026-11-26), after looking at herdr: its app draws the UI in Geist (13 px) and the terminal in
JetBrains Mono by default (`herdrdev/herdr-app`, `src/settings.rs`). Kaze does the same. This **replaces** the owner's
2026-11-05 rule of one font (JetBrains Mono everywhere), so `CLAUDE.md` changes in this PR too.

- **Terminal grid unchanged:** JetBrains Mono Regular and Bold, Noto Sans JP stays the Japanese fallback.
- **Chrome becomes Geist:** title bar, sidebar, status bar, tab strip, settings, palette, dialogs, toasts, cards, the
  tour and the new Workspaces list. Folders, branch names, commands and anything that is code or must line up in
  columns keep JetBrains Mono (the command chips in the Workspaces list, the usage numbers in the status bar are
  the likely cases). **(call)** Builder lists the exact split in the plan; rule of thumb: prose is Geist, code is mono.
- **Files:** Geist 1.x static `Geist-Regular.ttf`, `Geist-Medium.ttf`, `Geist-SemiBold.ttf`, `Geist-Bold.ttf` from
  `vercel/geist-font` (`fonts/Geist/ttf/`), SIL OFL 1.1, with its `OFL-Geist.txt` and an entry in
  `assets/fonts/SOURCES.txt`. Static files, so the Gio shader needs no variable-font support (herdr's file is
  variable; Kaze's loader takes Regular and Bold statics, `assets/fonts/fonts.go`, `newChrome` in `tabbar.go`).
  **(call)** Medium and SemiBold are only added if the chrome actually uses those weights (Settings headings use
  SemiBold today); otherwise Regular and Bold, to keep the binary small.
- **Code shape:** `chromeFace` ("JetBrains Mono") is today the one face for chrome and the terminal. It becomes two
  names, a chrome face (Geist) and the existing mono face, and every call site picks one on purpose. A render test
  guards that the terminal still uses mono.
- **Widths change:** Geist is proportional, so places that assume 8 px per character in the chrome (column math,
  truncation, the tab strip, the status bar segments, the palette, the sidebar rows) must measure text instead.
  This is the real cost of the change and the main risk; it is why it goes with the Workspaces list, whose
  columns are being rebuilt anyway.
- **CLAUDE.md (call):** replace the "One font: JetBrains Mono" bullet with: Geist for UI chrome, JetBrains Mono for
  the terminal and anything that is code; Regular and Bold (plus Medium or SemiBold if used) only; Noto Sans JP only
  as the Japanese fallback; do not add other fonts or mix two within one run of text. Owner set the new rule
  2026-11-26. The website lander keeps its own fonts.
- The mockup boards are drawn in JetBrains Mono; they are for layout only and will not be redrawn.

## Approaches considered

UI font: (1, chosen) Geist chrome with JetBrains Mono terminal, the herdr look; (2) Geist Mono for the chrome, which
keeps fixed widths but is not the look the owner liked; (3) stay with JetBrains Mono everywhere.

Tour:

1. **Overlay with highlights on the real UI (chosen, owner saw the mockup).** Shows the actual app, needs positions
   of a few real elements.
2. A full-screen slideshow with screenshots. Easy to build but screenshots go stale with every UI change.
3. A fake sample space the user plays in. Far more work, and it touches real data.

Workspaces list:

- **A. Collapsed rows (chosen, owner approved the mockup).** Scales with the number of workspaces and tab commands.
- B. Two panes (list left, tabs right). Matches the editor but costs list width on narrow windows.
- C. Compact cards. Smallest change but still grows without limit.

## What is left to the builder

- Reading element rectangles for the ring: the sidebar, status bar and agents button are drawn in `internal/ui`;
  find the least invasive way to get their rects without a second layout pass.
- The exact `tour.json` location code (reuse how `sessions.json` is placed from `cfgPath`).
- Whether the hooks card can call the Claude and Codex Fix flows directly or must open Settings, Agents; either is
  acceptable if the confirm dialog stays the only place a file is changed.
- Tests, in the repo's style (strict TDD, bounded): first-run detection (fresh, upgrade, unreadable file, read-only
  directory), `seen` written once, the replay path not writing it, list rows for 0, 1 and many workspaces, "+N more"
  counts, filter and search, and that no card copy contains "sessions" or an emoji.
- Font tests: the bundled Geist files parse, the chrome shaper resolves "Geist" at the weights used, the terminal
  grid still resolves "JetBrains Mono", and the CJK fallback still works from a Geist run.
- README: add the tour and the new list to the matching sections; mention the font change. Changelog and lander follow the usual release
  steps after the PR merges.

## Open for the owner

None blocking. The calls above are mine; change any of them while reading.
