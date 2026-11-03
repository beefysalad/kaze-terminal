# Implementation plan: empty Kaze window (#113)

Approved with the design by the owner in this session on 2026-11-02.

Design: ../specs/2026-11-02-kaze-empty-window-design.md

1. Add bounded failing tests for remaining open after last-session close/natural
   exit and recovering via existing controls, Settings and configured shortcuts.
   Adjust cleanup tests to assert dialog lifecycle instead of implicit app exit.
2. Remove automatic window close on empty. Share the normal frame pipeline for
   zero tabs, with nil-safe terminal work, correct focus and idle behavior.
3. Add the empty-state message and New session action with existing Gio chrome
   primitives. Keep launch errors visible and existing cleanup dialogs on top.
4. Verify session restoration records zero sessions and explicit window close
   still shuts down. Run focused and full checks, render dark/light wide/narrow
   views and inspect them. Update README and HANDOFF with validation and limits.
5. Leave both #113 and #114 local and unreleased for the owner's later bundle.
