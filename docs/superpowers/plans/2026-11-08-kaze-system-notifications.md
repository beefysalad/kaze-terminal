# Native system notifications implementation plan (#172)

Date: 2026-11-08. Owner approved the design and this plan on 2026-11-08.
Implementation and independent review are complete with bounded RED/GREEN tests.
Native runtime checks remain open; no commit, merge or release has occurred.

Spec: `docs/superpowers/specs/2026-11-08-kaze-system-notifications-design.md`.

## 1. Establish the contract with failing tests

Write bounded table tests for focus, minimized/no-frame delivery, setting off,
bench, closed tabs, needs/done and every non-alert kind. Introduce a fake native
backend and record calls without using OS APIs. Run only these tests and record
the expected RED failure, then implement the smallest policy/controller to pass.

Keep the work on an isolated feature worktree. Never commit/push without explicit
authorization, never push main, never run a GUI test that injects global input.

## 2. Configuration and Settings

RED: default true, explicit false, wrong-type warning, preserved config comments,
round-trip save and a Settings toggle reaching the fake. GREEN: add the config
field/loader/writer and Agents Alerts row. Add fake permission status coverage for
undetermined, allowed, denied, unsupported bundle and backend error; verify no
permission call in bench or ordinary test construction. Reuse existing controls.

## 3. Background delivery and metadata

RED: invoke onNotify without running a frame and assert one fake post with the
sidebar session title, branch, cleaned reason and status. Test focused Kaze on a
different tab, unknown tabs, missing metadata and updates to names/commands.
GREEN: publish immutable card metadata from the UI and route valid events into
the controller before the frame-only inbox. Preserve chime/flash/cards tests.
Use the normal tab identity from the isolated Codex launches provided by #180's
fix. Already-running shared-daemon launches remain affected until restarted with
`--no-daemon`; notification dispatch cannot repair their inherited tab identity.

## 4. Lifecycle, bounds and activation

RED: same-tab replace, multi-tab independence, closed/reused tab, superseded click,
focus return before worker send, disable while permission is pending, visit/close
withdrawal, shutdown callback, bounded burst beyond 256 entries, malformed text
and window-generation isolation. GREEN: latest-per-tab queue, callback generation
checks and explicit removal. Drive worker turns deterministically, no sleeps or
unbounded waits. Fake activation must restore/raise and reach toastGo even when no
FrameEvent has arrived. Run focused race tests for publication and callbacks.

## 5. macOS adapter and feasibility gate

RED: fake bridge tests for asynchronous native operations, missing-bundle guard,
permission result forwarding, foreground suppression, identifier replacement and
late callbacks after detach. GREEN: small cgo UserNotifications adapter following
the existing native-thread bridge. Native tests exercise serialization/callback
plumbing without scheduling requests or prompting.

Build a separate development bundle using the normal packaging flow. Inspect
codesign verification, sealed identifier and Info.plist. Record that successful
compilation/signature validation is not delivery proof. With an owner-driven
installed-app check after design approval, verify permission, allowed/denied,
background/minimized delivery, correct click, replacement, Focus and update
permission continuity. If ad-hoc signing fails, document the exact native error
and revise the signing proposal before declaring this platform done.

## 6. Windows adapter and feasibility gate

RED: pure/fake tests for escaped XML, stable AUMID, short group/tag, shortcut target
canonicalization, ownership/refusal of an unrelated shortcut, COM object release,
Activated/Dismissed/Failed handling and update-old path rejection. GREEN: Windows
WinRT adapter, one-time per-user shortcut registration, retained live callbacks
and native restore/foreground on an explicit notification click. No PowerShell
process per alert, no external runtime, no global input.

Cross-build both Windows architectures. Real Windows verification must cover an
installed zip build, existing copy upgraded in place, minimized notification,
Notification Center click selecting the correct session, replacement and multiple
Kaze instances. If running-toast Activated cannot satisfy this on the supported
OS, revise for COM activation before marking complete. Keep Smart App Control on.

## 7. Integrate, review and document

Run `go test ./...`, `go vet ./...`, focused races, native macOS build and Windows
amd64/arm64 cross-builds. Check no new periodic timer, goroutine wake, metadata
read or native call on unchanged frames. Record bounded queue/native object
lifetime and any platform measurements in `docs/audits/2026-11-08-system-notifications.md`.

Use a fresh review subagent to check the whole change against the accepted spec,
especially minimized dispatch and callback/thread races. Fix real findings with
RED/GREEN tests. Update README config/alerts and docs/HANDOFF.md with exact native
checks still open. Settings screenshots need native retakes if the added row is
shown; never substitute development renders for product screenshots.

When explicitly authorized to commit/push, open a feature PR. The owner merges.
After merge, follow CLAUDE.md's release workflow: a minor release for this feature
(bundled with #180 if appropriate), version test RED/GREEN, all platform metadata,
Windows resources, README, full changelog and lander's latest three/news strip.
Watch release CI and verify both Windows zips, Kaze-macOS.zip and SHA256SUMS. Do
not bump the version before the feature merge.
