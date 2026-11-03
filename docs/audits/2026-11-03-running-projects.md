# Running mark and active project (#107), 2026-11-03

The owner approved the running mark in #107, then requested clearer emphasis on
the active project after sharing the sidebar. The owner approved the
[before/after mockup](../design/active-project-header-mockup.html) on localhost on
2026-11-03. Both parts are implemented locally with #113 and #114. No release,
version bump, commit or push. Unrelated original working-tree changes remain.

## Result

A still closed ring and bright core replace the open running arc in session
rows, subagent lines, the title breadcrumb and the status-bar agents pill. At
14 px its radius is 5.6, stroke 1.3 and core radius 2.6; the pill scales to 12 dp.
The mark stays inside its centered slot. Neighboring text positions and existing
needs/done/idle priority are preserved. It has no timer or frame invalidation.

The selected main session's repository group gains a soft accent tint, subtle
border and accent name/arrow. The session keeps its existing selection highlight.
The count remains present and a waiting count remains amber; local foreground
adjustments keep header name/count/waiting text at least 4.5:1 in every theme.

The current project uses the existing common-repository key, including linked
worktrees. That key stays in effect when the selected row is filtered away, the
group is folded, or Settings, a document or a floating pane is shown. A plain
session with no repo, or an empty manager, highlights no group. Hidden groups do
not cause another visible project to gain emphasis. The sidebar draw cache
includes the key without rebuilding the row model on selection changes.

## Regression evidence

Bounded tests were run failing before the relevant implementation, then passing:

- Real Gio pixels have a closed ring, solid core and no paint outside its slot
  in every theme at both 1x and 2x.
- Real header pixels change for the active repo and remain identical for an
  unrelated repo key.
- Emphasis follows selected sessions and their changed working folders, preserves
  Settings/document/pane context and survives fold/filter changes.
- Filtered sibling rows retain the active common repo; empty/plain sessions clear it.
- Selection changes do not rebuild the sidebar model. Existing unchanged-frame
  cache tests still measure zero allocations and no row reshaping.
- Running marks still request no idle frames.
- Header name/count/waiting text contrast meets 4.5:1 in all five themes. Initial
  tests caught count contrast of 3.71 in Sumi and 3.15 in Washi, and 3.22 for Washi's
  waiting count; a local theme-foreground blend corrects these without changing
  the global theme palette.

## Development captures

Actual Gio headless render galleries, including the active project, selected
session, subagent running mark, breadcrumb and agents pill:

| Theme | 1x | 2x |
|---|---|---|
| Midnight | [capture](running-projects/midnight-1x.png) | [capture](running-projects/midnight-2x.png) |
| Sumi | [capture](running-projects/sumi-1x.png) | [capture](running-projects/sumi-2x.png) |
| Tide | [capture](running-projects/tide-1x.png) | [capture](running-projects/tide-2x.png) |
| Washi | [capture](running-projects/washi-1x.png) | [capture](running-projects/washi-2x.png) |
| Sakura | [capture](running-projects/sakura-1x.png) | [capture](running-projects/sakura-2x.png) |

These use fixture content from the owner's sidebar scenario. They are development
render evidence, not native product captures or replacements for lander images.

Regenerate and verify mark/header pixels:

```sh
KAZE_RUNNING_RENDER_DIR="$PWD/docs/audits/running-projects" \
  go test ./internal/ui -run 'TestRunningMarkPixels|TestActiveProjectHeaderPaint|TestRunningProjectRenders' -count=1
```

## Final checks

All checks below passed after the final code changes. The independent UI review
returned `ship` for the active-project addition with no material fixes. The
running mark had already received its own scoped `ship` review.

```sh
go test ./...
go vet ./...
go test -race ./internal/ui -run 'TestActiveProject|TestRunningMarkIsStill|TestSidebarCache|TestEmptyWindow|TestShiftEnter' -count=1
go build -o /tmp/kaze-issues-107-113-114-macos ./cmd/kaze
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/kaze-issues-107-113-114-windows.exe ./cmd/kaze
```

Native installed macOS/Windows interactions remain owner checks: switch projects,
fold/filter them, view Settings/docs/panes, change cwd across repositories, and
observe real agent state marks. No global input was injected and the installed
app was not replaced.
