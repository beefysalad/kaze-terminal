# Native system notifications (#172)

The owner approved the written design and TDD plan on 2026-11-08. Implementation
uses separate core/UI/config, Windows and macOS lanes, followed by independent
review. Core evidence: `2026-11-08-system-notifications-core.md` beside this file.

## Controller, configuration and integration

Default-on config parsing, explicit false, invalid values and preserving config
comments passed bounded RED/GREEN tests. The first runtime UI tests failed because
no native-fake post or Settings row existed. They passed after integration.
Coverage includes minimized/no-frame delivery, focused suppression, live-tab and
window generation, stale clicks, setting off, bench mode, replacement, permission
status, bounded bursts, failure and concurrent hooks/metadata publication.

Review identified canceled replacement IDs losing the original submitted ID,
and metadata that could be missing or stale before a frame. Regression tests
reproduced these first, then passed after separate submitted/in-flight tracking,
controller-owned hook metadata updates and publication after draining agent state.
The same unchanged metadata publication uses zero allocations in its regression
check. A sleeping worker sends alerts; a serial event pump allows activation to
reach the UI even when Gio sends no minimized frames. No periodic timer is added.

## Windows adapter evidence

Portable XML/identity/shortcut ownership tests and fake-backend lifecycle tests
failed before implementation and passed afterward, including focused race runs.
They cover silent escaped XML, short stable tag/group identities, canonical
executable ownership, exact replacement IDs, timed-out banner callback retention,
failure and explicit dismissal cleanup, late callbacks, policy and explicit retry.

Independent review identified native objects outliving per-call COM apartments.
The adapter now keeps one sleeping worker locked to an initialized MTA thread
until all objects and callback subscriptions are released. Injectable apartment
lifetime tests followed RED/GREEN. Windows-only fake COM/delegate/ABI tests are
compiled for both architectures; they cannot be executed on this Mac. Native
vtable slots were independently compared with Microsoft's Windows metadata.

## macOS adapter evidence

Bounded fake-adapter tests first failed to compile without the implementation,
then passed. Native pure helper tests similarly failed before implementation,
then verified missing/unbundled identity rejection and stable per-tab replacement
keys without initializing UserNotifications or requesting permission.

Independent review found that asynchronous close could return to `os.Exit`
before native cleanup. A bounded regression first failed because the Go lock
remained held during native completion; after the fix it passes. Close removes
callback registration and releases the Go lock, then completes native cleanup
inline on AppKit's main thread or waits up to one second for queued cleanup.
Calls from Go threads use autorelease pools for Cocoa temporaries.

Gio's native ClosingEvent waits for an event acknowledgment while servicing its
own native work queue. A GCD-only close could still miss that window. An additional
bounded regression failed before native-runner scheduling, then passed with
`Window.Run` scheduling and explicit completion. The Go lock is released first;
cleanup can run inside Gio's native event delivery without calling Gio inline
from a native callback. A missing/closed runner still has a bounded wait.

A separate RED/GREEN regression verifies explicit Retry is distinct from normal
permission refresh. Native authorization requests remember a prior attempt;
ordinary focus refresh does not repeatedly ask after an undetermined error.
The user can explicitly retry by turning the setting off and on.

`go test ./internal/sysnotify -count=1` and
`go test -race ./internal/sysnotify -count=1` pass. Automated tests use fake
native adapters or pure bridge helpers, never real notification posting.

## Native checks still required

The installed app's existing matching ad-hoc signature passed read-only strict
verification during design. This does not verify notification authorization,
delivery or activation. A separate packaged development bundle was built at
`/tmp/kaze-notifications-Z2ihNk/Kaze.app` and passed strict signature validation
with `Identifier=com.patrickgogo.kaze` and ad-hoc signing. It was never launched
or installed over the owner's copy.

Owner checks on macOS: allow/deny permission, background and minimized alerts,
replacement, click selecting the right session, Focus behavior and permission
continuity through an update. Owner checks on Windows: zip install registration,
existing-copy upgrade, background/minimized delivery, Notification Center click,
replacement, multiple instances and Smart App Control unchanged.

No native delivery or click results are claimed from cross-builds or fakes.
The owner authorized committing, pushing and opening a review PR on 2026-11-09.
No merge or release has been performed for this work.

## Final integration, 2026-11-09

`go test ./...` and `go vet ./...` pass. Package race tests pass for sysnotify,
codexlaunch and config; focused UI notification and shell-environment/cleanup
race tests pass. Native macOS and Windows amd64/arm64 application builds pass.
Both Windows architectures compile native sysnotify and launcher test binaries;
none was executed on this Mac. Final separate Mac bundle:
`/tmp/kaze-issues-final-y4RNnS/Kaze.app`, strict signature validation passed,
never launched or installed over the owner's app.

An existing dock input test read its fake PTY immediately after queueing input
and failed once under the integrated full run. The terminal's writer is
asynchronous. Using the existing bounded `waitSent` preserves its exact input
and focus assertions without adding frames or input. Independent review agreed;
the package and full suite then passed. No production input behavior changed.
