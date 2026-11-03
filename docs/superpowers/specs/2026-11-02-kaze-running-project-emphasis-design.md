# Running mark and active project (#107)

The running-mark design is already approved in GitHub issue #107. The owner
requested implementation and added active-project emphasis after showing a
sidebar where only the selected session row was emphasized. No release, commit
or push; include this with the existing local #113/#114 work for a later bundle.

## Running mark

Use the approved still closed ring with a bright center, everywhere the current
open arc appears: session rows, subagents, title breadcrumb and status agents pill.
At a 14 px box, radius 5.6, stroke 1.3, center radius 2.6. Use each theme's accent
for the ring at approximately 55% and its brighter accent tone for the center.
Keep the whole symbol inside its supplied box. Preserve row/text geometry and
needs/done/idle priority. No animation, timer or extra idle frames.

## Active project addition (approved 2026-11-03)

Review mockup (2026-11-03):
[before/after HTML](../../design/active-project-header-mockup.html) and
[PNG](../../design/active-project-header-mockup.png). The preview uses the owner's
three projects and allows selecting another session to show how emphasis follows
its project. The owner approved this treatment after reviewing the localhost mockup on
2026-11-03. The image is a design mockup, not a product capture.

Give the current repository's group header a soft accent-tinted background and
accent-colored project name and disclosure arrow. Keep its existing count or
amber waiting count and keep the selected session row's existing highlight.
Do not add another status dot, badge or animation to group headers.

The current project is the selected main session's repository, including linked
worktrees grouped under their common repo. Keep that context while viewing a
session's document, a floating pane or Settings. A plain session without a Git
repo, or an empty window, highlights no repository. A filter may hide the group;
never highlight an unrelated visible group. Folding the group keeps its emphasis.

Use the existing sidebar palette and geometry. The accent text must have at
least 4.5:1 contrast against the tinted header in all five themes. Keep sidebar
model/draw caching effective; selection changes should invalidate drawing without
rebuilding every row or shaping unchanged frames.
