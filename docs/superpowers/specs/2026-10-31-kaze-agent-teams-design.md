# Kaze: one lead, Claude and Codex helpers

Date: 2026-10-31. Status: approved by the owner on 2026-10-31.

Implementation: [Agent teams plan](../plans/2026-10-31-kaze-agent-teams.md).

## Owner decisions

- A task has one lead, selected as Claude Code or Codex.
- The lead delegates implementation, review, investigation or testing to helpers.
- Helpers can use the other tool and report back to the lead.
- Once the user starts a task, the lead may launch helpers automatically using
  the tools configured for that team. No extra launch confirmation each time.
- The lead is the user's main point of contact. Helper activity stays inspectable.

## First version

Start with a local team in one repository: one lead and up to two concurrently
running helpers. The concurrency limit is a proposed default, not an owner ruling.
Helpers report directly to the lead; helpers cannot spawn more helpers in v1.
This is separate from Claude's built-in Agent tool and Kaze's existing subagent
tracker. Both remain identifiable in the sidebar.

Example: the user selects Codex as lead and Claude Code as the helper tool,
then asks for a feature. Codex delegates implementation to Claude, receives its
report, reviews the changes, asks Claude for a correction, and answers the user.
Selecting Claude as lead reverses the tools without changing the workflow.

## User flow

1. A New team action asks for a folder, task, lead tool and allowed helper tools.
   The last tool choices are remembered; starting the task authorizes delegation.
2. The lead opens as the main session. Helpers appear nested beneath it, each
   showing its tool, assigned task and state: starting, working, waiting, done,
   failed or stopped. A process exiting does not by itself mean task success.
3. Selecting a helper shows its actual output and exchanged messages. The lead
   stays responsible for consolidating results and reporting to the user.
4. The user can send a follow-up to the lead, inspect a helper, stop a helper,
   or stop the team. Requests for permissions identify the agent that needs them.
5. Completion leaves the result available for inspection. It does not commit,
   merge, push or publish anything on the user's behalf automatically.

Use existing sidebar and pane patterns. A large new dashboard is unnecessary.
Settings > Agents gains the default lead, helper tools and concurrency limit.

## Communication contract

Kaze owns a team identifier, a stable member identifier and an ordered mailbox
for each member. Messages have an identifier, sender, recipient, kind and body.
Kinds include task, follow-up, result, question and lifecycle event.

Proposed CLI surface, to be finalized in the implementation plan:

- `kaze team spawn --agent claude --task-file <file>`: launch a helper; return
  its member identifier. A file or stdin carries task text without shell quoting.
- `kaze team send <member> --file <file>`: enqueue a message and return its id.
- `kaze team inbox`: read pending messages for the calling member.
- `kaze team wait`: wait for a message or a bounded timeout, without polling.
- `kaze team stop <member>`: stop an owned helper and report the outcome.

Each managed agent receives short instructions describing its role, these
commands and how to return a result. A helper's structured final result is also
captured by its adapter, so forgetting a reporting command cannot silently lose
its completion. Delivery acknowledges receipt separately from completion.
Repeated delivery must not start duplicate work. Unknown recipients, oversized
messages and full mailboxes return explicit errors rather than silent drops.

This must deliver results into the lead's workflow automatically, including
when the lead is idle. Showing a report in a pane alone does not meet the goal.
A follow-up received while an agent is busy is queued unless that adapter
explicitly supports steering an active turn.

## Integration and feasibility gate

Use each tool's supported automation interface for managed turns, conversation
continuation and final results. Do not guess readiness from screen text, type
messages into permission prompts, or inject global keyboard/mouse events.
Existing interactive sessions are not automatically converted into team leads.

Before committing to the runtime design, a bounded, isolated probe must prove
both directions with the installed CLIs: Codex lead -> Claude helper -> Codex
follow-up, and Claude lead -> Codex helper -> Claude follow-up. It must check
idle delivery, a busy lead, a helper failure and cancellation. Record CLI
versions, supported interfaces, permission behavior and limitations. Supported
managed turns may need a dedicated task view rather than an ordinary interactive
CLI; that UI decision follows the probe, not an assumption about PTY input.
No owner's credentials, settings or live sessions are modified by the probe.
If real model calls are needed, use a tiny bounded task and the user's normal
permission policy; do not silently change access or approval settings.

## Repository and process ownership

The first version shares the selected checkout. Each assignment states whether
it may edit files; the lead must serialize editing helpers in that checkout.
Read-only review and testing may overlap subject to their actual commands.
Worktree creation and automatic merging are deferred, and the UI explains the
shared checkout. Kaze does not claim this prevents every file conflict.

Reuse local authenticated transport where suitable, with team/member-scoped
credentials. A member can address only its own team. Lifecycle and message
traffic must not be discarded by the lossy tool-history queue.

All agent processes belong to the team and are cleaned up on cancellation or
window exit, including child processes. They do not survive closing Kaze in v1.
No retries that spend more model calls without an explicit policy. A failed
helper returns its error to the lead; cancellation is distinct from success.

## Performance and acceptance

Bound running helpers, pending messages, message sizes and retained output.
Specify exact limits in the implementation plan. Wake on events; no idle ticker,
background transcript watching or parsing on the terminal rendering hot path.
Do not store tokens or authentication material in task history.

Acceptance requires both lead choices to delegate, receive a result, request a
revision and report completion without user message copying. Cover queued
follow-ups, permission waits, failed launches, duplicate delivery, full queues,
stale members and stopping a team with bounded RED tests before implementation.
Verify Go tests, vet, targeted races and macOS/Windows builds. Exercise GUI code
paths directly without global input. Show actual CLI output; do not label a
simulated exchange as a verified integration.

## Deferred

Remote teams, agents spawning arbitrary descendants, persistence across app
exit, automatic worktree merging, model billing dashboards and arbitrary mods.
