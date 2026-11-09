# Kaze Devbox page: design

Status: built as an experiment. Mock-up: `docs/design/devbox-settings-mockup.html`.

## 1. What it is

A Devbox page in Settings that answers one question: are my laps still running, finished, stopped, or waiting
on me? It solves the "Monday miss" from the sidebar spec (`2026-10-30-kaze-agent-sidebar-design.md`).

It reads status only. It does not start, stop, answer or bring home jobs.

## 2. Who sees it

There is one download for everyone. Kaze decides at runtime.

- Kaze looks for `~/.config/devbox/credentials`, the file `/devbox:setup` writes, each time Settings opens.
- No file, or no `DEVBOX_API_KEY` in it: no Devbox page and no network call.
- The file also supplies `DEVBOX_URL` and `DEVBOX_NAME`, so neither the key nor the address is built into Kaze.
- This is an owner-approved exception to the sidebar spec's "no devbox-specific code" rule.

### Key handling

- Read only. Never saved elsewhere, logged, shown or put in an error message.
- Sent only over `https`, only to `DEVBOX_URL`. Any other scheme counts as not paired. Redirects are not followed.

## 3. Getting the data

- Called on the page opening and on Refresh. There is no timer and no fetch while the page is closed.
- One background request at a time, never on the render path, 10 second timeout, reply capped at 1 MB.
- Kaze speaks the small part of MCP it needs over streamable HTTP to `<DEVBOX_URL>/mcp`, with
  `Authorization: Bearer <key>`: `initialize`, `notifications/initialized`, then `tools/call` for `list_agents`.

### Reading the reply

`list_agents` returns one text line per job, written for people:

`<id> <status> <kind> <age> [model] <title>  <details>`

- Statuses: `running`, `needs_you`, `queued`, `done`, `stopped`, `failed`.
- Progress is the `NN%` in the details. `needs you: ...` in the details is the question to show.
- A title like `Batch lap lap-...` or `lap-...: Run the...` shows as the lap id. Other titles show as they are.
- Control characters are removed from everything before drawing.
- An unknown status or unreadable line appears as "Unknown" with its raw title. Kaze never guesses a status.
- A structured JSON reply from the devbox would replace this parser later; nothing else would change.

## 4. The page

Top to bottom:

1. Header: title, "Checked just now", Refresh.
2. Summary strip: `DEVBOX_NAME` and counts for running, need you, and failed in the last 24 hours.
3. Needs you: jobs with a lane and the question.
4. Running: jobs with a lane, newest first.
5. Queued: one-line rows, no lane, because they have not started.
6. Recently finished: five one-line rows, then "Show N older jobs".

Other states:

- Checking: placeholder rows.
- Can't reach it: a plain message and Try again. Earlier statuses are hidden, because they may be out of date.
- Key rejected: "This laptop isn't paired any more", with no jobs.
- Nothing running: a message and the recent finished jobs.

There is no badge on the menu item, because a badge would need background checks.

## 5. The wind spirit

An original character for Kaze, drawn with Gio paths like the existing icons. Pac-Man characters and art are not used.

| Status | Color | Face | Motion |
|---|---|---|---|
| Running | accent | eyes open | drifts, with gusts behind it |
| Needs you | amber | eyes open, badge | slow breathing |
| Done | green | smiling | none |
| Stopped | muted | eyes closed | none |
| Failed | red | crossed eyes | none |

A lane has 26 dots. Dots behind the spirit fade, and a trail runs up to it. Position is the job's progress.

### Cost

- Animation runs only when the Devbox page is showing, the window is focused, and at least one lane is on screen.
- Capped at about 20 frames a second. Spirits sit still when any of those stop being true.
- Nothing is added to terminal drawing or to idle Kaze. A hidden page costs nothing.
- Gate before merging: measure CPU with the page open against another Settings page. If the cost is noticeable,
  lower the frame rate first, then make the spirits still.

## 6. Code

- `internal/devbox`: credentials, the MCP calls, the line parser. No UI imports.
- `internal/ui/settings_devbox.go`: section, states, drawing and the spirit.
- `internal/ui/settings.go`: adds the section only when credentials exist.

## 7. Tests (RED first, all bounded)

- Credentials: missing file, missing key, non-https URL, valid file.
- Page appears only with credentials, and no request is made without them.
- Parser: every status, the `needs_you` spacing, missing progress, both lap title forms, control characters,
  unknown status, an oversized reply.
- Client against a local test server: handshake order, bearer header, 401, timeout, refused redirect.
- The key never appears in any error text or drawn text.
- Fetch happens on open and Refresh only, with no second request while one is running.
- Animation stops when the page is left or the window loses focus.
- Render allocation check for the page.

## 8. To confirm

1. With the devbox owner: each key only sees its owner's jobs, on the server side. If someone else's work ever shows, remove the page.
2. With the devbox owner: the age number is time since the job started.

A laptop paired while Settings is open gets the page the next time the Settings tab is opened.

## 9. Shipping

Experimental, by the owner's call on 2026-11-09. It ships in a normal release, but stays out of the changelog,
the lander, the release notes and the README.
