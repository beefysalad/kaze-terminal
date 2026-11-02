# Kaze sidebar redesign: design

Date: 2026-11-02. Status: draft for the owner's approval.

## Why

The owner's words: "the clunkiness and repeatitiveness of some of the words". With two sessions open in
one repo, the sidebar showed "kaze-terminal" five times (group header, both row titles, both folder
paths), and the same branch and `+47` on both rows. The only thing that told the two rows apart was the
word "claude" vs "codex". The "Next waiting" button is "kinda useless" to the owner. The dots on group
headers repeat what the count already says.

What the owner needs from a row: **which branch or worktree it is on**, plus the session's name when it
has one.

The sidebar is also the main per-frame cost the 2026-11-02 optimization audit found: it rebuilds its
whole model (and re-lays out every row) on every frame, even when nothing changed. That is most of the
~15 KB and ~116 allocations an unchanged frame costs.

The new bottom section, a **branch launcher**, replaces "Next waiting": pick a repo, type a branch,
choose Claude, Codex or a shell, and kaze makes a sibling worktree, opens a session in it and starts the
agent there.

Approved mockups: `docs/design/sidebar-before-after-mockup.html` (rows, "After") and
`docs/design/sidebar-launcher-toggle-mockup.html` (launcher shown, picker open, tucked away). Open them in
a browser. The text below is the authority where they differ.

## Goals

1. Rows without repetition: two lines, the branch or worktree always visible, no folder path.
2. Group headers without dots.
3. The branch launcher at the bottom of the sidebar, collapsible with one click.
4. Kaze remembers the worktrees it creates and offers to clean them up, never silently.
5. An unchanged frame does not rebuild the sidebar model or re-measure its rows.

## Non-goals

- Changing the filter field, subagent lines under a row, the Workspaces section, renaming by double
  click, close ×, middle click, drag order or folding. They stay as they are.
- Removing the `next_waiting` action. Only the button goes; Cmd+J / Ctrl+Shift+J and the palette entry
  stay.
- Deleting branches. Cleanup removes worktree folders only.
- Touching worktrees kaze didn't create.
- Fetching from the network. Kaze never runs `git fetch`.
- Fixing session restore's `codex resume` line to add `--no-daemon`. Worth a separate small issue.

## Anti-goals

- Kaze deleting a folder without the owner's click. Never `git worktree remove --force`, never `rm`.
- An agent started in the wrong folder. If git fails, nothing starts.
- A launcher the owner can't get back without Settings (owner: "dont fully hide it to the point of me
  going to the settings just to show it again").
- Git commands or file system checks on the UI goroutine while drawing.

## The rows (decided with the owner, mockup "After")

### Group header

`▾ kaze-terminal ............ 2`, or `1 waiting` in amber instead of the count. **No dots** (owner: "we
already show a number so there is no need for the dots as well").

### Row line 1

State dot, agent logo (Claude ✳ / Codex mark, none for a plain shell), then the name, then a status on
the right.

The name is the first of:

1. the owner's kaze name for the session (rename),
2. the argument of the last slash command, as today's title rule uses it (`a.arg` in `agent.go`), so a
   `/rename foo` name keeps showing after the agent quits (my call: kept, it is how names survive today),
3. the title the agent set (Claude Code's conversation summary), with leading status marks
   (`✳`, braille spinner) removed, only while an agent is active in the tab,
4. the folder's name, only when it differs from the group's repo name and the session is **not** in a
   linked worktree (my call: line 2 already names the worktree, so repeating it here is the clutter this
   redesign removes),
5. the program: `codex`, `claude`, or the shell's name (`zsh`).

The words "claude" / "codex" and the magenta `/command` after the name no longer show (my call, matching
the approved mockup): the logo says which agent it is.

Right side, first that applies: `needs you` (amber), `done` (green), `N agents` while it has running
subagents, `running` (accent), nothing.

### Row line 2

The branch (green) and its `+N −M` counts on the right. In a linked worktree:
`⑂ <worktree folder> · <branch>`, or just the branch when the folder name already ends with the
branch's slug (the launcher's own folders always do, so they show only the branch).

### A session outside any repo (my call)

There is no group header naming where it is, so line 2 shows the short folder (`~/Downloads`), not a
branch. Every row stays two lines, so row height never depends on the content.

### Removed

The folder path line, the dots on group headers, the "Next waiting" button.

## The branch launcher (decided with the owner)

### Bottom of the sidebar

- A small header line `▾ NEW SESSION   hide`. Clicking it collapses everything to the single line
  `▸ NEW SESSION   show`; clicking again brings the button back. Kaze remembers the state in
  `config.toml` (my call: `sidebar_launcher = true|false`, default true).
- Under the header when shown: a button `＋ New session on a branch` with its shortcut.
- New action `new_branch_session` (my call), default Cmd+Shift+N on macOS and Ctrl+Shift+N on Windows
  (both free in kaze today), rebindable under `[keys]`, listed in the palette. It opens the picker
  whether the launcher is shown or collapsed (it shows it if collapsed).

### The picker

Opens in place of the button (mockup "After clicking the button"):

- **Repo:** a dropdown of the repos in the sidebar's groups, defaulting to the shown session's repo.
- **Branch:** a text field. A line under it says what will happen: `new branch, new worktree`,
  `existing branch, new worktree`, or `opens its worktree` (an existing worktree for that branch).
- **Claude / Codex / Shell** choice, defaulting to the last one used (my call).
- **Enter** starts, **Esc** cancels. Errors (git's message, cut to one or two lines) show inside the
  picker and nothing starts.

### What Enter does

1. **Worktree.**
   - The branch already has a worktree: use it, create nothing.
   - Otherwise create a **sibling folder** of the repo's main checkout (owner's choice):
     `<parent>/<main checkout folder>-<branch slug>`, e.g. `~/Documents/coding/kaze-terminal-fix-sidebar-rows`.
     Slug: the branch with `/` and other characters not safe in folder names turned into `-` (my call).
     If the folder exists, refuse with a message (my call: no `-2` suffixes).
   - Existing branch: `git worktree add <path> <branch>`. A branch that exists only on the remote
     (`origin/<branch>`, no local branch) counts as existing (my call): the same command makes a local
     branch tracking it, so picking up a PR branch never forks a same-named branch from `main`.
   - New branch: `git worktree add -b <branch> <path> <base>`. Base (my call): the remote default branch
     as last fetched (`origin/HEAD`), else the local default branch (`main`, then `master`), else the main
     checkout's `HEAD`. No fetch.
2. **Session.** Open a new session with that folder as its start folder, shown in front.
3. **Agent.** Once the shell is ready, type the agent's command into it, the same way session restore
   and workspace runs start commands: `claude`, or `codex --no-daemon`. Codex needs `--no-daemon` or its
   hooks never reach kaze (the 0.17 debugging session). Shell types nothing.
4. **Remember** the worktree if kaze created it (next section).

Git runs off the UI goroutine with a timeout (my call: 15 s). The picker shows "Creating worktree…"
meanwhile and can't be submitted twice.

## Remembering and cleaning up worktrees (decided with the owner)

- Kaze records each worktree it creates in a small state file in its config folder (my call:
  `worktrees.json`): folder, repo key, branch, the base commit it was made from, created time. Records whose folder no longer exists are
  dropped.
- **On closing the last session in a kaze-made worktree** (no other tab, scratch shell or pane open in
  it): if the worktree is clean (`git status --porcelain` empty), kaze asks, as a toast or small dialog:
  "Remove the worktree folder `<name>`? The branch stays." Buttons: Remove / Keep. Keep means kaze won't
  ask again for that folder until a session is opened in it again. If it isn't clean, kaze doesn't ask.
- **Tidy-up line:** when a repo has kaze-made worktrees with no open session whose branch is merged into
  the default branch (local `main`/`master` or `origin/HEAD`) **and** that has commits beyond the recorded
  base commit (my call: a branch with no work of its own is not "finished", it is just new), a quiet line shows under that repo's group:
  `2 finished worktrees · clean up`. Clicking it opens a confirm dialog listing the folders; confirming
  runs `git worktree remove <path>` for each and reports any that git refused (uncommitted or untracked
  changes) with git's reason.
- Removal is always plain `git worktree remove`, never `--force`. Branches are never deleted.
- Merge and cleanliness checks run off the UI goroutine, triggered by events (a session closing in a
  worktree, window focus, the sidebar first drawn after start), at most once a minute per repo (my call).
  No timers while idle.

## Sidebar performance

- An unchanged frame must not call `buildSide`, not re-shape any row text, and allocate nothing for the
  sidebar. Today `drawSide` builds the model every frame, and `sessionIDs()` builds it again for keyboard
  navigation and close.
- Approach left to the builder. One way: keep the built model and the recorded draw call and rebuild only
  when an input changed: the tab list, a tab's title or exit, locations/branches, marks, agent state,
  names, line counts, subagent lines, the filter, folding, runs, hover, rename state, the shown tab,
  sidebar size, theme or font. A missed input is a stale-sidebar bug, so tests must cover each input.
- Measure before and after with an unchanged-frame probe and put the numbers in the PR. The audit's probe
  (not committed): `sideWindow` with 7 tabs in 3 repos, 3000 lines of output each, 3 subagents per tab,
  one reused `op.Ops` reset per frame, `win.frame` at 1600x1000, 200 warm frames; it read about 0.4 ms,
  116 allocations and 15 KB per unchanged frame on an Apple M3, mostly `buildSide`, `sideAgents` and
  row labels.

## Constraints

- Go + Gio, macOS and Windows. Strict TDD, RED tests fail safely (bounded sizes, timeouts). Never inject
  global keyboard or mouse input. Tests never run real `git worktree add` outside a temp dir.
- `go test ./...` and `go vet ./...` pass; Windows cross-build passes.
- Commits `type(scope): Verb ...`, no AI attribution. Feature branch, PR, owner merges. No release.

## Left to the builder

- Exact spacing, font sizes and colours, within the current Midnight look and all five themes.
- How the picker's repo dropdown and text field are built (reuse the Settings controls and the sidebar's
  filter editor where possible).
- Whether the launcher's toast/dialog for cleanup reuses the existing done-toast or the Settings confirm
  dialog.
- How git is invoked (the existing line-count runner in `internal/ui` already runs git with a context).
- Splitting the work into PRs. Suggested order: rows and headers, then performance, then launcher, then
  worktree memory and cleanup.
