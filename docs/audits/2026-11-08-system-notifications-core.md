# System notifications: core, Settings and configuration

Date: 2026-11-08. Implements the approved #172 design and TDD plan. Native
adapters are reviewed and validated separately; this report covers the generic
controller, UI wiring and config only.

## Behavior

Normal Run creates one controller; ordinary test-window constructors never create
native notification services. Bench skips native initialization entirely. Hook
delivery queues eligible needs/done notifications before any Gio frame. A sleeping
serial worker handles latest-per-tab posts and withdrawals with no periodic timer.
UI metadata snapshots use the sidebar's label rules and zero allocations on
unchanged publication. Prompt hooks update the immutable notification snapshot
without a frame, including a safe first-alert fallback and fixed workspace names.

Opaque IDs contain window, tab and revision. The controller validates the current
ID and original Tab pointer on the UI goroutine before raising the native window
and calling toastGo. The Event pump permits this while minimized, with a handshake
ensuring each Gio event/frame finishes before reading the next event.

Live entries and pending work are each capped at 256. Already-submitted IDs and
one in-flight predecessor are retained separately from newer desired content,
so visiting/disabling before a replacement posts still withdraws the real native
notification. Cleanup retains at most 768 unique IDs and runs before fresh posts.

Settings adds default-on `system_notifications`, immediate persistence and native
permission/error hints. Explicit off/on invokes Retry; ordinary focus/Agents
refresh invokes CheckPermission. Existing cards, chimes and taskbar flash retain
their tests and behavior.

## TDD evidence

Observed bounded RED failures before the corresponding GREEN implementation:

- Config default lookup: missing boolean; false round-trip reported unknown key.
- Runtime no-frame notification: `posts=[]`; Settings row was missing.
- Review: sent A, queued B, then visit/disable/focus left native A visible. A fake
  tracking exact current native IDs reproduced all three failures.
- Review: queued Remove(A), fresh B, canceled B lost A's withdrawal. A separate
  exact-ID fake regression failed and then passed with retained cleanup.
- Prompt/current command without frames posted the previous title and empty
  context; first alert for a new tab was dropped. Both failed then passed.
- Fixed workspace title before first publish became the prompt argument; failed
  then passed after honoring immutable FixedTitle in fallback metadata.
- Allocation assertion measured two allocations on unchanged metadata, then zero
  after delaying escaping sidebar callbacks until a comparable input changed.

Focused fake tests cover policy, done/needs, wrong/closed tabs, replacement/stale
click, cancellation, caps, shutdown, metadata, backend failure, Settings, bench,
real controller worker delivery without a frame and concurrent publication/hooks.

`go test ./internal/ui ./internal/config` and `go vet ./internal/ui
./internal/config` passed before the final two metadata review corrections.
`go test ./internal/ui -run TestSystemNotification -count=1` passed after those
corrections; focused race checks are rerun with the final integration. All tests
use fake backends and bounded fixtures. No real notification or permission prompt
was requested and no global input was injected.

## Review and remaining checks

Independent review found the exact-ID cleanup and pre-frame metadata issues above;
both were addressed. The reviewer checked the Event pump handshake and no-frame
activation path. Final integrated full-suite, race and platform builds belong to
the parent integration. Native banner, click, permission, OS Focus and installed
update behavior still require their platform checks; this core audit does not
claim they passed.
