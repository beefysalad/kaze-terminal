# Agent teams implementation plan

Date: 2026-10-31. Design approved by the owner; implementation not started.
Design: [One lead, Claude and Codex helpers](../specs/2026-10-31-kaze-agent-teams-design.md).

## Outcome

A user chooses Claude Code or Codex as lead, gives it a task, and the lead can
launch helpers automatically. Helpers return results to the lead, which can
request revisions and answer the user. No manual copying between terminals.

Do the integration gate first. The architecture below is conditional on its
results. Passing a fake-provider test is not proof of real model cooperation.
Do not introduce UI that promises an unverified capability.

## 0. Isolate the work and record a baseline

- Use a separate worktree on `feat/agent-teams` from current main. Inspect the
  latest main and PR #49 status before choosing the baseline; do not accidentally
  base this feature on the owner's screenshot branch or move that checkout.
- Carry only the approved design and this plan into that worktree. Leave all
  unrelated branding and screenshot work alone. No commit/push until requested.
- Read AGENTS, HANDOFF and CLAUDE; record baseline commit and test results.
- Run `go test ./...` and `go vet ./...`. Keep the HANDOFF-known UI test callback
  race separate from new team races. All RED tests use bounded input/timeouts.

## 1. Prove the integration before choosing adapters

Installed interfaces inspected on 2026-10-31: Codex CLI 0.166.0 and Claude Code
2.1.316. Version/help reads only; no model calls or live team exchange verified.

Relevant official references, checked alongside local CLI help:

- [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode):
  JSON events and continuation by explicit session id.
- [Codex app-server](https://learn.chatgpt.com/docs/app-server): a candidate for
  managed turns and permission requests that require a client response.
- [Claude programmatic execution](https://code.claude.com/docs/en/headless):
  structured output and continuation by explicit session id; SDK interfaces
  must be evaluated when interactive permission responses are needed.

Start with an isolated temporary HOME, temporary agent config roots and a demo
Git repository. Do not copy account credentials into it. Keep probe binaries,
logs and configuration outside the repo. CLI help flags are not runtime proof.

1. Build a small bounded probe coordinator and a stub `kaze team` command.
   Give each member an id, role instructions, mailbox and a private credential.
2. Use local fake providers where supported to deterministically verify real
   CLI process startup, event decoding, conversation id capture, continuation,
   error handling and process cleanup without external model requests.
3. Verify a tiny real exchange in each direction: lead delegates a bounded
   task, helper answers, lead receives the answer and requests a revision, helper
   revises, lead summarizes. Give each turn a 120-second deadline, at most
   three turns per member per exchange and no automatic retries. Respect the
   existing permission policy. If isolated authentication cannot be used without
   changing owner settings, record the limitation and resolve it explicitly.
4. Return a helper report while the lead is idle, while busy, and while waiting
   in `team wait`. Delivery must not create concurrent writers to one agent
   conversation, double-deliver a report or deadlock a waiting tool command.
5. Exercise denied permission, helper launch failure, malformed output, child
   process cancellation and window shutdown. Do not bypass permissions to pass.
6. Record exact interfaces, flags, event examples without secrets, permission
   semantics, model-call counts and what was simulated versus real.

Decision record: choose the smallest supported adapter for each tool that meets
all requirements. A plain non-interactive CLI may suffice for resume but not
permission dialogs. Use app-server/SDK support where necessary; resolve any
additional runtime dependency explicitly. Do not silently downgrade permission
requests into a fake waiting state or silently broaden permissions.

Managed turns may use a Kaze task view with actual streamed output and a message
composer. If the probe requires that UI instead of an interactive terminal,
record it in the design and show the concrete flow before implementing the UI.
Stop at this gate if either lead choice cannot complete the exchange reliably.

## 2. Team model and scheduling

New package: `internal/teams`, independent of Gio and terminal rendering.

Write failing tests, then implement:

- Team/member/message ids, validated roles and tool names, member state machine,
  ordered mailboxes and explicit queued/received/completed delivery states.
- One lead and up to two active helpers per team by default. Config range 1-4;
  at most two active teams and 32 retained members per team, including finished
  members. Reject further launches explicitly; no unbounded history.
- Message bodies up to 32 KiB, at most 64 pending messages per member, at most
  256 retained delivered messages per team. Limit task/title metadata separately.
- Idempotency keys for mutating requests. Retain dedup entries for the team's
  bounded lifetime; reject further mutations if that ledger reaches 4096 entries
  instead of evicting keys that could cause a duplicate launch.
- Reserve a completion/error slot for each accepted turn, separate from optional
  history. A full ordinary mailbox never silently loses a final result. Apply
  backpressure or fail the affected operation explicitly.
- At most one active turn per member. Idle reports schedule a resumed lead turn;
  busy reports queue for the next safe boundary. A `wait` tool receives the
  queued report in its current turn without a second resumed turn.
- Team close invalidates credentials and pending requests. Failed, stopped,
  waiting and done stay distinct. Process exit and task success stay distinct.
- Lead-only spawn/stop authority; helpers can report or ask the lead questions.
  No recursive delegation. Serialize edit assignments in the shared checkout;
  permit non-edit assignments concurrently. Explain that this scheduling policy
  is not a filesystem sandbox and cannot make arbitrary shell commands read-only.

Test both event orders around turn completion/report arrival, cancellation,
full queues, duplicate commands, stale ids and concurrency limits. Inject clocks,
ids and adapters for deterministic tests; no unbounded sleeps.

## 3. Process adapters and permissions

New package: `internal/teams/agentproc` (final boundaries follow the probe).

- Launch resolved executable paths using argument arrays and stdin, never a
  shell command assembled from user task text. Use explicit conversation ids;
  never `--last` or implicit most-recent-session continuation.
- Isolate member env and credentials. Drop inherited team identity for ordinary
  Kaze tabs; never accidentally enroll an owner's existing agent session.
- Decode supported structured events incrementally. Limit an individual event
  to 1 MiB and retained rendered output to 1 MiB per member, with a visible
  truncation marker. Drain excess history without dropping lifecycle/results.
- Capture final results even if a helper never calls `team send`. Associate a
  turn result id so explicit reports and adapter completion cannot duplicate it.
- Handle permission questions through the verified adapter, with agent identity
  and exact requested action. Correlate responses to the pending request; stale
  responses cannot approve another request. No approval-policy or user config
  rewrites. Cancellation resolves outstanding requests without granting them.
- Provide graceful stop with a 3-second deadline then terminate the owned process
  tree. Implement Unix process groups and Windows Job Objects or an equivalent
  verified mechanism, not only killing the top-level CLI process.
- Reap descendants and close pipes on failure, cancellation and window exit.
  Keep cleanup asynchronous to the UI with bounded shutdown waits.

Use fixture streams plus small real subprocess fixtures for RED tests: malformed
and oversized events, stderr flooding, missing finals, permission denial, exited
parents with live children, and cancellation during a blocking read.

## 4. Authenticated team service and CLI

New package: `internal/teams/rpc`; commands in `cmd/kaze/team.go` and dispatch
before window startup in `cmd/kaze/main.go`.

The existing notify listener is fire-and-forget with a nonblocking callback.
Do not extend it into a blocking mailbox service. Use a separate loopback-only
request/reply listener, reusing small transport helpers only where useful.

- Random per-member credentials, scoped to team and member, constant-time
  comparison and no tokens in stdout, logs, saved messages or error strings.
- At most 16 active connections, 40 KiB request envelopes, a 2-second request
  read deadline and 5-second ordinary operation deadline. `wait` is event-driven
  with an explicit maximum 30-second timeout; disconnect/cancel releases it.
- Implement spawn, send, inbox, wait and stop from the design. Accept task/message
  text from stdin or a bounded file read. Return JSON ids/results and meaningful
  nonzero failures; unlike hooks, team command errors must not always exit zero.
- Inbox reads are non-destructive until acknowledgment. Define the acknowledgment
  in the JSON contract and support redelivery with the same message id.
- Invalid credentials, cross-team access, missing destinations, full queues,
  unsupported tools and disallowed operations fail without side effects.
- Make the helper instruction text available through `kaze team --help` and an
  injected short role description. Do not write into project CLAUDE.md/AGENTS.md
  or the owner's global agent settings to teach the commands.

Test socket framing/limits, authorization, stdin/file failures, request replay,
waiting disconnects, blocked clients and server shutdown. No real model calls.

## 5. User interface and settings

Files: `internal/ui/teams*.go`, sidebar models/drawing, pane routing, palette,
`settings_claude.go`, `internal/config` and their tests.

- Add New team to the sidebar/palette. Form: folder, task, lead and allowed helper
  tools. Validate executable availability and task size before starting.
- Show lead plus helper rows, tool labels, assigned task and state. Distinguish
  managed team members from Claude's built-in subagents and ordinary sessions.
- Use the task/session view selected by the integration gate. Actual streamed
  output, ordered exchanged messages, final report, follow-up composer and
  Stop member/Stop team must work without copying terminal text.
- Follow-ups queue while busy and show that state. Waiting permissions open the
  correlated adapter request. A send acknowledgment is not a completion badge.
- Settings > Agents remembers default lead, allowed helpers and helper limit
  through existing config writers. Existing hook setup and alerts keep working.
- UI takes bounded snapshots from the team service; no network calls, process
  waits, transcript parsing or model work in layout/rendering. Events wake Gio.
- Closing Kaze stops teams; explain this in the team flow. Do not claim persistence.

Use existing Gio test helpers to create a team, inspect a helper, send a message,
resolve/cancel a permission request, return to the lead and stop a team. Include
narrow-window layout, long text, unavailable tools and overflowing history.
Native inspection uses an external input-blocked overlay with demo fixtures.
Never global keyboard/mouse injection. Apply the relevant UI skill when building
this surface; no dependency on the proposed layout before the gate passes.

## 6. End-to-end verification and documentation

- Deterministic fake adapters cover both lead choices, helper result delivery,
  revision, busy/idle transitions, errors, permission waits and cleanup.
- Repeat the bounded real CLI exchange after integration. Record the exact
  platform/version matrix; Windows cross-builds do not prove Windows execution.
- Run `go test ./...`, `go vet ./...`, targeted team/transport/adapter races,
  native macOS build, Windows amd64 and arm64 builds. Check idle behavior and
  bounded retention, including a minimized window receiving a message burst.
- Update README and HANDOFF with setup, automatic delegation, shared-checkout
  limits, permissions, cancellation and the actual verification evidence.
- Inspect the diff for secrets, owner paths, unrelated files and probe artifacts.
  Keep commits and PR text free of attribution; commit/push only when requested.
- After an authorized feature merge, follow CLAUDE.md's release workflow: minor
  version, website changelog/feature description, screenshot needs, release PR,
  tag and download verification. Do not release unfinished team support.

## Completion checklist

- [ ] Both lead tools proven in the isolated integration gate.
- [ ] Team model, bounded scheduler and reliable mailboxes tested.
- [ ] Adapters, permissions and process-tree cleanup tested.
- [ ] Authenticated team CLI and automatic lead continuation tested.
- [ ] UI, settings and existing subagent behavior verified.
- [ ] Both-direction real exchange repeated on the integrated feature.
- [ ] Required checks and documentation complete; remaining platform gaps stated.
