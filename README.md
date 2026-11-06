<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/branding/kaze-logo.svg">
    <img src="assets/branding/kaze-logo-light.svg" alt="kaze by Patrick" width="420">
  </picture>
</p>

<p align="center">
  <strong>風 · "wind"</strong><br>
  <em>Light, fast, and out of the way.</em>
</p>

kaze is my personal terminal: a small, fast terminal with a session sidebar, written in Go with
[Gio](https://gioui.org). I built it for working in [Claude Code](https://claude.com/claude-code)
all day, so it does a few things well (a sidebar of sessions, fast redraws, Japanese text, Claude Code's
symbols and colour emoji) and leaves out most of what a full terminal suite has. It has a small settings page for
the everyday choices, and everything it changes lives in one config file you can also edit by hand.

## Why "kaze"

Kaze (風) is Japanese for "wind". That is the feel I want from a terminal: light, quick, and
never in the way.

- **Light:** kaze draws nothing when nothing changes, so an idle window uses no CPU (0.0% with
  four tabs open) and its memory stays flat.
- **Fast:** only the rows that changed are redrawn. Replaying a Claude Code reply as it streams,
  99% of frames took 5.1ms or less to draw, well inside the 16.7ms a 60Hz screen allows.
- **Out of the way:** a sidebar of sessions, a nudge when a Claude Code session needs you or is done, and not
  much else.

These numbers were measured on Windows; the details are in
[`docs/benchmarks/2026-10-28-milestone-1.md`](docs/benchmarks/2026-10-28-milestone-1.md).

## Status

- **Windows:** works. This is what I use every day (Windows 11, PowerShell 7).
- **macOS:** works, and I use it too (Apple Silicon, zsh). Build it as an app with
  `./tools/package-macos.sh`. Still missing on the Mac: a Dock bounce for alerts, an Option-as-Alt toggle, Japanese file names with dakuten
  (macOS stores them decomposed and kaze drops the combining mark), and benchmark numbers.

## Requirements

- Go 1.27.0 or newer (see `go.mod`).
- macOS only: the Xcode Command Line Tools, because Gio uses cgo there:

  ```
  xcode-select --install
  ```

Windows needs nothing else; the Windows build uses no cgo.

## Install

### Download

Get the latest build from [Releases](https://github.com/beefysalad/kaze-terminal/releases/latest):
`Kaze-macOS.zip` (macOS 13 or newer, Apple Silicon and Intel), `Kaze-Windows-x64.zip` or
`Kaze-Windows-arm64.zip` (Windows 10 and 11).

kaze isn't signed by Apple or Microsoft yet, so the first launch needs one extra step:

- **macOS:** unzip, drag Kaze into Applications and open it. When macOS says it can't verify the
  app, click Done, then click **Open Anyway** next to Kaze in System Settings > Privacy & Security.
- **Windows:** unzip and run `kaze.exe`. If SmartScreen warns, click More info, then Run anyway.
  Smart App Control may block it with no way past; build it from source then.

### With Go

With Go installed, this puts a `kaze` binary in your Go bin folder (`go env GOPATH`, then `bin`):

```
go install github.com/beefysalad/kaze-terminal/cmd/kaze@latest
```

On Windows, add the GUI flag so no console window opens next to kaze:

```
go install -ldflags "-H=windowsgui" github.com/beefysalad/kaze-terminal/cmd/kaze@latest
```

## Build and run from a clone

```
git clone https://github.com/beefysalad/kaze-terminal.git
cd kaze-terminal
```

Windows (`-H=windowsgui` stops a console window from opening next to kaze):

```
go build -ldflags "-H=windowsgui" -o kaze.exe ./cmd/kaze
.\kaze.exe
```

macOS:

```
go build -o kaze ./cmd/kaze
./kaze
```

For the Kaze icon in the Dock, build and open the macOS app bundle instead:

```
./tools/package-macos.sh
open dist/Kaze.app
```

The app bundle is written to `dist/Kaze.app`. A bare `./kaze` binary still works, but macOS gives it a generic executable icon.

The shell is PowerShell 7 (`pwsh`) on Windows, falling back to Windows PowerShell, and your
`$SHELL` (or `/bin/zsh`) as a login shell on macOS. Set `shell` in the config to use another;
if it fails to start, kaze falls back to the default shell and says so in the first tab. If no
shell can start at all, kaze shows the error in a message box on Windows (on macOS it prints
it to stderr) and exits.

On Windows the kaze icon (exe file, taskbar, title bar, Alt+Tab) and the version details come
from `cmd/kaze/rsrc_windows_*.syso`, which `go build` picks up by itself. They are generated
from `cmd/kaze/winres/winres.json` and `assets/branding/kaze-icon-512.png`. After changing the
icon or the version, regenerate them from `cmd/kaze`. `TestWinresMatchesVersion` fails when `winres.json` and `internal/version` disagree:

```
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --out rsrc --arch amd64,arm64
```

### Smart App Control (Windows)

With Smart App Control on, Windows sometimes blocks a fresh unsigned build ("An Application
Control policy has blocked this file"). Rebuild so the binary gets a different hash, for example:

```
go build -ldflags "-H=windowsgui -s -w -X main.r=12345" -o kaze.exe ./cmd/kaze
```

Change the number and rebuild until the binary is allowed. The same thing can happen to test
binaries; re-run `go test` with `-ldflags "-X main.r=<random>"`.

## Emoji

Emoji such as `✅ ❌ 🔄 🟢 ⏳ 🚀` are drawn in colour, from Segoe UI Emoji on Windows and Apple
Color Emoji on macOS, so Claude Code's status tables look the way they do in other terminals.
Each emoji is drawn once and then reused, so colour costs nothing on later frames. Symbols that
are text by default (`⚠ ✔ ❤ ⏺`) stay one colour; Claude's `✳` spinner keeps the terminal font.
Not yet: the emoji form selected by U+FE0F (`⚠️`), joined sequences such as families, skin tones
and flags, and colour emoji on Linux.

## Shortcuts

| Action | Windows | macOS |
|---|---|---|
| New session | Ctrl+Shift+T | Cmd+T |
| Close session | Ctrl+Shift+W | Cmd+W |
| Next session | Ctrl+Tab | Ctrl+Tab |
| Previous session | Ctrl+Shift+Tab | Ctrl+Shift+Tab |
| Hide or show the sidebar | Ctrl+Shift+B | Cmd+B |
| Jump to the newest alert card, or the next session that needs you | Ctrl+Shift+J | Cmd+J |
| Open the palette | Ctrl+Shift+K | Cmd+K |
| Show or hide the agents board | Ctrl+Shift+A | Cmd+Shift+A |
| Copy the selection | Ctrl+C (with a selection) or Ctrl+Shift+C | Cmd+C |
| Paste | Ctrl+V or Ctrl+Shift+V | Cmd+V |
| Find in the terminal or the docked tab with the keyboard | Ctrl+Shift+F | Cmd+F |
| Open settings | Ctrl+, | Cmd+, |
| New line in Claude Code (without sending) | Shift+Enter or Alt+Enter | Shift+Enter or Option+Enter |

Any of these can be changed in Settings > Shortcuts: click a shortcut, press the new keys, and it is
saved at once. If the keys are already used, Kaze asks before taking them. Keys with no Ctrl, Alt or
Cmd are refused, except the F-keys (F1 to F12), which work on their own. A plain Ctrl+letter works but warns you, because the shell loses it. You can reset one
row or all of them. The same changes can be made by hand under `[keys]` in `config.toml`. Writing
`none` leaves an action without a shortcut (it is still in the palette). Settings always keeps one,
so it cannot be set to `none`.

Closing the final session keeps Kaze open. Start again with **New session**, the title bar
button or your configured shortcut. Settings and Workspaces stay available. Exiting the final
plain shell shows the same empty state; stopped workspace members keep their output.
Use the window close button or the system Quit command to exit Kaze.

Session switching follows sidebar order, skips workspace members, and works while a docked
member has the keyboard too. If a busy terminal cannot accept more input, Kaze shows
"Input not sent"; wait for it to respond before trying again.

Pasting with an image on the clipboard (a screenshot, say) puts it into Claude Code or Codex:
kaze sends them the key they read images with (Ctrl+V; on Windows, Alt+V for Claude Code). A
plain shell never gets it.

On Windows, Ctrl+C and Ctrl+V work like Warp: Ctrl+C copies when text is selected and clears the
selection, so the next Ctrl+C interrupts; with nothing selected it goes to the shell as an
interrupt. Typing clears a selection, so an old one never turns Ctrl+C into a copy. Ctrl+V always
pastes, so it can no longer send a literal Ctrl+V to a program (such as Vim's block select). When
the clipboard holds both text and an image, the text is pasted. On macOS, plain Ctrl+C always goes to the shell. Shift+Enter sends the same sequence as
Alt+Enter (Option+Enter on macOS), adding a new line in Claude Code while plain Enter submits.
Other terminal programs handle that sequence according to their own keybindings. The session, copy,
paste, find, sidebar, next-waiting, palette, agents board and settings shortcuts can be changed under `[keys]` in the
config. Plain Ctrl+F on Windows still goes to the shell (and to programs such as Vim or less). Settings, Shortcuts lists every shortcut as this machine has it.

## Mouse

- **Select:** drag with the left button. Copy with the copy shortcut, or turn on
  `copy_on_select` to copy when you let go. Dragging past the top or bottom edge scrolls.
- **Double-click** selects a word, **triple-click** a line. Keep dragging after the click to
  extend by words or lines.
- **Apps that use the mouse** (vim, htop and the like) get the clicks and the wheel. Hold
  **Shift** while dragging to select text in them anyway.
- **Links:** `http` and `https` links are drawn in blue with an underline (a link a program
  colored keeps its color) and show a hand cursor. Cmd+click (Ctrl+click on Windows) opens
  one in your browser, even while Claude Code or vim has the mouse. A link that wraps onto the
  next row opens whole. `.md` paths use the same styling; Cmd+click (Ctrl+click on Windows)
  opens them inside kaze.
- **Wheel:** scrolls back through history. Typing jumps back to the bottom. Full-screen apps
  that don't use the mouse have no history to scroll.

## Find

Press **Ctrl+Shift+F** (**Cmd+F** on macOS) to search the terminal that has the keyboard: the
session's terminal, or a workspace member's tab in the panel. A small bar opens at the top right
of that pane. In a full-screen program (vim, htop) it searches the program's screen.

- **Type** to search. Every match is tinted in the text, the current one stronger, and the bar
  says "3 of 12". It searches the whole scrollback, not only what you see. It ignores case
  unless you turn on **Aa**. There is no regex.
- **Enter** or **Down** jumps to the next match, **Shift+Enter** or **Up** to the previous one,
  and both wrap around. The buttons in the bar do the same.
- **A match can wrap.** A word the terminal split across two rows is still found.
- **Marks on the right edge** show where every match sits in the whole scrollback, with the
  current one brighter and a faint band for the part you can see.
- **New output** updates the count without moving the view. The view moves only when you change
  the text or step to another match.
- **Esc** closes the bar and puts the view back where it was. Opening it again keeps your last
  text, already selected, so typing replaces it. Esc in the terminal itself still goes to the
  shell.
- **A click in the terminal** gives the keyboard back to the terminal and leaves the bar and the
  highlights open. A click in the bar's field takes the keyboard again.
- **Limits:** at most 1,000 matches are kept (the newest, nearest the live screen), and the count
  then reads "N of 1000+". Find costs nothing while the bar is closed.

## The sidebar

Sessions are listed down the left side, in place of a tab strip.

- **Groups:** sessions in the same git repo sit under one header, and worktrees count as the
  same repo. The project you are in is marked by a thin accent rail down the left edge, from its
  header to its last row and agent tray, with its name in bold, alongside the selected session's
  highlight. That project context stays visible
  in Settings, documents and while a docked member has the keyboard. Groups start open. Click a header to fold or unfold
  it. A folded header shows an amber "N waiting" when a session in it needs you.
- **Rows:** two lines: state dot, agent logo and name with status on the right;
  branch and `+N −M` below. Linked worktrees show their folder and branch unless
  the folder already ends in the branch's slug. Outside a repo, the second line
  shows the shortened folder. Hover for ×, or middle-click to close.
  - **Agent:** Claude and Codex have their own logos. Hooks or foreground-program
    detection identify the agent. Codex hooks need `codex --no-daemon` for now.
  - **Status:** `needs you`, `done`, the active subagent count, then `running`, in
    that order. A session with subagents shows its tray's fold control (`N agents ⌄`) there
    instead, and its dot keeps the state. Running uses a closed ring with a bright core, drawn still
    without animation frames.
  - **Name:** your Kaze rename, then the last slash command's first argument,
    then the active agent's title without its leading status symbol (a spinner frame such as
    `✳` or `◐`). Otherwise Kaze
    uses a folder name that differs from the repo, or the program's name.
    Linked worktree folder names appear only on the second line. Command arguments
    keep their name after the agent exits; old agent titles do not.
  - **Rename:** double-click the title. Enter saves, Esc cancels, and blank
    returns to the name rule. Workspace members keep their fixed names.
  - **Line counts:** `+N −M` compares the folder with `HEAD`, on prompt/hook
    events, with a two-second background Git limit and no index refresh.
- **Filter:** the "Filter sessions…" field at the top. Click it to type. Typing narrows the list
  by title, folder, branch and command, and folded groups open while you filter. Enter shows the first
  match (on an empty field it only gives the keyboard back). Esc clears the field and gives the
  keyboard back to the terminal. Switching or opening a session, opening Settings or hiding the
  sidebar also gives the keyboard back and keeps the text.
- **Next waiting:** Cmd+J / Ctrl+Shift+J and its palette action open the newest alert
  card's session while cards are showing. Without cards they jump to the next session
  that needs you, wrapping around. With none, the status bar says "Nothing is waiting on you."
- **Workspaces:** a section at the bottom lists each running workspace (see Workspaces below).
- **Hiding it:** the sidebar toggle (Ctrl+Shift+B, Cmd+B on macOS) hides or shows it, and kaze
  remembers the choice as `sidebar` in the config. Below 560 dp of window width it also hides
  by itself, without saving that, and the toggle does nothing until the window is wider.
- **Title bar:** Kaze draws one title bar, laid out the same on macOS and Windows. On the left,
  over the sidebar: the window buttons on macOS (the real traffic lights) or the K mark on Windows,
  the sidebar toggle and **+** for a new session. Then the session you're in (repo, title, command
  and its state), a search field that opens the palette, and on the right Agents
  (with a badge) and Settings; on Windows, minimize, maximize and close follow. Drag any
  empty part of it to move the window; double-click it to zoom (macOS follows your setting) or
  maximize. Hiding the sidebar leaves the title bar's buttons where they are.

## The status bar

Along the bottom, left to right: the folder and branch of the session in front (with its lines
added and removed), then an **Update** pill when a newer kaze is out, the running subagents
(amber when one waits on you), workspace dots with how many members are up, CPU and memory
for Kaze and its sessions, Claude usage as small meters, the tab count and, on a laptop, the
battery. Clicking the update, agents, workspace dots or Claude usage
opens what they point at. In a narrow window the least important parts give way first: the
battery, then the tab count, then the resource label and meter, then other labels, so nothing
overlaps.

**Battery (Windows and macOS):** at the far right, a small battery that fills to the level and
the percent. A green bolt shows while it charges below 100% (a full battery shows no bolt, even
plugged in). It turns amber at 20% or less and red at 10% or less. A desktop shows nothing, and
Kaze stops asking after the first check. Kaze hears about changes from the OS instead of polling
(Windows power-setting notifications; on macOS the system's battery percent and power source
alerts); if that is not available it reads at most once a minute, only on frames that are drawn
anyway. Nothing is read while the window is minimised, or in bench mode. It is not clickable and
has no tooltip.

Hover **CPU · memory** for **Kaze resource usage**: Kaze itself first, followed by
session totals for its observable child processes, including shells, agents and
local servers. Workspace members have their own stable rows, labeled by workspace
and member; opening a member as a tab under another session does not change
ownership. Observed processes still alive after a tab closes
keep a row marked closed. Unassigned Kaze children appear separately. External
shared services and remote processes are excluded; processes that detach before
Kaze first observes them may be missed.

CPU is a share of the whole machine, with 100% meaning all logical cores are busy.
The card names the core count. Amber starts at 50%, red at 80%, based on the last
three intervals. RAM has no meter or warning threshold. It is summed physical
footprint on macOS, including compressed memory; on Windows it is private
committed bytes, which can include paged-out memory. MB means 1,048,576 bytes and
GB means 1,024 MB. GPU is not measured. Unknown readings show **n/a**, and partial
totals have an asterisk with unavailable counts in the card. New processes warm
up for one CPU reading without making healthy totals partial.

Collection runs in the background on existing window activity, at most every two
seconds. There is no new polling timer. The card shows sample age; a completely
idle window does not wake just to refresh resources. CPU averages the time since
the preceding native counter read, even across idle gaps; the card shows that
interval. Title changes and other tabs opening or closing keep unaffected
process baselines. Sampling pauses while
minimized and resets CPU baselines on return. The card scrolls for many sessions;
in very short windows its whole content scrolls so the details remain reachable.

## Workspace members in the panel

A workspace member opens as a tab in the panel docked under your session's terminal, the same
panel subagents use, so you can watch a server's log without leaving your Claude session. Click
its line in the sidebar, the server dots in the status bar, or pick it in the palette.

- **One panel per session:** each session has its own panel with its own tabs. One tab shows at
  a time; click a tab to switch. Members and subagents share the tab row and the panel's height
  (drag the line above it, as for subagents).
- **Up to 6 tabs** per session. A seventh replaces the oldest tab that isn't showing. Long names
  get shorter tabs first, then are cut with "…".
- **×** on a tab closes it; the member keeps running. **Hide ⌄** (or a second click on the shown
  tab, or on its line in the sidebar) folds the panel and keeps its tabs.
- **Expand** makes the member the main view; pick any session to go back.
- **Keyboard:** opening a member gives it the keyboard, and so does clicking it. A thin accent
  line marks whichever terminal has the keyboard while a member tab shows. Every key goes to the
  member, Esc and Ctrl+C included; kaze's own shortcuts still work. Click your session's terminal
  to take the keyboard back.
- **Size:** the member's shell gets the panel's size while its tab is open, even when the panel
  is hidden, so hiding doesn't reflow its output. Closing the tab or Expand gives it the shared
  size again.
- **One session at a time:** opening a member under another session moves its tab there.
- A member that has stopped keeps its tab, with a red dot and its last output. To start it again,
  run the workspace from the sidebar.
- Tabs live while kaze runs; they are not kept across restarts.

Members no longer float over the session, and the scratch shell is gone. An old `scratch_shell`
line under `[keys]` in `config.toml` is ignored, with a warning in the first tab.

## Subagents

When Claude Code hands work to subagents (Explore, Plan, general-purpose or your own agents), or
Codex starts helpers, kaze shows what each one is doing. Hooks provide their state. While a Claude
subagent's panel is shown, kaze also reads its transcript for live messages.

Several agents of one type started together each keep their own name and task. When the order is
in doubt, kaze matches each agent to its own prompt (the first message of its transcript, read at
most 3 times and 64 KiB at a time, and only when two agents could be mixed up). A wrong early
guess swaps; a name that has settled never moves.

- **In the sidebar:** each session's subagents sit in a slim tray under its row, behind a thin
  tree line. Each agent is one line: its state mark, type, task and how long it has run. A dot
  inside a ring means running, an amber dot means it is waiting for your approval, a green dot
  means done. Hover a line to see the step it is on now (for example `Grep authMiddleware in src`).
  Finished ones leave the list at your next prompt.
  - **Click a line** to open that agent's tab in the panel; click it again to hide the panel.
  - **Three lines, then a chip:** a tray shows at most 3 agents, waiting ones first. The rest are
    summed up in a chip such as "+17 more · 15 running, 2 done ⌄", with "N need you" in amber when
    a hidden agent waits. When the sidebar is narrow the chip keeps "+N more" and its `⌄` and drops
    the counts first. Click it to open the board filtered to that session.
  - **Fold:** the row's "N agents ⌄" folds the whole tray ("N agents ›" brings it back). The count
    turns amber when a folded tray holds an agent waiting on you.
- **Agent panel:** click a subagent to open it as a tab in the panel docked under the terminal
  (click it again to hide the panel). Nothing covers the terminal; only that session's terminal gets shorter. Drag the line
  between them to any height; Kaze remembers it (`agent_panel_height` in `config.toml`). Under
  the tab row, a line shows the state, type, task and time, an **Activity / Report** switch and,
  once done, **Copy report**; **Open in reader** sits at the right of the tab row. The prompt sits folded in one line; click it to read it all.
  - **Activity** reads like Claude Code's own transcript view. The agent's messages lead, with the
    coral bar and their time. Each step is one sentence: a shell command uses Claude's own
    description in the past tense ("Ran go vet and the ui tests") with **passed**, **failed · N
    tests** or **done**; an edit says "Edited `<path>` +N −M"; a commit says "Committed "<subject>""
    with its short hash. Runs of small steps fold ("Read 3 files", "Searched the code 4 times"),
    and housekeeping tools such as ToolSearch, Monitor and TaskStop are left out. Click a step to
    open its diff, or the command and the last lines of its output. It follows new rows unless you
    scroll up. A waiting agent shows what it wants to run; a finished one says "Finished in Nm ·
    read the report".
  - **Readable | Every step:** the switch at the top of Activity. **Every step** shows the full
    list, each tool step with its time, tool and target. The choice is for the whole window (the
    panel and the board preview) and is not saved; Kaze starts in Readable. Codex helpers have no
    transcript, so their steps come from the hook summaries.
  - **Report** shows the report as formatted markdown (bold, code, lists, headings, tables). The
    panel switches to it when the agent finishes, unless you picked a tab yourself. **Copy report**
    copies the original markdown.
  - Each session remembers its open panel: switch away and back and it is still there. A "needs
    you" card's **Open session** opens the waiting subagent's tab.
  - Each subagent you open gets a tab in the panel, next to any workspace members (see Workspace
    members in the panel). Click a tab to switch, × to close it, **Hide ⌄** to fold the panel and
    keep its tabs.
  - The keyboard stays with your session, so Esc still reaches Claude. Click into the panel first;
    then Esc hides it and ↑/↓ move to the previous or next subagent.
  - Cmd+click (Ctrl+click on Windows) a web link or `.md` path in the panel to open it. Missing
    transcripts and Codex helpers show the tool steps without notes.
- **The board:** every subagent in every session, as a list of session cards with a preview
  beside it. Open it with the Agents button in the sidebar, the "N agents running" text in the
  status bar, "Show agents" in the palette, or Ctrl+Shift+A (Cmd+Shift+A on macOS). Esc or
  "Back to ..." returns you.
  - **Filter and search:** All, Needs you, Running and Done, each with its count. The search
    matches type, task, session title and repo; `/` jumps to it and Esc clears it (Esc on an empty
    search closes the board). Opened from a session's "+N more", the board shows only that
    session, with a "<session> ×" token to show all again.
  - **Cards:** agents waiting on you sit in an amber "Needs you" card on top. Then one card per
    session, in sidebar order, with its repo and branch, "Go to session ›" and a thin bar that
    splits its agents into waiting, running and done. Each row shows the type, task, time and the
    current step (or the report's first line once done). Finished agents fold into
    "✓ N finished ›"; "Hide finished" folds them back. A done row's × removes it, and **Clear**
    removes a session's finished agents.
  - **Preview:** click a row to see that agent on the right: its state, task and session, its
    Activity (readable, as in the panel, with the same Readable | Every step switch) or its report, and **Open in session** (plus **Copy report** and **Open in
    reader** when done). The first agent waiting on you is picked when the board opens.
  - **Keys:** ↑/↓ move the selection, Enter or a double-click opens the agent in its session with
    its panel. In a narrow window there is no preview, and a click opens the agent in its session.
- **Setup:** it needs three more Claude Code hooks (SubagentStart, SubagentStop and a PreToolUse
  hook). Settings > Agents lists them in the setup check, and **Fix** adds them. In the main
  conversation the tool hook exits at once unless Claude starts a subagent or the call touches a `.md` file.

## Reading docs

Cmd+click an `.md` path in terminal output (Ctrl+click on Windows) to read it in kaze.
Relative paths use that terminal's folder, then its Git repo root.
Links to `.md` files the agent wrote or read anywhere (a vault, another repo) open too: kaze
checks the files the agent touched in that tab before the folder, and other tabs' files last.

- **Spaces:** a path with single spaces in it, such as
  `C:/Users/you/Documents/My Notes/plan.md`, is underlined whole. So is a path in quotes.
- **Folder printed earlier:** when a line names a folder and then a `.md` file in it, the click
  tries that folder plus the file name.
- **On a click** kaze tries the longest candidate first, then shorter ones, then the terminal's
  folder and Git root, and opens the first that exists. If none does, it says
  "Can't find plan.md in <folder>", naming the last folder it tried.
- Links set with OSC 8 (hidden link targets) are not read yet.

Each doc gets a temporary row under its session. Click a heading in the outline to
jump there, or scroll with the wheel, arrows, Page Up/Down and Home/End. The outline
hides in narrow windows. Click the session row to return to its terminal.

Links inside a doc need only a plain click: another `.md` opens under the same session,
heading links jump to that section, and web links open in your browser. Other file
and URL types do nothing. This is a read-only view; closing a row never changes its file.

The shown doc reloads when its file changes, keeping your scroll position. Reading is
capped at 2 MB, with a notice for longer files. A removed file keeps its last loaded
content. Close a row with its ×, middle click, or the session close shortcut
(Cmd+W on macOS, Ctrl+Shift+W on Windows). Closing its session closes its docs too.
Doc rows are not saved or restored after restart, and do not count as terminal tabs.

## The palette

One key finds and starts things: Ctrl+Shift+K (Cmd+K on macOS), or the search field in the
middle of the title bar. It opens on:

- **Waiting on you:** sessions where Claude is asking for something.
- **Sessions:** every other session and workspace member, with its state. Enter shows it.
- **Recent skills:** up to 3 skills you started lately, then a "Browse all N skills" row.

Type to narrow the list. The text can match anywhere in a name or description. Typing also finds
Kaze's own actions (and "Run workspace" for each saved workspace), plus up to 3 matching skills.
Servers are not listed here; use the status bar or the sidebar for those.

To see every skill, type `/`, press Tab, or click "Skills" on the switch beside the box. Skills
are grouped as **Yours**, **This project**, then one group per plugin. Each group shows 4 and ends
with a row like "N more in plugin-name" that opens the rest. Tab or the switch goes back to the
short list. On a narrow window the switch hides, and Tab still works.

Up and Down move, Enter acts, Esc or a click outside closes. On a skill:

- **Enter** types the command into the session in front, without pressing Enter for you, when
  that session runs Claude Code and is not waiting on a permission. You add arguments and send it.
- Otherwise, or with **Shift+Enter**, kaze opens a new session in the same folder and runs
  `claude '/the-skill'` there once its shell shows a prompt. If no prompt marker arrives within
  3 seconds, the tab asks first (Run commands / Use shell); kaze never sends on a timer alone.
  PowerShell, zsh, bash and sh are supported; other shells get a message instead.

Skills come from `~/.claude/skills/*/SKILL.md`, `~/.claude/commands/**/*.md`, the same folders
under the current session's repo (`.claude/`, which win over yours), and enabled plugins in
`~/.claude/settings.json` (newest installed version only, shown as `plugin:name`). kaze only
reads these files, when the palette opens, and rereads only the ones that changed. The last 3
skills you started show under Recent while kaze runs.

## Settings

Open it with the gear button in the title bar or with Ctrl+, (Cmd+, on macOS). While it is open, a pinned Settings row sits at the top of the
sidebar, and it closes with that row's ×. Its menu on the left has:

- **Appearance:** theme cards (each a small terminal in that theme's colors), a live preview,
  font size, line spacing, cursor shape and blinking.
- **Workspaces:** saved launch recipes, one card each with its state, plus Start folder, startup and session restoration choices (see Workspaces below).
- **Agents:** shared alert choices (chimes, taskbar flash and done-toast duration), Claude usage,
  and separate setup checks for Claude Code and Codex.
- **Shortcuts:** every shortcut with its keys and config name, grouped and searchable, and kaze's
  command line (`kaze --workspace`, `kaze --version`, and the hook commands). Read-only for now.
- **Updates:** whether a newer kaze is out, and the update itself (see Updates below).
- **About:** the logo, version, platform and license.

A change applies right away and is saved when you make it. kaze edits only that one key in
`config.toml` and leaves every other line and your comments as they are, so a hand-edited file
stays yours.

### Themes

| Theme | Look |
|---|---|
| Midnight | The original: soft dark blue-black with a violet accent and rounded tabs. The default. |
| Sumi | Ink-black with warm paper text and a vermilion accent; tabs get an underline. |
| Tide | Deep navy with a turquoise accent; pill-shaped tabs. |
| Washi | Light: warm paper background, dark ink text, indigo accent. |
| Sakura | Dark plum with soft pink accents. |
| Matcha | Olive-black with a soft green accent. |
| Kōyō | Autumn leaves: brown-black with a maple orange accent. |
| Hai | Ash grey with muted colors, for low glare. |
| Fuji | Wisteria: deep violet-black with a lilac accent. |
| Kiri | Light: cool, foggy grey-blue with a steel blue accent. |

A theme sets the terminal colors and the window colors together. Colors you set under `[colors]`
still win over the theme's terminal palette.

### Claude Code setup check

The Agents section reads `~/.claude/settings.json` and shows the state of the entries kaze
needs: the Stop hook (`notify done`), the Notification hook (`notify needs`), the prompt hook
(`notify prompt`, shown as "Command each session runs"), the status line (`statusline`), the
three subagent hooks (`notify agent-start`, `notify agent-stop` and `notify tool`, see Subagents)
and the SessionStart hook (`notify session`, which names the agent in the sidebar as soon as it opens):

| State | Meaning |
|---|---|
| OK | The entry is there and the kaze exe it points to exists. |
| missing | No kaze entry. |
| broken | A kaze entry whose exe is gone (for example after moving kaze). |
| taken | The status line is another program's. kaze never overwrites it by itself. |

**Fix** adds missing entries and repoints broken ones; **Replace** swaps another program's status
line for kaze's. Both show exactly what will change in a confirm dialog first, and write only after
you confirm. Before writing, kaze copies the file to `settings.json.bak-<date>-<time>`, and it
touches no other entry. If the file is empty or not valid JSON, kaze reports that and offers no fix.

### Codex setup check

Settings > Agents also checks Codex's `~/.codex/hooks.json` and reads inline hooks
in `~/.codex/config.toml`. If the `.codex` directory is absent, it shows **Codex
not found**. It checks these handlers:

| Codex event | Command |
|---|---|
| PermissionRequest | `kaze notify needs --agent codex` |
| Stop | `kaze notify done --agent codex` |
| UserPromptSubmit | `kaze notify prompt --agent codex` |
| Interrupt | `kaze notify interrupt --agent codex` |
| SubagentStart | `kaze notify agent-start --agent codex` |
| SubagentStop | `kaze notify agent-stop --agent codex` |
| PreToolUse | `kaze notify tool --agent codex` |
| SessionStart | `kaze notify session --agent codex` |

SessionStart shows Codex on its sidebar row as soon as it opens. SubagentStart, SubagentStop and PreToolUse show Codex helpers in the sidebar, live panes and board, like Claude Code
subagents (see Subagents). A helper is named after its `spawn_agent` task, and a helper that
gets a follow-up turn shows as running again.

**Fix** previews the changes and asks before writing. It adds missing handlers
beside other programs' hooks and repairs broken Kaze handlers. An existing file
is copied to `hooks.json.bak-<date>-<time>` first. If either source changed while
the preview was open, nothing is written. A working inline hook is left in place;
inline problems and duplicate Kaze handlers require manual repair so Kaze does
not silently install a second copy. Empty or malformed hook files are not fixed.

After Fix, **open Codex and run `/hooks` to review and trust Kaze's hooks**. The
settings check verifies configuration and executable paths, not hook trust. The
reminder stays until dismissed or Kaze restarts. Kaze never changes Codex's
`notify` command, writes its `config.toml`, or approves a Codex permission request.

Session rows and the palette show `claude` or `codex` before the submitted command.
Codex commands can start with `/` or `$` (for skills). Codex's Interrupt hook clears
`running` without a done notification; Claude's existing interrupt limitation
still applies. The shared usage switch shows **Claude** usage only. The palette's
skill catalog and skill launcher remain for Claude Code.

Codex Stop and SubagentStop print `{}` and always exit successfully, including outside Kaze.
Other hook commands are silent. The hook transport, prompt, Stop and Interrupt lifecycles were checked
with Codex CLI 0.166.0 on macOS; the helper hooks follow that version's source. Windows hook
execution still needs a manual check.

## About

Open Settings with Ctrl+, (Cmd+, on macOS), then choose **About** for the Kaze logo, version, platform and license. **Copy build info** copies the version, build and platform for a bug report; **View on GitHub** opens the repository and **Report an issue** opens a new issue.

### Updates

kaze asks GitHub for the latest release 10 seconds after it starts and then once a day while it
runs. The request carries nothing about you beyond a `kaze/<version>` user agent, and a failed check
stays quiet. Turn **Check for updates automatically** off in Settings, Updates (or set `check_updates = false`) and kaze sends
no request at all.

- When a newer version is out, the status bar shows an **Update 0.12** pill and the
  Updates menu item gets a badge. Click either for the Updates page, which says "Kaze 0.12 is
  available" with **Update and restart** and **What's new**.
- **Update and restart** asks first ("Kaze will close your 3 sessions and restart in 0.12."), then
  downloads the release's zip for your computer, checks it against the release's `SHA256SUMS`, swaps
  the new kaze in and restarts it. With session restoration enabled, conversations and folders
  reopen automatically; running commands stop. If the swap fails, the old kaze is put back. The copy it replaced
  is removed the next time kaze starts.
- It only replaces a kaze from a release download: `Kaze.app` (macOS) or `kaze.exe` (Windows),
  in a folder kaze can write to. A kaze built from a clone shows **Download**, which opens the
  release page, and the reason it can't update in place.

## Workspaces

A workspace is a saved launch recipe: a name and a list of tabs. Each tab has a title, a folder,
zero or more command lines, a color and something to wait for first. Running a workspace opens
its tabs in one go, each in its folder with its commands typed for you, so a project with an
API, a front end and a log tail starts with one click.

Make one under Settings > Workspaces: **+ New workspace**, then add tabs. The editor has two
panes: the workspace's tabs on the left (color dot, title, how many commands; a red dot marks a
tab with a problem) and the selected tab on the right. **+ Add tab** adds one in the next unused
color. For each tab:

- **Title** and **folder.** **Choose…** opens the system folder picker (Finder's on macOS,
  Explorer's on Windows) and fills in an empty title from the folder's name. You can also type or
  paste the path; folders under your home folder show as `~/...`.
- **Commands:** one numbered line per row, run in that order.
- **Color:** cyan, magenta, yellow, green, blue or red. It colors the tab's dot.
- **Wait for:** Nothing, Docker, or a port. The tab's commands are held until Docker is up, or
  until something answers on that port on this machine.

The Workspaces list shows each workspace's tabs with their color, folder, commands in order and
what they wait for. Nothing is saved until you press Save, and Cancel drops the draft. **Run** opens the tabs after
the ones you already have. If any of the workspace's tabs is still open, the button reads
**Go to tabs** and switches to the first one instead of opening copies.

### Running workspaces in the sidebar

A running workspace gets a header in the Workspaces section at the bottom of the sidebar,
reading "all up", "N ready" (muted, when some members' commands finished) or "N down" (red,
which wins). Click the header to fold it.

- Each member gets one line: its color bar, its title and its status. A member reads `Ready`
  (muted, not down) when its commands finished and its shell is still open, such as after
  `docker compose up -d` or activating a venv. It reads `stopped` in red when its shell exited
  or was closed, and `Failed` in red when its run failed. A member's line shows an amber dot when it needs you and a green dot when it is done;
  a folded header shows the amber one if any member needs you, else the green one. Click a
  member to open it as a tab under your session (see Workspace members in the panel above);
  click it again to hide the panel. Its line is highlighted while its tab shows.
- The Workspaces list in Settings shows how far each member got: Starting, Waiting for Docker
  (or a port), Confirm to run, Running, Finished, Failed, Cancelled or No commands. Finished means
  the shell came back to its prompt (it needs the snippet from "Folder and branch in the status
  bar"); kaze can't tell whether the commands worked.
- Running a workspace keeps your current session in front.
- The status bar shows one dot per workspace member (green running, red down, muted otherwise)
  and a count such as "1/2 up". A closed member shows stopped and counts as down while the
  workspace still has an open tab. Click the dots to open the member that is down as a tab, or the first open member
  if none is.
- Ctrl+Tab and Ctrl+Shift+Tab step through sessions in sidebar order and skip workspace members.
- Close a member with your close-session shortcut (Ctrl+Shift+W, Cmd+W on macOS), the × that
  shows when you hover its line, or a middle click on the line. Only a workspace run makes these lines; you can't drag ordinary sessions into one.

### How commands are sent

A tab's commands are typed as one line, then Enter. In PowerShell the lines are joined as
`a; if ($?) { b }; if ($?) { c }`, and in zsh, bash and sh as `{ a; } && { b; } && { c; }`, so a line runs only
if the one before it worked. It runs in the tab's own shell, so a venv activation or `cd` stays in
effect, the shell stays open afterwards, and the exact line shows in the shell's history. If your
shell isn't pwsh, powershell, zsh, bash or sh and any tab of the workspace has commands, kaze
refuses the whole run and shows a status-bar message naming that tab; nothing opens. (At startup
the message goes into the usual first tab instead.)

kaze types only after the shell has shown its first prompt. It knows that from the folder report
described under "Folder and branch in the status bar" below, so add that snippet to your profile
for hands-off launching. Without it, after 3 seconds the tab shows a banner with **Run commands**
and **Use shell**: the first types the commands anyway, the second leaves the tab to you. Typing in
a tab that is still waiting cancels its commands. A workspace waiting in a minimized window starts
its commands when the window is shown again.

When a tab waits for Docker, kaze starts Docker Desktop once if it isn't running. Docker and port
waits give up after 3 minutes with a message in that tab, and the commands are not run.

### The workspaces file

Workspaces are stored in `workspaces.toml` next to `config.toml`. kaze owns its layout and
rewrites it when you save, so comments are not kept. If you edit it by hand, kaze checks it
strictly when it loads; a file with a problem is reported, and kaze never overwrites it, so fix or
delete it before saving from the settings page.

### Starting kaze with a workspace

Pick one under Settings > Workspaces (`startup_workspace` in `config.toml`), or run
`kaze --workspace "My project"`. Either way kaze opens only that workspace's tabs, without the
usual plain tab. An explicit `--workspace` wins over session restoration and the startup
workspace setting. Otherwise, saved sessions restore first; the startup workspace runs when
there is nothing to restore. An unknown name opens a normal tab and shows a warning.

### Start folder for plain tabs

Under **Settings > Workspaces > Startup**, use **Start folder > Choose…** to pick the
folder for the first ordinary session and every new plain tab (Cmd+T on macOS,
Ctrl+Shift+T on Windows). **Use home** clears the override. The current path is shown
next to the controls; choosing a folder applies to subsequent tabs immediately.

You can also set `start_dir = "~/Documents"` in `config.toml`. Empty means your home
folder. `~` expands to home; relative paths resolve against the folder Kaze was
launched from. A missing folder or a path to a file falls back to home with a warning.
Changing this setting never moves existing tabs. Workspace tabs and restored sessions
keep their own folders, and the existing startup-workspace/restore precedence stays
in effect.

### Known limits

- No folder Browse button yet; type the path.
- Commands can't be reordered, only added and removed.
- Closing the window drops a workspace draft you haven't saved.

## Session restoration

Kaze automatically restores the previous sessions when you reopen it, including after
**Update and restart**. It keeps their folders, custom names, order, colors and selected
session. Claude Code and Codex reopen the exact conversations reported by their Kaze hooks;
ordinary terminals start fresh shells. Multiple conversations in one repo stay separate.
Make sure the agent hooks in Settings > Agents are configured and, for Codex, trusted.
Process detection can show an agent logo without hooks, but cannot identify its conversation.

Restoring a conversation waits for its shell's first prompt marker. Set up the shell prompt
report described above. If no marker arrives within three seconds, the session offers
**Run commands** or **Use shell**; typing first cancels automatic resumption. Kaze never
resubmits the previous prompt or replays commands. Missing folders, tools or conversation
files leave a shell and an explanation. A failed resume does not select a different chat.
The ready shell finds Claude or Codex using its own PATH, including paths set in shell startup
files when Kaze opens from Finder or restarts after an update. Missing commands and resume
errors appear in that tab as shell or agent output. The allowlisted agent command
is unquoted so shell aliases, functions and PATH installs all resolve normally.

Running commands, servers, shell variables, scrollback, panel tabs and
live helpers do not survive exit. Workspace members reopen without replaying their recipe
commands. Kaze saves small metadata snapshots on changes, including while minimized, so it
can recover the last saved set after a crash. Changes in the last 250 ms may be lost.

Turn off **Restore sessions on launch** under Settings > Workspaces > Startup to clear the
snapshot and start fresh. Deliberately closed sessions stay closed. `sessions.json` lives
beside `config.toml`, with a limit of 128 sessions and 1 MiB; exceeded limits show a warning
and preserve the last valid snapshot. A second Kaze instance opens fresh and does not
replace the first instance's saved sessions.

## Config

kaze reads one TOML file at startup:

- Windows: `%AppData%\kaze\config.toml`
- macOS: `~/Library/Application Support/kaze/config.toml`

The file is optional. The settings page writes to it for you; restart kaze after editing it by hand.
A bad value falls back to its default and shows up as a warning in the first tab; a TOML syntax
error makes kaze use all defaults.

| Key | What it does | Default |
|---|---|---|
| `font_size` | Font size, 4 to 72 | `14` |
| `shell` | Program and arguments, e.g. `["pwsh", "-NoLogo"]` | PowerShell 7 / `$SHELL` |
| `scrollback_lines` | Lines of history per tab, up to 100000 | `10000` |
| `copy_on_select` | Copy a selection when the mouse button is released | `false` |
| `chime_needs` | Chime when Claude Code needs you (see Alerts) | `true` |
| `chime_done` | Chime when Claude Code finishes its turn | `false` |
| `chime_file` | Your own chime, a WAV file up to 2MB; `~` means your home folder | the bundled chime |
| `theme` | `midnight`, `sumi`, `tide`, `washi`, `sakura`, `matcha`, `koyo`, `hai`, `fuji` or `kiri` | `midnight` |
| `line_spacing` | `tight`, `normal` or `roomy` | `normal` |
| `cursor_style` | `block`, `bar` or `underline` | `block` |
| `cursor_blink` | Blink the cursor (off keeps idle CPU at zero) | `false` |
| `taskbar_flash` | Flash the taskbar button on alerts (Windows) | `true` |
| `done_toast` | How long a "done" alert card stays: `3s`, `6s`, `10s` or `until_seen` | `6s` |
| `show_usage` | Show the Claude usage readout in the status bar | `true` |
| `check_updates` | Ask GitHub once a day whether a newer kaze is out (see Updates) | `true` |
| `start_dir` | Start folder for fresh plain sessions and new plain tabs; `~` expands to home | empty, home folder |
| `restore_sessions` | Restore folders and exact Claude/Codex conversations on launch | `true` |
| `startup_workspace` | The workspace to run when kaze starts (chosen in Settings > Workspaces, stored as its ID) | none, a normal tab |
| `[colors]` | `fg`, `bg`, `cursor`, `selection`, `color0` to `color15`, as `"#rrggbb"` | the built-in Midnight palette |
| `sidebar` | Show the session sidebar (the toggle saves it here) | `true` |
| `[keys]` | `new_tab`, `close_tab`, `next_tab`, `prev_tab`, `copy`, `paste`, `find`, `toggle_sidebar`, `next_waiting`, `palette`, `agents_board`, `open_settings` (old `scratch_shell` and `new_branch_session` lines are ignored with a warning) | the shortcuts above; `none` removes one |
| `font_family` | **Not supported yet.** Setting it only adds a warning; kaze always uses its bundled font. | |

Shortcuts are written like `"ctrl+shift+t"`. Modifiers: `ctrl`, `shift`, `alt` (or `option`),
`cmd`, `super` (or `win`). Keys: a letter or symbol, `tab`, `enter`, `space`, `esc`,
`backspace`, `delete`, `pageup`, `pagedown`, `home`, `end`, the arrows (`up`, `down`, `left`,
`right`) and `f1` to `f12`.

Example:

```toml
font_size = 15
shell = ["pwsh", "-NoLogo"]
scrollback_lines = 20000
copy_on_select = true
chime_done = true
chime_file = "~/sounds/ding.wav"

[colors]
bg = "#0f1117"

[keys]
new_tab = "ctrl+t"
```

kaze sets `TERM=xterm-256color`, `COLORTERM=truecolor` and `TERM_PROGRAM=kaze` for its shells,
plus `KAZE_TAB_ID` with the tab's number, and `KAZE_NOTIFY` (the local address kaze listens on
for alerts) with `KAZE_NOTIFY_TOKEN` (a random secret for that kaze run). Apps you launch from a
kaze tab inherit these variables too, the same way they inherit `PATH`.

## Optional shell setup

History suggestions and Tab completion come from your shell, not from kaze, so kaze leaves your
shell configuration alone. Here is how to turn them on.

### PowerShell (Windows)

PowerShell's own prediction and menu completion work well in kaze. To turn them on, add this
to your PowerShell profile (`notepad $PROFILE`):

```powershell
if ($Host.Name -eq 'ConsoleHost' -and -not [Console]::IsOutputRedirected) {
    Set-PSReadLineOption -PredictionSource History -PredictionViewStyle ListView
    Set-PSReadLineKeyHandler -Key Tab -Function MenuComplete
}
```

The guard keeps it quiet in scripts and other non-interactive sessions.

### zsh (macOS)

kaze starts your login shell, which is zsh on macOS (`/bin/zsh -l`). Tab completion is built into
zsh. Gray suggestions from your history need one plugin,
[zsh-autosuggestions](https://github.com/zsh-users/zsh-autosuggestions), which you install with
[Homebrew](https://brew.sh):

```sh
brew install zsh-autosuggestions
```

Then add this to the end of `~/.zshrc`:

```zsh
# Tab completion, with a menu you can move through with the arrow keys
autoload -Uz compinit && compinit
zstyle ':completion:*' menu select

# Gray suggestions from your history as you type; Right arrow or End accepts one
source "$(brew --prefix)/share/zsh-autosuggestions/zsh-autosuggestions.zsh"
```

If you use [Oh My Zsh](https://ohmyz.sh), it already runs `compinit`. Skip the `autoload` line,
keep the `zstyle` line, and either keep the `source` line or install the plugin the Oh My Zsh way:

```sh
git clone https://github.com/zsh-users/zsh-autosuggestions ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-autosuggestions
```

then add it to the plugin list in `~/.zshrc`, for example `plugins=(git zsh-autosuggestions)`.

To check it, open a new kaze tab (existing tabs keep the old settings):

1. Type the start of a command you ran before, such as `git st`. The rest (`atus`) appears in
   gray after the cursor. Press Right arrow to accept it.
2. Type `ls /Libr` and press Tab. zsh completes it to `/Library/`. Press Tab again to list what is
   inside, and once more to highlight an entry; move with the arrow keys and press Enter to pick.

The suggestion uses color 8 (bright black) from the active theme, so it stays dim but readable in
every kaze theme, including the light Washi.

## Folder and branch in the status bar

kaze shows the active tab's folder and git branch (`~\src\kaze  ⎇ main`) on the left of the
status bar. The shell has to tell kaze where it is with an OSC 7 sequence from its prompt.
The branch is read from `.git/HEAD` (no git process runs) whenever the prompt reports, when you
switch tabs, and when the kaze window comes back into focus, so a branch that Claude Code
switched shows up too.

PowerShell, in your profile:

```powershell
if (-not $global:__kazePrompt) { $global:__kazePrompt = $function:prompt }
function prompt {
    $segs = (Get-Location).ProviderPath.Split('\') | ForEach-Object {
        if ($_ -match '^[A-Za-z]:$') { $_ } else { [uri]::EscapeDataString($_) }
    }
    [Console]::Write([char]27 + ']7;file://' + $env:COMPUTERNAME + '/' + ($segs -join '/') + [char]7)
    & $global:__kazePrompt
}
```

It percent-encodes each folder name, so names with spaces, `#`, `%` or Japanese come through
whatever the console code page is. Loading it twice is safe. It works in PowerShell 7 and 5.1.

zsh, in `~/.zshrc`:

```zsh
_kaze_osc7() { printf '\e]7;file://%s%s\a' "$HOST" "${${${PWD//\%/%25}//\#/%23}// /%20}" }
autoload -Uz add-zsh-hook; add-zsh-hook precmd _kaze_osc7
```

Without this the folder and branch stay hidden. UNC paths (`\\server\share`) are not shown.

## Alerts (Claude Code hooks)

kaze marks a tab and shows an alert card when Claude Code in that tab needs you (a permission
prompt or a question) or has finished its turn.

- **Where:** at the bottom right, above the status bar, or above the agent panel when it is
  open. The newest card sits nearest the corner. At most 3 show, one per session; in a short
  window the oldest are left out.
- **What a card shows:** the Claude, Codex or workspace mark with a `!` (needs you) or `✓` (done)
  badge; a small line with the branch and the session's slash command (`⎇ feature/x · /review-pr`);
  the session's name as it reads in the sidebar; two lines of text, cut with "…"; and a status
  line: "Needs your permission", "Waiting for your answer" or "Done · ran 6m 40s".
- **The text:** for "needs you" it is Claude's message. For "done" it is the start of Claude's
  last reply (Codex too), so you can see what it finished without switching.
- **Open session** (or a click anywhere on the card) switches to that session. Ctrl+Shift+J
  (Cmd+J on macOS) opens the newest card's session; its key caps show on that card.
- **×** closes a card and keeps the session's dot in the sidebar. "Needs you" cards stay until
  you visit the session; "done" cards hide after `done_toast`.
- A workspace run's summary uses the same card, with the workspace mark.

"Needs you" plays a chime (unless you're already looking at that tab); "done" is silent
unless you turn on `chime_done`. While kaze is minimized, its taskbar button flashes (Windows).

kaze learns about this from Claude Code hooks that run `kaze notify`. Add them to your Claude
Code settings (`~/.claude/settings.json`), with the full path to your kaze binary:

```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input",
        "hooks": [
          { "type": "command", "command": "\"C:\\Users\\you\\kaze\\kaze.exe\" notify needs", "timeout": 3 }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "\"C:\\Users\\you\\kaze\\kaze.exe\" notify done", "timeout": 3 }
        ]
      }
    ]
  }
}
```

If you already have hooks, add these entries next to them. On macOS the command is
`"/path/to/kaze" notify needs` (and `notify done`).

- **The prompt hook** is a third entry, under `UserPromptSubmit`, that runs
  `kaze.exe notify prompt` (timeout 3). It sends kaze the slash command that starts your prompt
  (like `/work-ticket`) and its first line of arguments, so the sidebar can show the command and
  mark the session as running. A prompt that does not start with a slash command sends only
  "running". **Fix** in Settings > Claude > Claude Code setup adds it, with the usual
  show-the-change, confirm and back-up steps, so you don't need to edit the file by hand. Claude
  Code adds a prompt hook's output to the conversation, so this one prints nothing at all. Its
  text is cleaned of control characters and capped before kaze shows it.
- The hooks do nothing outside kaze: `kaze notify` looks for the kaze variables, finds none
  and exits quietly, so Claude Code in another terminal is unaffected.
- `kaze notify` (including `notify prompt`) always exits 0 and prints nothing, so a hook can never block or slow Claude Code.
- Even without the matcher, kaze ignores the "idle for a minute" and "login finished"
  notifications.
- **Windows with Smart App Control:** after each rebuild, run `kaze.exe notify done` once by hand.
  That lets Smart App Control check the new exe right away, instead of during a hook, where the
  check can take longer than the 3 second timeout.

## Claude usage in the status bar

kaze shows Claude's 5-hour and weekly plan usage and how full the current tab's context is, on the
right of the status bar: a small meter and the percentage for each of 5h, wk and ctx. A meter
turns amber at 80% and red at 95%. Hover the usage for a card with each limit and when it resets;
click it for Settings, Agents.

The numbers come from Claude Code's status line. Point it at `kaze statusline` in
`~/.claude/settings.json`:

```json
{
  "statusLine": { "type": "command", "command": "\"C:\\Users\\you\\kaze\\kaze.exe\" statusline" }
}
```

On macOS the command is `"/path/to/kaze" statusline`. Claude Code allows one status line, so this
replaces any you have.

- `kaze statusline` prints the same line under Claude's prompt, so it is useful in other terminals
  too. Inside a kaze tab it also sends the numbers to kaze.
- The 5-hour and weekly numbers are for the whole account, so kaze shows the latest from any tab.
  Context is per session, so it shows only for the tab you are on, and goes away when Claude exits
  back to the shell prompt (this needs the folder snippet above).
- Plan usage appears only for Claude Pro and Max subscriptions, after the first reply in a session.
- It always exits 0 and finishes within about 300ms.

## Version

`kaze --version` prints the version and the commit and time it was built from, then exits without
opening a window. The settings page shows the version at the bottom.

`kaze --workspace "<name>"` starts kaze with that workspace's tabs (see Workspaces).

## Releases

Pushing a tag `v` + the version in `internal/version` (for example `v0.10`) runs the Release
workflow (`.github/workflows/release.yml`). It builds the universal Mac app
(`KAZE_UNIVERSAL=1 ./tools/package-macos.sh`) and the Windows exes for x64 and ARM, then publishes
them as a GitHub Release with the first-launch notes from `.github/release-notes.md`. A tag that
doesn't match the version fails before anything is published. Running the workflow by hand from
the Actions tab builds the files without releasing them.

```
git tag v0.10 && git push origin v0.10
```

## Benchmarks

Numbers for the first milestone on Windows (key latency, Claude Code streaming, launch,
scrolling, idle CPU, memory), including the goals it missed:
[docs/benchmarks/2026-10-28-milestone-1.md](docs/benchmarks/2026-10-28-milestone-1.md).
`bench/` has the harness to run them yourself.

## Roadmap

Done:

- Editable shortcuts: change any of them in Settings > Shortcuts, or write `none` to drop one (built on `feat/shortcuts-edit`, ships in 0.31 with the palette redesign).
- Find in the terminal and the docked tabs (#155): Ctrl+Shift+F or Cmd+F, every match marked, on the whole scrollback (built on `feat/find`, release next).
- 0.34: the “New session on a branch” launcher and its worktree cleanup removed; the session list runs down to the workspaces (PR #171).
- 0.33: the Kaze mark and name in the title bar on Windows and macOS; the Windows exe details carry the real version, name and publisher (PR #168).
- 0.32: five more themes, ten in all, and JetBrains Mono across the whole window (PR #165).
- 0.31: a cleaner palette with sessions first and skills behind / (PR #163); shortcuts editable in Settings (PR #162).
- 0.30: find in the terminal and docked tabs, Ctrl+Shift+F / Cmd+F (#155, PR #156).
- 0.29: the battery in the status bar on macOS (#139, PR #158); a compact About page built around the refreshed logo (PR #160).
- 0.28.2: a .md link Claude printed opens the file Claude wrote or read, wherever it lives (#143, PR #153).
- 0.28.1: the project moved to beefysalad/kaze-terminal; updates and the website follow it (PR #151).
- 0.28: readable agent activity in the panel and the board (#134); the battery in the status bar on Windows (#111, macOS in #139); subagents started together keep their own names (#135) (PR #140).
- 0.27: workspace members open as tabs in the panel under the terminal and the scratch shell and floating panes are gone (#133); richer alert cards at the bottom right (#9); .md links with spaces or an earlier folder open (#128) (PR #136).
- 0.26: an agents board you can work from, with filters, search, per-session cards and a live preview; one click on an agent line opens its panel and the project rail is gone (PR #131).
- 0.25: a slim, capped subagent tray with safe clicks, a rail for the project you’re in (#125), clean session titles (#124) and colour emoji on Windows and macOS (#127).
- 0.24: subagents open in a panel docked under the terminal with an Activity timeline and formatted reports (#106, #103); Ctrl+C/Ctrl+V copy and paste on Windows, screenshots included (#117); finished workspace setup tabs read Ready (#118).
- 0.23: active-project header emphasis, still ring-and-core running marks, a reusable empty window and Shift+Enter multiline input (#107, #113, #114).
- 0.21: read markdown docs inside kaze, follow live Claude subagent messages, recover empty reports and see a cleaner resource card.
- Plan 1: the terminal core with tabs.
- Plan 3: alerts with a chime when Claude Code needs you, kaze's own title bar on Windows, and
  the current folder and git branch in the status bar.
- Plan 4: Claude's 5-hour and weekly usage and the tab's context in the status bar.
- Plan 5a: the settings page (Appearance and Claude), five themes and the Claude Code setup check.
- Plan 5b: Workspaces (launch a saved set of tabs in one click).
- Plan 6a: the session sidebar, replacing the tab strip, with repo groups, a filter and the running workspaces.
- Plan 6b: agent awareness, titles you can rename, line counts and the Next waiting shortcut.
- 0.22: two-line session rows, retained sidebar drawing, a branch/worktree launcher and confirmed cleanup of Kaze-created worktrees (#101).
- Subagents: each session's subagents in the sidebar, a live pane per subagent and the agents board (replaces Plan 2's child tabs).
- 0.16: clickable links, Codex helpers alongside Claude Code subagents, and screenshot paste with Cmd+V.
- 0.17: Claude and Codex logos on the sidebar rows as soon as an agent opens, and links drawn in blue.
- Start folder (#77): choose where new plain tabs open in Settings > Workspaces > Startup.
- 0.18: automatically reopen folders and exact Claude/Codex conversations after restart, including updates.
- 0.17.1: kaze names Claude and Codex from the program running in the tab, so the Codex logo needs no setup.
- 0.20: CPU and RAM per session in the status bar (#92); resumed conversations after updates, the update prompt layout and closed workspace status fixed (#88, #89, #90).
- 0.19.1: restored macOS session-switching shortcuts, faster subagent text wrapping, fresh helper turns, background Git lookups and bounded input queues.
- 0.19: the subagent pane shows the prompt, a one-line summary and the report; a start folder for new tabs.

Next:

- Find in the agent panel and the doc reader, and a regex option for find.
- A redesign of the alert notification card (GitHub issue #9).
- Emoji follow-ups: keep U+FE0F so `⚠️` draws as an emoji (and decide its width), then joined
  emoji sequences.
- Plans 6d and 6e: floating panes and a scratch shell, then the command palette. (`kaze status`, Plan 6c, is parked.)
- Real panes, which would replace the workspace section of the sidebar as the way a workspace is shown.
- The Mac milestone: run, fix and benchmark kaze on macOS.

Later:

- A browser button.
- A graph view of my notes vault.

## License

kaze's code is under the [MIT License](LICENSE). The bundled fonts in `assets/fonts`
(JetBrains Mono and a subset of Noto Sans JP) are not covered by it: each keeps its own
SIL Open Font License 1.1, in `assets/fonts/OFL-JetBrainsMono.txt` and
`assets/fonts/OFL-NotoSansJP.txt`, with sources in `assets/fonts/SOURCES.txt`.
