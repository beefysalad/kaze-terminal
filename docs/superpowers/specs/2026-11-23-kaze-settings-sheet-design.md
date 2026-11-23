# Settings as a floating sheet (design)

Date: 2026-11-23. Requested by the owner ("yes please float it"), after seeing herdr's settings sheet.
Builder's calls are marked **(call)**.

## Intent

Settings is a full page that replaces the terminal. Make it a centred floating sheet over a dimmed backdrop,
like the search palette, so the terminal and sidebar stay visible behind it and closing it returns the owner to
exactly where they were. Same sections and content as today.

## Behaviour

- Opens from the same entry points (Settings row, title bar gear, shortcut, palette). A centred sheet, up to
  880 x 640 dp, rounded border, soft shadow, backdrop dimmed with the theme's own colour **(call: 55% black on
  dark themes, 25% on light)**.
- Below about 900 dp window width (or 680 dp high) the sheet fills the window as the page does today **(call)**.
- Esc closes it. A click on the backdrop closes it. Opening it takes keyboard focus; closing returns focus to the
  terminal that had it. The terminal underneath is drawn and keeps running, but takes no input while the sheet is
  open.
- The section menu stays on the left, the shown section on the right, scrolling inside the sheet.
- Sections that assumed a full-width page (Shortcuts table, Workspaces editor, Updates, About) are retuned to the
  sheet width.

## Goals, non-goals

Goals: no new idle wakeups, 0 allocations for an unchanged frame with the sheet open (as today's page), one font
(JetBrains Mono), no emoji, drawn icons only. Non-goals: the layout picker with thumbnails (herdr's Look page),
new settings, moving settings out of `config.toml`, a Settings tab.

## Approaches

1. **Draw the existing page into a centred rect over a backdrop (chosen)**, copying the palette's overlay,
   dismiss and focus pattern. Least change; content code is rect-based.
2. Open Settings in its own native window. More plumbing per platform; breaks the "stays beside the work" idea.
3. Keep the page and only restyle it. Does not give the floating behaviour that was asked for.
