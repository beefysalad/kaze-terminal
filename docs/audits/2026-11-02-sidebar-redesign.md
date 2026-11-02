# Sidebar redesign, issue #101

Implemented from the approved sidebar spec and its two mockups on
`feat/sidebar-rework`. No version, changelog, release, tag or installed copy changed.

## Behavior

- Two-line session rows: state, agent logo, title and status; branch/worktree
  identity and additions/removals below. Repo headers show a count or waiting
  count, without dots. Names follow rename, command argument, active agent title,
  distinct non-worktree folder, then program. Leading status glyphs are removed.
- Session filtering, renaming, docs, subagent lines, workspace members, close,
  selection and group folding remain supported. Next waiting stays in the
  palette and its shortcut; its button is replaced by the launcher.
- The launcher saves its expanded state and last agent choice. Repo selection
  supports mouse, keyboard and scrolling. Cmd+Shift+N / Ctrl+Shift+N is configurable
  and available through the palette and Shortcuts settings. The picker also
  remains usable when the window normally hides the sidebar for being narrow.
- Branch previews and creation run on background workers. Reuse has no ownership
  side effect. New worktrees are siblings of the main checkout; local/remote
  branches keep their history, and selected remotes are tracked explicitly.
  Folder collisions and Git failures leave the picker open and start no session.
  Claude/Codex start only after a shell prompt, through the existing launch queue.
- `worktrees.json` records only Kaze-created worktrees. The state is versioned,
  size/record bounded and atomically replaced. Missing folders are pruned;
  unreadable/corrupt state is reported and preserved. Git commands have a 15 s
  context, bounded output, bounded pipe shutdown and no Windows console window.
- Cleanup checks use events and a one-minute limit per repo. If a close falls
  inside that limit, one trailing wake finishes that pending action. A checking
  dialog lets Keep cancel it; this is not recurring idle polling. The final
  window waits for the result or Keep. Remove is offered only from a fresh clean
  result and when no terminal, including scratch/workspace members, occupies the
  folder. Keep suppresses the offer until reopening that folder.
- Finished cleanup requires commits since the recorded creation tip and ancestry
  in the default branch. Every removal confirms its folder list, revalidates
  ownership/repository/branch identity and uses plain `git worktree remove`.
  No force, branch deletion or fetch is issued. Refusal reasons wrap and remain
  dismissible even after the last terminal closes.

## Measurements

Apple M3, native macOS, Go 1.27.3. Reproducible probe:

```sh
go test ./internal/ui -run '^TestSidebarUnchangedFrameProbe$' -v -count=1
```

Seven tabs in three repos, 3,000 output lines and three subagents per tab,
1600 x 1000, 200 warm frames, one reused/reset `op.Ops`. These are whole-frame
CPU microbenchmarks, not installed-app responsiveness measurements.

| Unchanged whole frame | Before | After |
|---|---:|---:|
| Time | 167,370 ns | 92,881 ns |
| Allocated bytes | 16,040 | 1,688 |
| Allocations | 111 | 29 |

The matched frame is about 44.5% faster with 89.5% fewer allocated bytes. The
remaining allocations belong to other frame work. The targeted sidebar tests
assert **zero allocations**, no model build and no row layout on unchanged
frames. Coverage includes a nonempty filter and workspace members waiting on a
port. Focused editors paint/register separately over retained rows, so their
caret and selection do not reshape the session list.

The model cache compares raw tab/title/exit/order, cwd/branch/worktree, agent,
rename, marks, line counts, document and run snapshots, subagent revision/time,
filter and folding inputs. Draw recordings additionally track dimensions, scale,
locale, theme/shaper, hover, shown tab/document/member, settings, scroll, rename
and launcher state. Workspace input storage is reused; cached models own copies.

## Verification

RED tests preceded implementation for row naming/status, model/draw retention,
configuration and shortcuts, worktree service and cleanup edge cases. Follow-up
regressions cover popup hit priority/focus, existing-branch creation tips, remote
ambiguity, port-wait allocation and last-tab refusal/deferred cleanup.

- Full `go test ./...` and `go vet ./...`.
- Race checks for UI, branchsession, gitinfo, subagents and tabs.
- Native macOS build and Windows amd64/arm64 cross-builds.
- Real Git integration runs exclusively inside temporary directories: new/local/
  remote branches, remote default base, external-worktree reuse, collisions,
  cancellation, corrupt state, dirty/untracked refusal, merge eligibility and
  branch preservation. UI integration verifies duplicate-submit protection,
  correct start folder, exact Codex command after readiness, errors with no new
  session, and clean-close offers through the throttle.
- Headless Gio GPU renders inspected in all five themes, plus wrapped cleanup
  errors. No native window was opened and no global input was injected.

Development render evidence, shown / picker / tucked-away columns:
[Midnight](sidebar-redesign/midnight.png), [Sumi](sidebar-redesign/sumi.png),
[Tide](sidebar-redesign/tide.png), [Washi](sidebar-redesign/washi.png),
[Sakura](sidebar-redesign/sakura.png),
[cleanup refusal](sidebar-redesign/cleanup-errors.png).
These are actual headless UI renders with deterministic fixture content, not
native product captures or replacements for the lander's screenshots.

To regenerate, use an absolute output path:

```sh
KAZE_SIDEBAR_RENDER_DIR="$PWD/docs/audits/sidebar-redesign" \
  go test ./internal/ui -run 'TestSidebar(Redesign|Cleanup)Renders' -count=1
```

## Still to check natively

Installed macOS interaction with real Claude/Codex, native Windows runtime,
keyboard/editor feel, and removal refusals during an external Git operation.
Cross-builds and headless routing do not establish native behavior. Lander
captures showing the sidebar need retaking after owner review, as recorded in
`docs/site-assets/SCREENSHOTS.md`.
