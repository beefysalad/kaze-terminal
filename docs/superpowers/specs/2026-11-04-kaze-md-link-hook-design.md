# Open the .md file the agent actually touched

Status: design for the owner's review (2026-11-04). Agreed in chat: the hook approach, link click
only, no list UI, no settings switch, nothing kept across a Kaze restart.

## 1. Why

Opening `.md` links has taken four rounds of fixes and still misses. The owner's last miss: in a tab in
`~\Desktop\samurai_cart_v3`, Claude wrote the plan to the Obsidian vault and printed
"Plan is in the vault at `tickets/V3-3154/plan.md`". Clicking it said
"Can't find plan.md in ~\Desktop\samurai_cart_v3".

The miss is built into the current design. `mdoc.Resolve` only tries the tab's folder and its git top
(`internal/mdoc/resolve.go:40-46`). A short link that doesn't say where it lives can't be fixed by
smarter guessing, and the last four rounds all made the guessing smarter.

What Kaze is missing is a record of where the agent really put the file. Kaze already has a hook on
every tool call, in Claude and in Codex: `kaze.exe notify tool` on `PreToolUse`, matcher `*`
(`internal/claudesetup/setup.go:220`, `internal/codexsetup/setup.go:36-39`). The hook already parses
`tool_input` (`cmd/kaze/notify.go:95-96`). It then drops every main-conversation call except
Agent/Task/spawn (`cmd/kaze/notify.go:267`), and the full file path goes with it.

The owner works in Kaze all day and wants plans, specs and brainstorms to open there, without
switching to Obsidian or Notion. Those stay as the long-term archive.

## 2. Goals, non-goals, anti-goals

Goals
- Clicking a `.md` link the agent printed opens the file the agent touched, wherever it lives,
  for Claude and Codex, main conversation and subagents.
- No new hook, no setup step, no "Fix" press: it rides on the existing `notify tool` hook.

Non-goals (decided by the owner)
- No list or panel of "docs this session wrote". The link click is the whole feature.
- No settings switch, and no instructions written into Claude's or Codex's memory.
- Nothing survives a Kaze restart. After a restart, links fall back to today's behaviour.
- No transcript scanning.

Anti-goals
- Any cost on the typing, drawing or scrolling path. Work happens once per tool call (already
  paying for a process spawn) and once per click. No polling, no file watching.
- Opening the wrong file when today's behaviour would have opened the right one (see the order in 4.3).

## 3. Known gap

A file written only through a shell command (`cat > plan.md`, `Set-Content`, a heredoc) reaches the
hook as a command line, not a path. Those clicks fall back to today's guessing. If this bites, the
owner can add "write markdown files with the Write tool" to their own CLAUDE.md. Kaze does not do this.

## 4. Design

### 4.1 Hook side (`cmd/kaze`)

On a `notify tool` call, from any agent (Claude or Codex), with or without an `agent_id`, collect
the `.md` paths in the tool input (`mdoc.IsMarkdown`, case-insensitive):

- `file_path` (Claude Read, Write, Edit, MultiEdit; also any MCP tool that uses the field).
- `path` (only when it ends in `.md`; for Grep and Glob it is usually a folder and is ignored).
- Codex `apply_patch`: every `*** Add File: `, `*** Update File: ` and `*** Move to: ` header in
  `command` (the parsing `toolSummary` already does at `cmd/kaze/agents.go:44-51`, but every header,
  not just the first). `*** Delete File:` is ignored.

Reads count as well as writes: an agent that read the vault file knows where it is, and so should Kaze.
(Codex reads files through its shell, so for Codex only patch writes are seen.)

**Codex payload is unverified (builder must check first).** Nothing in the repo records a real Codex
`PreToolUse` payload for `apply_patch`; the existing parsing and its tests use hand-made fixtures. The
plan's first Codex task captures a real one before any Codex test is written, without touching the
owner's `~/.codex/hooks.json` or live sessions (a throwaway `CODEX_HOME` with a hook that dumps stdin to
a temp file, run with `codex exec` in a temp folder). It confirms that the patch is in
`tool_input.command` and that relative headers are relative to the payload's `cwd`. If either is false,
adjust the parsing to what the payload really carries. If the patch isn't in the payload at all, Codex
keeps today's behaviour, the release notes say Claude only, and the Claude half ships unchanged.

Make each path absolute: a relative path is joined to the hook's `cwd` (`notify.go:112`). Drop a path
that is still relative, longer than `notify.MaxPath`, or has control characters. Keep at most
8 paths per call, in order.

The message carries them in a new field, `Docs []string` (`json:"docs,omitempty"`) on `notify.Msg`.
- A subagent tool call already sends a `KindTool` message; it also carries `Docs`.
- A main-conversation call that `agentMsg` would drop today sends a new `KindDoc` ("doc") message
  instead, but only when `Docs` is not empty. A main call with no `.md` path still sends nothing,
  as today.

The server side must accept the new kind. The 32 KiB body cap holds easily (8 × 1 KiB).

### 4.2 Kaze side (`internal/ui`)

Each tab keeps a short in-memory list of the docs touched in it: at most 64 paths, newest first,
deduplicated by `mdoc.PathKey` (seeing a path again moves it to the front). The list is filled from
`Docs` on any message for that tab, before the existing routing, so `KindDoc` never reaches the
subagent or alert code (`internal/ui/subagents_ui.go:152-155`, `internal/ui/agent.go:88`,
`internal/ui/alerts_ui.go`). The list is dropped with the tab. It is not saved.

Subagent hooks run with the parent tab's `KAZE_TAB_ID`, so their docs land on the parent tab already.

### 4.3 Click side

A recorded path matches a link ref when the ref's path components equal the recorded path's last
components: on whole folder names, case-insensitive on Windows and macOS (`PathKey` rules), with
`/` and `\` treated alike. A bare `plan.md` matches any recorded `plan.md`. Among matches the newest
wins, and it must still exist as a regular file when clicked (`os.Stat`). A `..` in the ref never matches.

Terminal links (`openDocRefs`, `internal/ui/docs_open.go:71-84`) try, in order:
1. Absolute refs, as today.
2. Relative refs against the clicked tab's list.
3. Relative refs through `mdoc.Resolve` (tab folder and git top), as today.
4. Relative refs against every other tab's list, newest first per tab, tabs in tab order.
   (One tab may print a path that another tab wrote.)
5. The existing miss message, unchanged.

Each step goes through every ref in `refs` order before the next step starts, the same as the
absolute-then-relative loop today.

Why the list comes before `Resolve` (my call): if the agent touched `<vault>\docs\plan.md` and the repo also
has `docs\plan.md`, the agent's link means the file it touched. If the agent touched nothing that
matches, step 3 behaves exactly as today.

Links inside an open doc (`followDocLink`, `internal/ui/doc_links.go`): the doc-relative resolve
stays first, then the doc's session tab's list, then the other tabs' lists, then the existing miss.

## 5. Tests (TDD, red first)

Hook (`cmd/kaze`): Claude Write, Edit and Read with an absolute `.md` path; non-`.md` path ignored;
a relative path joined to cwd; Grep with a folder `path` ignored; a subagent call carrying `Docs`
on its `KindTool`; a main call with no `.md` path sending nothing; Codex `apply_patch` with two
headers (relative) plus a delete; the 8-path and `MaxPath` caps.

UI: the list's dedup, move-to-front and 64 cap; `KindDoc` touching no agent or alert state; the
owner's case end to end (a tab in a repo folder, a recorded `<vault>\tickets\V3-3154\plan.md`,
clicking `tickets/V3-3154/plan.md` opens it); list-before-Resolve order; a recorded file deleted
before the click falls through; another tab's list as the last resort; a link inside a doc.

All RED tests use bounded inputs and temp dirs.

## 6. Shipping

A user-facing fix to an existing feature, so under the release rules in `CLAUDE.md` it ships as a
patch release, 0.28 → 0.28.1 (my call: it changes how an existing click behaves and adds no new UI). The changelog line says what the owner gets: "Links to markdown files the
agent wrote or read anywhere (a vault, another repo) now open in the reader." No screenshots change.

## 7. Left to the builder

Names, where the per-tab list lives (on the tab's UI state, next to the agent state, is the obvious
place), and how the click code shares the matcher with `doc_links.go`.
