# Editable shortcuts in Settings > Shortcuts

Status: design for the owner's review (2026-11-05). Approved in chat: click a shortcut and press the new
keys; a clash asks and moves the keys (option 1); refuse keys with no modifier, warn on keys the shell
uses; the look of the clickable mockup (`docs/design/shortcuts-edit-mockup.html`).

## 1. Why

Settings > Shortcuts only lists Kaze's shortcuts today and says "Change any of them under [keys] in
config.toml". Changing one means finding the file, the action name and the key syntax, and a typo or a
clash is only reported as a warning while the default quietly stays. The owner wants to change them in
place, the way most apps do.

Success: a shortcut changes with one click and one key press, takes effect at once, survives a restart,
and nothing breaks quietly: clashes ask, risky keys say so, impossible keys are refused.

## 2. Goals, non-goals, anti-goals

Goals
- Click a shortcut, press keys, saved to `config.toml` and live in the same frame.
- A clash with another action asks: Replace moves the keys and leaves the other action with no shortcut.
- A per-row reset to the default, and "Reset all to defaults" behind a confirmation.

Non-goals
- No per-workspace shortcuts (the palette's "Run workspace <name>" covers running one; owner, 2026-11-05).
- No multi-key chords, no second shortcut per action, no editing the mouse gestures.
- No syncing between machines: each computer keeps its own `config.toml`, as today.

Anti-goals
- A shortcut that silently stops working, or a key press that silently stops reaching the shell.
- Rewriting the person's `config.toml` beyond the one `[keys]` line (comments, order and other tables stay).
- Any cost while Settings is closed.

## 3. Behaviour

### 3.1 Recording

- Each action row's shortcut is a button. A click (or Enter / Space when it has keyboard focus) puts the
  row into **recording**: the button reads "Press the new keys… Esc cancels" in the accent, with a still
  dot (no animation, so no extra frames).
- While recording, the next key press with its modifiers is the new shortcut. Modifier keys alone don't
  end recording. **Esc** cancels and keeps the old keys. A click anywhere else cancels too. Only one row
  records at a time; while recording, the press never runs a Kaze action and never reaches a shell.
- The keys are read with the same names and modifiers `parseShortcut` understands, and saved in the same
  text form (`ctrl+shift+f`, `cmd+k`, `f5`).

### 3.2 What happens to the keys pressed

In this order:
1. **Refused:** no Ctrl, Alt or Cmd (macOS) and not an F-key: a plain letter, digit, symbol, Enter, Esc,
   Tab, Space, arrows, or any of those with only Shift. The row shows "<keys> needs Ctrl, Alt or Cmd with it,
   or it would stop you typing it. <action> keeps <old keys>." (Cmd only on macOS.) Nothing is saved.
2. **Same as now:** nothing changes, recording ends.
3. **Taken by another action:** the row shows "<keys> is used by <other>. Use it here instead? <other> will
   have no shortcut." with **Cancel** and **Replace**. Replace saves both lines (`<this> = "<keys>"` and
   `<other> = "none"`); Cancel saves nothing. The other action stays reachable from the palette.
4. **Saved, with a warning** when the keys are Ctrl plus a single letter and nothing else (no Shift, Alt or
   Cmd), on either OS: "Saved. Your shell and Claude Code use Ctrl+<L> (<what it does>). Kaze will take it
   from them." The "what it does" part comes from a small fixed table for well-known keys (R reverse
   search, C stop a command, D end input, L clear the screen, A / E line start / end, U delete the line,
   W delete a word, Z suspend); other letters just omit it. The warning stays until dismissed (OK) or
   another row is recorded.
5. **Saved** otherwise.

"Saved" means: the `[keys]` line is written, the window's keymap is rebuilt from the new config, and the
row shows the new keys at once. A write failure shows in the row ("Couldn't save: <error>") and the live
keymap is left unchanged.

### 3.3 Unbound actions ("none")

`none` becomes a valid `[keys]` value meaning "no shortcut". `resolveBindings` gives that action no
binding (and no warning); the row shows "None" in muted italics. Recording on such a row works as usual.

**Clash resolution must not depend on order.** Today an override that clashes with another action's
*default* falls back to its own default. After a Replace, both actions are overridden, so resolution must
apply every override first, then report a warning only for a real remaining clash (two actions with the
same final keys), in which case both keep their defaults (as close to today's behaviour as possible).

### 3.4 Reset

- A row whose keys differ from the default shows a small **"changed"** tag and a **reset** button (↺,
  tooltip "Reset to <default>"). Reset removes that action's line from `[keys]` (so the default applies
  again). If the default is now used by another action, the reset goes through the same clash question
  as 3.2 step 3.
- **"Reset all to defaults"** at the bottom of the page asks "Reset every shortcut to its default?" in the
  existing dialog style, then removes every action line from `[keys]`. Lines for unknown names stay.

### 3.5 Unchanged parts of the page

The header search still filters rows (by label, keys and action name). Mouse gestures stay as read-only
rows. The command-line list stays. The page hint becomes "Click a shortcut to change it. Changes save at
once." If `config.toml` can't be read (it has an error), editing is disabled and the hint says so, with the
file's error; the list still shows the active keys.

### 3.6 macOS and Windows

Defaults differ per OS (`defaultBindings`); the page shows this machine's. Recording on macOS reads Cmd
as `cmd`; on Windows the Windows key is not offered as a modifier (it's taken by the system), so a press
with it is ignored while recording.

## 4. Look (from the mockup)

Settings' own style and tokens, so all five themes get it: Manrope for labels and messages, JetBrains
Mono keycaps (1 dp border, 2 dp bottom border) and the muted mono action name under each label. The
shortcut button is borderless until hover / focus, then a hairline and `CapHover` fill; recording uses an
accent border and an accent-tint fill. Messages sit under their row as a rounded strip: refused in the
error tint (`DiffDelFG` family), clash in the accent tint with Cancel / a strong Replace button, warning
in the amber tint with OK. Every text at least 4.5:1 on its surface, marks at least 3:1, in all five themes.

## 5. Constraints

- `config.toml` is changed through `internal/config`'s line-preserving writer: a new function that sets
  or removes one key inside the `[keys]` table (creating the table at the end if it's missing), keeping
  comments, order, other tables and the file's line endings. Never a full re-encode.
- After a write, the keymap is rebuilt from the file the same way start-up builds it, so what you see is
  what start-up would give.
- Nothing runs while Settings is closed. Recording adds no timer and no animation.
- Strict TDD, bounded tests, no global input: tests queue key events on the router.

## 6. Tests the builder must include

- Config: set and remove a `[keys]` line in a file with comments, other tables, CRLF; a missing `[keys]`
  table is created; `none` parses as unbound; two overrides that swap keys resolve without a warning; a
  real final clash warns and keeps both defaults.
- Page: click → recording; Esc and a click elsewhere cancel; each 3.2 outcome (refused, same, clash →
  Cancel / Replace writes both lines, warning text for Ctrl+R, plain save); the keymap acts on the new keys
  in the same frame and no longer on the old ones; per-row reset removes the line and handles a clash;
  Reset all asks and removes only known action lines; a broken `config.toml` disables editing.
- While recording, the pressed keys reach neither a Kaze action nor the shell.
- Contrast for the new texts and strips in all five themes; headless captures of the page in Midnight
  and Washi (default, recording, clash, warning, refused).

## 7. Shipping

A user-facing feature: a minor release after the merge (can share one release with the palette
redesign). README: the "## Shortcuts" section says they can be changed in Settings as well as under
`[keys]`, and documents `none`. Settings screenshot noted for retaking in `docs/site-assets/SCREENSHOTS.md`.

## 8. Calls made after the builder check (mine)

1. The editable rows are exactly the actions in `defaultBindings` (all of today's actions); nothing else.
2. Live rebuild: re-`Load` the file, take only `.Keys`, run `buildKeymap`, and replace the window's keymap
   and its name-to-keys map together. No other setting is re-applied. When Kaze has no config path,
   editing is disabled with a hint, like a broken file.
3. `none` is trimmed and case-insensitive. Any action may be unbound, **except Settings** (`open_settings`):
   a Replace or Reset that would leave Settings with no shortcut is refused with "Settings needs a
   shortcut. Pick other keys." Every place that shows an action's keys (palette, title bar, empty window,
   alert cards, branch launcher) must handle a missing binding by showing no keys.
4. Only keys `parseShortcut` can read back are recordable. A press it can't name (Insert, numpad keys,
   AltGr characters) is ignored and the row keeps recording. Ctrl+Alt is accepted only when the key event
   carries both modifiers (not when Windows turns it into AltGr text).
5. Clash rule after applying every override: if two actions end with the same keys, the overridden ones
   among them go back to their defaults with a warning; a default that still clashes is left as is. A
   small patch to `resolveBindings`, not a fixed-point solver. The swap case (both overridden to each
   other's keys) has no clash and resolves cleanly.
6. The writer edits only a `[keys]` table with plain `name = "value"` lines. If the keys are written another
   way (an inline `keys = {...}` or dotted `keys.find = ...` lines), editing is disabled and the hint says to
   move them into a `[keys]` table.
7. A row set to `none` counts as changed (it has the tag and the reset). Reset all removes the line of every
   known action whatever its value; unknown names stay.

## 9. Left to the builder

Names, where the recording state lives (the settings page state is the obvious place), the exact message
layout in narrow windows, and how the fixed "what it does" table is stored.
