# Uninstall: remove Kaze's data and agent hooks before deleting the app

Status: design for the owner's review (2026-11-04), builder-checked. Issue #112. Agreed in chat: its own Settings section
(not About), one button, remove Kaze's status line without restoring an older one, never edit Codex's
`config.toml`, and list Kaze-made worktrees without touching them.

## 1. Why

Deleting Kaze (dragging `Kaze.app` to the Trash, or deleting the Windows folder) leaves things behind.
The one that hurts: Kaze's hooks stay in Claude Code's and Codex's settings and keep pointing at a
`kaze.exe` / `Kaze.app` that no longer exists, so the person gets hook errors on every tool call with
no idea where they come from. The rest (settings, saved sessions, cached chimes) is clutter.

Kaze can't notice being deleted: nothing runs at that moment, and there is no installer on either
platform. So the cleanup is a step the person takes from inside Kaze, just before deleting the app.

Success: after the cleanup, Claude Code and Codex behave as if Kaze was never installed, Kaze's own
files are gone, and nothing the person made (projects, worktrees, branches, other tools' settings,
conversations) was touched.

## 2. Goals, non-goals, anti-goals

Goals
- One place, **Settings > Uninstall**, with one button that removes everything Kaze put on the machine
  except the app itself, after a preview of exactly what will go.
- Clear instructions to delete the app afterwards, per platform.
- An honest report of anything that couldn't be removed, with how to remove it by hand.

Non-goals (decided by the owner)
- No lighter "disconnect but keep Kaze" option. One button only.
- No restoring a status line Kaze replaced. Kaze removes its own; the old one is in Claude's backups.
- No editing Codex's `config.toml`, ever.
- No deleting the app itself (Windows can't delete a running exe; macOS asks the person to trash it).

Anti-goals (a failure even if it "works")
- Touching anything that isn't Kaze's: another tool's hooks or status line, other keys in a shared
  settings file, conversations, the `.claude` / `.codex` folders, existing settings backups, projects,
  git worktrees or branches.
- Overwriting or deleting a shared settings file that Kaze can't parse.
- Making any change when the person presses Cancel.
- Kaze writing its settings or session state back to disk after the cleanup, recreating the folder.

## 3. Where it lives

A new section in Settings, **Uninstall**, added last, after About, in the "Kaze" group with Updates and
About (`internal/ui/settings.go:115-129`). Icon: builder's call (a simple trash or exit glyph in the
same style as the others). It follows the pattern of the other sections (`settingsSection` with
`draw` and `act`).

## 4. The page

Top to bottom (wording is a guide; final copy is the builder's, plain and short, no em dashes):

1. **Title and one line:** "Remove Kaze from this computer. Do this before you delete the app."
2. **What will be removed**, built fresh each time the page opens (and refreshed after any change):
   - **Claude Code** (`~/.claude/settings.json`): each Kaze hook by event (Stop, Notification,
     UserPromptSubmit, SubagentStart, SubagentStop, PreToolUse, SessionStart), and the status line if
     it runs Kaze. If Kaze's status line is there, add: "If you had your own status line before Kaze,
     it's saved in a settings.json.bak-... file in ~/.claude." (Only when at least one such backup
     exists.)
   - **Codex** (`<CODEX_HOME or ~/.codex>/hooks.json`): each Kaze hook by event.
   - **Kaze's settings and saved state:** the folder (`%APPDATA%\kaze` / `~/Library/Application
     Support/kaze`; on Linux whatever `os.UserConfigDir()` gives), listing what's in it that the
     person would miss: `config.toml` (preferences), `workspaces.toml` (workspace recipes),
     `sessions.json` (tabs restored at start), `worktrees.json` (which worktrees Kaze made).
   - **macOS only:** cached chimes in `~/Library/Caches/kaze`.
   - Items that are absent are simply not listed. If nothing at all is left, the page says "Nothing
     of Kaze's is left on this computer" plus the app-deletion instructions, and no button.
3. **Will not be touched** (shown only when there is something to show):
   - Kaze hooks found in Codex's `config.toml`: "Kaze doesn't edit config.toml. Remove these lines
     yourself:" with the exact lines (or the hook's event and command when exact lines aren't
     available from the parser), and the file path.
   - Each worktree in Kaze's list that still exists on disk: folder and branch, under "Kaze made
     these worktrees. They may hold unfinished work, so Kaze leaves them. Remove one with
     `git worktree remove <folder>` when you're done with it."
   - Another program's status line or hooks are not mentioned at all (they're not Kaze's business).
4. **One button: "Remove Kaze data and hooks"**, then a confirmation dialog in the existing dialog
   style ("Remove Kaze's data and hooks?", a line saying preferences, workspace recipes and restored
   tabs will be lost, then Remove / Cancel). Cancel changes nothing.
5. **After removal:** a result view (step 6), then Kaze quits.

## 5. What "Kaze's" means

Reuse the existing ownership checks; don't invent new ones.

- **Claude hooks:** a hook command is Kaze's when `claudesetup`'s existing recognition
  (`kazeCommand` / `findHook`, `internal/claudesetup/setup.go`) says so for that event's args, whether
  OK or Broken (pointing at a missing exe counts: that's the main case). Remove **every** Kaze hook
  entry for the event, duplicates included (my call: one left behind is exactly the stale hook this
  feature exists to remove), and hooks pointing at **any** Kaze copy (a dev build, an old install
  path), since recognition is by exe name and args, not by this copy's path. If
  that leaves its group's `hooks` list empty, remove the group; if that leaves the event's list empty,
  remove the event key; if `hooks` is then empty, remove it. Every other key, group, hook and the
  original formatting rules of the existing writer stay as they are.
- **Claude status line:** remove the `statusLine` key only when its command is Kaze's
  (`entryFor(..., argsStatus, ...)` is OK or Broken). A "Taken" status line is left alone.
- **Codex hooks:** the same in `hooks.json`, using `codexsetup`'s recognition (`own`, `candidates`),
  including the "multiple hooks" case that `Fix` refuses: uninstall removes all of them.
  Only `Source == "hooks.json"` entries are removed; `config.toml` entries are listed (section 4).
- **Writes:** through the existing writers (wherever their backup and atomic-write code lives; both
  files get the same rule), which back up the file first
  (`settings.json.bak-YYYYMMDD-HHMMSS`, never overwriting a backup) and write atomically. A file that
  ends up with no Kaze content left is written, not deleted. Pre-existing backups are never removed.
  The backup made by the uninstall itself also stays (it's the person's safety net).
- **Unparseable or wrongly shaped file** (the existing `load` errors): don't write it. Report "Couldn't
  read ~/.claude/settings.json, so Kaze left it as is. Remove the lines that run kaze yourself." and
  carry on with the rest.
- **Missing file or folder:** nothing to do, not an error.
- **Kaze's own folder:** remove the whole config folder named above. It is Kaze's alone
  (`filepath.Dir(config.Path())`). Refuse (report, don't delete) if that path is empty, is not named
  `kaze`, or is a home or root folder: a guard against a broken environment. On macOS also remove
  `~/Library/Caches/kaze` with the same guard.

## 6. Order, shutdown and the result

The order is chosen so that a failure leaves the most useful state:

1. **Stop Kaze writing state.** Before touching anything, put the window into an "uninstalling" state
   in which nothing writes `config.toml`, `workspaces.toml`, `sessions.json` or `worktrees.json` again,
   for the rest of the process: session restore stops (and its lock on `sessions.json.lock` is
   released, which Windows needs to delete the file), setting saves (`saveSetting`, the agent panel
   height, theme, etc.) become no-ops, the worktree store and workspace store don't write. The builder
   finds every writer **and every open file handle** in Kaze's folder (logs, update downloads, `.tmp`
   files from atomic writes, the lock): on Windows an open handle makes the folder delete fail. A test
   must prove none of them recreates the folder after the cleanup.
2. **Agent hooks:** Claude settings, then Codex `hooks.json`.
3. **Kaze's folder**, then (macOS) the chime cache.
4. **Result view** in the Uninstall page:
   - All done: "Kaze's data and hooks are removed." Then the app step, per platform:
     - Windows: "Kaze will close now. Then delete the folder it runs from: <exe's folder>." with a
       button that opens that folder in Explorer after Kaze quits, or just the path if that's awkward.
     - macOS: "Kaze will close now. Then drag Kaze from Applications to the Trash." (Use the real
       `.app` path if it's not in /Applications.)
     - Also: "Claude Code and Codex sessions that are already open keep the old hooks until you
       restart them." (Builder checks whether this is true for Claude Code; drop the line if not.)
   - Something failed: list each failure with the path and the manual step, and what did succeed.
   - One button: **Quit Kaze**. Quitting closes every tab's shell as a normal quit does, with no
     "save your sessions" and no state write.
5. Running shells and agents in tabs are closed by the quit, as a normal quit closes them. The
   confirmation dialog says so if any tab has a running agent ("3 tabs are still running. They will
   close.").

**Another Kaze process running** (a second window or instance that would recreate the files): checked
**at confirm time, not when the page opens** (my call: a check on page open is racy and would have to
give up this window's own `sessions.json.lock` just to look). Opening the page and pressing Cancel
change nothing, including this window's session saving. The "uninstalling" state of step 1 is
one-way and starts only after Remove is pressed. If another Kaze holds the session lock at that
point, nothing is removed and the page says "Close your other Kaze windows, then try again"; this
window may stop saving its sessions until it restarts, and the message says so. How the probe works
with the lock this process already holds is the builder's call.

## 7. Constraints

- No cost to anything else in Kaze: the page reads files only when it opens or after a change, never
  per frame. No polling.
- Every file operation is bounded and has a clear error; nothing hangs the UI (do the file work off
  the UI goroutine if any of it can block, then show the result).
- Works on Windows and macOS; Linux builds compile and behave sensibly (config folder only).
- Strict TDD, RED tests bounded, all file tests in temp dirs with fake home / config / CODEX_HOME
  paths. Tests never touch the real `~/.claude`, `~/.codex`, `%APPDATA%\kaze` or the installed Kaze.
- Never inject global input when testing the page; test through code paths and headless captures.

## 8. Tests the builder must include

- Claude: settings with Kaze hooks (OK and Broken), another tool's hooks in the same group and event,
  a Kaze status line, unrelated keys → only Kaze's entries gone, everything else byte-for-byte
  equivalent per the existing encoder, a backup written. A "Taken" status line stays. Malformed JSON
  → file unchanged, failure reported.
- Codex: `hooks.json` Kaze hooks removed, others kept; a `config.toml` Kaze hook listed, file
  unchanged.
- Folder removal: removes the folder; refuses a wrong-looking path; missing folder is fine.
- Cancel makes no change (all files identical).
- After the cleanup, no writer recreates the config folder (simulate the usual writes: a setting save,
  a restore flush, a worktree record, a workspace save).
- The preview lists exactly what the removal then removes (same function feeds both).
- Worktrees from the registry that exist are listed; ones gone from disk are not.
- A partial failure (e.g., a read-only file) shows in the result with the path; the rest still ran.

## 9. Shipping

A user-facing feature: minor release after the merge (the next minor after whatever is current then,
per `CLAUDE.md` "Shipping a feature"). README gets a short "Uninstalling" section; the lander FAQ can
get "How do I uninstall?" if the FAQ has room. No screenshot changes are required (the Settings
screenshot may show the new menu item; note it in `docs/site-assets/SCREENSHOTS.md` if so).

## 10. Left to the builder

Names, the icon, exact copy, how the "no more writes" switch is threaded through the writers, whether
the removal functions live in `claudesetup` / `codexsetup` (recommended: `Remove(settings []byte)
([]byte, []string, error)` beside `Fix`) and a small new package for the folder part, and whether the
work runs on a goroutine.
