# Sidebar redesign implementation plan

Issue #101 authorizes implementation of the approved sidebar design and mockups.
Authority: `docs/superpowers/specs/2026-11-02-kaze-sidebar-redesign-design.md`.

1. Record the unchanged-frame baseline using seven sessions across three repos,
   3,000 output lines and three subagents per session; reuse/reset operations,
   warm 200 frames, render at 1600 x 1000.
2. Write failing row tests, implement the title priority and two-line layout,
   retain filtering/renaming/docs/subagents/workspaces and remove header dots.
3. Write failing cache/invalidation tests, retain the model/session order and
   sidebar draw operations; check every visible input without shaping unchanged
   rows. Keep editors live while focused. Repeat the probe.
4. Write failing temporary-repo tests for a bounded background worktree service:
   branch inspection, reuse/create, remote tracking, base selection, collision
   refusal, atomic bounded state and safe cleanup. Never fetch, force or delete
   branches.
5. Add the collapsible launcher, repo picker, branch preview and agent choice;
   route the new configurable shortcut through commands/palette/settings. Start
   sessions only after git succeeds and agents only after shell readiness.
6. Integrate event-driven background worktree checks and explicit Remove/Keep
   and finished-worktree confirmations, accounting for all open terminal folders.
7. Run focused tests after each RED/GREEN step, then full tests/vet, race tests
   and macOS/Windows builds. Inspect headless renders against the approved
   mockups in all five themes. Document measurements and native checks still open.

Feature branch; owner merges. No version bump, changelog, release or tag.
