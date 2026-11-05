# Palette redesign: sessions first, skills behind `/`

Status: design for the owner's review (2026-11-05). Approved in chat: the behaviour (sessions first, no
servers, `/` switches to skills) and the look of the clickable mockup v3
(`docs/design/palette-mockup.html`).

## 1. Why

The owner opens the palette almost only to jump to a session. Today it lists every skill Claude has
(the owner's own plus every plugin's: devbox, superpowers, v3-gauntlet and more), so the sessions sit
above a long skill list and the palette feels crowded. The owner hasn't started a skill from it because
finding one means scrolling through names they don't know. Servers (workspace members) are listed too,
but they are already one click away in the status bar.

Success: on open the palette is short and is about sessions; a skill is one `/` away and easy to find
by part of its name or by what it does; the palette looks modern and premium in all five themes.

## 2. Goals, non-goals, anti-goals

Goals
- On open: waiting items, sessions, at most 3 recently used skills, one "Browse all N skills" row.
- `/` (or Tab, or the Skills switch) turns the palette into a skill picker, grouped by source.
- The new look from the mockup, drawn with Kaze's own fonts and theme tokens.

Non-goals (decided by the owner)
- No servers / workspace members in the palette at all.
- No new way to run a workspace: "Run workspace <name>" stays a Kaze action, as today.
- No "copy last answer" or other new actions.

Anti-goals
- Slower open or typing than today, or any work per frame while the palette is closed.
- Losing something you can do today (start a skill in this session, Shift+Enter in a new session, run a
  Kaze action, go to a session, the waiting items).
- Text below 4.5:1 or marks below 3:1 in any theme.

## 3. Behaviour

### 3.1 Two scopes

The palette has two scopes: **Sessions** (the default) and **Skills**. The scope is whatever the field
says: text starting with `/` is the Skills scope; anything else is Sessions.
- Typing `/` as the first character, pressing **Tab**, clicking **Skills** in the switch, or choosing the
  "Browse all skills" row puts a `/` in front of the text (keeping what was typed).
- Tab again, clicking **Sessions**, or deleting the `/` (Backspace at the start) goes back, keeping the
  rest of the text.
- The palette always opens in Sessions with an empty field, as today.

### 3.2 Sessions scope

Empty field, top to bottom:
1. **Waiting on you**: sessions that need you (as today's `palWaiting`).
2. **Sessions**: every other session, in sidebar order. **No workspace members** (drop `in.Members` from
   the palette; they keep their status-bar dots and sidebar rows).
3. **Recent skills**: the last 3 skills started from the palette (today's `recentPrompts`, most recent
   first). Hidden when there are none.
4. One row: **"Browse all N skills"** (N = all skills found). Enter or a click switches to Skills.
   Hidden when no skills are found.

Kaze actions (New session, Open settings, Run workspace ..., Find and the rest) are **not** listed on an
empty field (my call: they crowd the list; everyone reaching for one types its name).

With text (no `/`): Waiting and Sessions that match (as today's ranking), then **Kaze** actions that
match, then **Skills** that match, at most 3, then a row "**N more matching skills**" that switches to
Skills with the same text. Matching is today's `palTier` over the same fields.

### 3.3 Skills scope

Empty after the `/`:
1. **Recent** (the same 3 as above; hidden when none).
2. **Yours**: user-scope skills (`skills.User`), all of them, sorted by name.
3. **This project**: project-scope skills (`skills.Project`), all, when there are any.
4. One group per plugin (`skills.Plugin`, the part of the name before `:`), in alphabetical order. Each
   shows its first 4 skills and then a row "**N more in <plugin>**" that opens the rest of that group in
   place (until the palette closes or the text changes).

With text after the `/`: every matching skill, in the same groups, nothing folded, ranked inside each
group by `palTier` over name and description. A group with no match is left out.

Each group header shows the group's name and its count. A skill row shows the name with the plugin
prefix dimmed (`/devbox:` dim, `run` bright) and the description at the right.

### 3.4 Keys and actions (unchanged unless named)

- ↑ ↓ move over rows (headers skipped), Enter acts, Esc closes, as today.
- On a skill: Enter types it into this session, Shift+Enter starts it in a new session (as today).
- On a session: Enter goes there. On a Kaze action: Enter runs it.
- New: **Tab** switches scope (3.1). Today Tab is swallowed by the field; keep swallowing Shift+Tab.

### 3.5 Empty results

"No results for "<text>"" and one line of direction: in Sessions "Press / to search skills instead."; in
Skills "Try part of what the skill does, like /review.".

## 4. Look (from the mockup)

Everything uses existing theme tokens, so all five themes get it; Midnight values are given as the
reference. No new `[colors]` keys. Fonts: **Manrope** (`sansFace`) for labels, descriptions, status and
footer words; **JetBrains Mono** (`chromeFace`) for session agent badges, skill names, keycaps and the
`/`. Both are bundled already.

- **Box:** 600 dp wide (narrower windows: as today's palette sizing), 14 dp corners, `ToastBG`-family
  surface with a 1 dp `ToastLine` border, and a 1 dp accent hairline across the top that fades out at
  both ends (a horizontal `paint.LinearGradientOp`; a plain accent line at low alpha is an acceptable
  fallback). No blur shadow (Gio has no cheap blur); a dark 1 dp outer line gives the edge.
- **Field row:** 54 dp tall: a magnifier glyph, the editor at 16 sp, and at the right a two-part switch
  **Sessions | Skills `/`** (the active part raised with `CapHover`-style fill and a hairline, the `/`
  keycap in the accent when Skills is on). In Skills, a small "`/` Skills" chip in the accent tint sits
  before the text. Placeholders: "Search sessions and Kaze actions" / "Search skills by name or what
  they do".
- **Section headers:** small caps, letter-spaced, muted, count right-aligned in mono; plugin groups get a
  16 dp rounded tile with the plugin's initial in a colour picked from the theme's ANSI colours by a
  stable hash of the plugin name (my call: no per-plugin config).
- **Rows:** 38 dp, 8 dp corners, inset 6 dp from the box. Selected: accent at about 13% alpha plus a 2 dp
  accent bar on the left. Sessions: the existing status mark (dot or the running ring), title, a mono
  badge with the agent ("claude", "codex"), and at the right a state pill (needs you: amber tint; running:
  accent tint; idle: muted tint). "go to session ↵" shows only on the selected row.
- **Kaze action rows:** a 22 dp icon tile (reuse existing chrome glyphs where one fits; otherwise none),
  the name, and the shortcut as keycaps (mono, 1 dp border, a 2 dp bottom border).
- **Browse / more rows:** a small accent-tint tile with `/` or `+`, muted text with the count in the
  accent.
- **Footer:** keycaps for the keys; for a skill row "↑↓ move · ↵ run here · Shift ↵ new session · Esc
  close", otherwise "↑↓ move · ↵ open · / skills · Esc close".
- Contrast: every text at least 4.5:1 on its own surface (through `render.Readable` as the rest of the
  chrome does), pills' text on their tint at least 4.5:1, marks and tiles at least 3:1, in all five themes.

## 5. Constraints

- Open and typing cost no more than today: the list is rebuilt when it is today (text changes and the
  existing event passes, so a session turning "needs you" while the palette is open still shows); drawing
  an unchanged palette allocates no more than today's (measure before and after).
  Nothing runs while the palette is closed.
- The skills catalog is read the way it is today (cached); grouping is a pure function over it.
- Strict TDD, bounded tests, no global input (the palette tests queue events on the router).
- No new dependencies.

## 6. Tests the builder must include

- Model: empty Sessions field lists Waiting, Sessions, Recent (≤3), Browse, and no members and no Kaze
  actions; text lists ≤3 skills plus the "more" row with the right count; `/` text lists the groups in
  order (Recent, Yours, This project, plugins A to Z) with plugins folded at 4 and the "more" row;
  opening a group shows all of it; text after `/` unfolds everything and drops empty groups.
- Keys: Tab toggles the `/` and keeps the text; Backspace at the start of `/x` gives `x` in Sessions;
  the switch clicks do the same; Browse and "more" rows switch scope; Enter / Shift+Enter on a skill
  behave as today; Esc closes.
- Look: headless captures in all five themes (open, typing, `/`, `/review`, no results); contrast checks
  for the new texts, pills and tiles; a frame allocation check against today's palette.

## 7. Shipping

A visible UI change: minor release after the merge (next minor, per `CLAUDE.md`). README's palette
section updated; the lander's palette mention and any palette screenshot noted for retaking in
`docs/site-assets/SCREENSHOTS.md` (no faked shots).

## 8. Calls made after the builder check (mine)

1. "N more in <plugin>" opens that plugin only (a per-plugin set), cleared when the text or the scope
   changes or the palette closes. The selection moves to the first newly shown skill. The row is a normal
   keyboard target.
2. On an empty `/`, Recent rows also appear in their own group (they are shortcuts, as on Spotlight-style
   lists); they get their own keys (`recent:<name>`) so selection and hover stay unique. With text after
   the `/`, the Recent group is hidden and matches show only in their groups.
3. Recent = the recent prompts that are still in the catalog, most recent first, at most 3.
4. Groups follow `Scope`: User is "Yours" even with a `:` in the name; Plugin uses the text before the
   first `:`, and a plugin skill without one goes to "Other" (last).
5. Kaze actions keep matching on their shortcut text too (today's `palTier(q, a.Name, a.Key)`).
6. The Browse row only shows on an empty field.
7. Tab toggles the scope; Shift+Tab does nothing (still swallowed).
8. The switch and the chip are mouse targets with their own hit rects.
9. Sizes: box 600 dp (shrinks with the window as today), field row 54 dp, rows 38 dp, list at most
   420 dp tall; the short-window logic stays as today.
10. Plugin tile colours go through `render.Readable` against the box, with a contrast test (3:1).

## 9. Left to the builder

Names, how the scope is stored (derived from the text is the intent), the exact glyphs, the plugin tile
colour hash, and the narrow-window behaviour of the switch (hide it under a width, keep Tab working).
