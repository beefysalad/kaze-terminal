# Control space isolation (short design)

Date: 2026-11-27. Status: proposed, waiting for the owner's approval of this document.

## Intent and why

An agent in one space can today list, read, send to, interrupt and key into tabs in other
spaces. `crossSpace` in `internal/control/handler.go` only adds a warning, and agent-name
lookup (`ResolveFrom`) falls back to other spaces when the caller's own space has no match.
A prompt meant for "codex" can land in another project's codex. The owner wants each agent
to see and touch only its own space.

## Goals

- A tab that calls the control CLI sees and reaches only tabs in its own space.
- A cross-space target is refused, not warned about.
- A config switch, `[control] cross_space = true`, restores today's behavior.

## Non-goals

- No change to hook delivery, the wire protocol shape, worker rules, consent or limits.
- No change for the owner's own CLI use (requests with no `from` tab stay unrestricted).
- No per-space allow lists.

## Anti-goals

- A refusal that leaks anything about the other space (names, agents, ids) in its text.
- Breaking workers: a worker and its parent share a space, so they keep working.

## Approaches considered

1. **Warn only (today).** Easy for an agent to ignore. Rejected.
2. **Refuse in the handler, filter `ls` (chosen).** One enforcement point in
   `internal/control`, covered by pure unit tests. The host stays unchanged.
3. **Filter inside the host's `Tabs()` per caller.** Spreads the rule into the UI package
   and makes the host caller-aware. Rejected.

The owner chose approach 2 with refuse-by-default and the `cross_space` switch.

## Design

Scoping applies when `req.From` names a known tab and `cross_space` is false.

- `ls` returns only tabs whose `Space` equals the caller's.
- `ResolveFrom` resolves ids, names and agent names against the caller's space only.
  Today's global fallbacks are removed in scoped mode.
- `read`, `send`, `keys`, `interrupt`, `wait`, `name` and `label` on a tab in another space
  return a new error code `CodeCrossSpace` ("that tab is in another space; cross-space
  control is off"). An id that exists elsewhere gets the same text as an id that does not
  exist, apart from that fixed sentence, so nothing about the other tab is revealed.
  `crossSpace` warning code is deleted in scoped mode.
- With `cross_space = true`, behavior is exactly today's (warning included).
- Config: `Control.CrossSpace bool`, default false, parsed like `control.enabled`, passed
  through `Options`. Settings > Agents gets no switch in this change (file-only); that is
  the builder's call to revisit.
- The existing unknown-tab error lists all tabs (`describeTabs`); in scoped mode it lists
  only the caller's space.

## Tests (RED first, bounded)

- `ls` from a tab with two spaces open returns only its own.
- Explicit id, name and agent-name addressing across spaces are refused; none falls back.
- Same-space actions and worker parent/child flows are unchanged.
- `cross_space = true` restores the warning behavior (existing tests keep passing).
- No-`from` requests stay unrestricted.
- Config default, parse and invalid-value warning.

## Left to the builder

Exact error code number, README and agent-instruction text updates (the peer-messaging
block in `internal/agentdocs` changes, so append the outgoing text to its history), and
whether `wait --children` needs any change (parent and workers share a space).

## Release

User-facing behavior change: a minor release after the PR merges, per CLAUDE.md.
