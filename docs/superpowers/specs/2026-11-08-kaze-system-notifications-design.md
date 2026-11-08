# Native system notifications (#172)

Date: 2026-11-08. Status: owner approved the design and TDD plan on 2026-11-08;
implementation and independent review complete. Native delivery and activation
checks remain open as recorded in the audit.

## Recommendation

Add a native notification for an agent's existing needs-you or done alert only
while Kaze is in the background or minimized. Clicking it restores Kaze and opens
that session. Keep the latest notification for each session. Add
`system_notifications = true`, enabled by default, to Settings > Agents > Alerts.

Use the OS APIs in the existing process: UserNotifications on macOS and WinRT
toast notifications on Windows. Do not add an external notifier, a polling loop,
the Windows App SDK runtime, a protocol handler, or a separate notification app.
This feature concerns running sessions; reopening a session after Kaze exits is
outside its scope. Remove this instance's notifications when it exits.

## User behavior

1. A needs-you or done alert that qualifies for an existing card also qualifies
   for a system notification if Kaze is unfocused, the setting is enabled and the
   native backend is available. A focused Kaze never posts a native notification,
   even if a different session is selected. Existing in-window cards, chimes and
   taskbar flashing keep their behavior.
2. The title is the sidebar's session name. The context contains its branch and
   slash command when known. The body uses the cleaned card reason and its short
   status, such as "Needs your permission", "Waiting for your answer" or "Done".
   Reuse the existing card formatting and its bounded 160-rune reason. Sanitize
   the title and context too. OS layouts may truncate the resulting text.
3. One newer alert replaces the previous notification for that session. There is
   no reminder or repeated notification on focus changes, redraw or settings
   changes. Prompt, usage, tool, doc and subagent lifecycle messages never post.
   Workspace run summaries are outside this first change.
4. A click restores/raises the originating window and uses `toastGo` to select
   the session, including the existing waiting-helper panel behavior. A callback
   for a closed tab, previous window generation or replaced notification does
   nothing. A stale callback must never open a different tab with a reused ID.
5. Visiting a session withdraws its notification. Closing its tab withdraws it.
   Turning the setting off withdraws this instance's notifications and drops its
   pending work. Merely focusing the app does not discard other sessions' alerts.
   The in-window done-card timer remains `done_toast`; the system banner and
   Notification Center lifetime follow OS rules until visited, replaced or closed.
6. Native notifications are silent so the existing chime remains the only Kaze
   sound. Use ordinary notification priority and let OS Focus/Do Not Disturb and
   per-app permissions control native delivery. This change does not alter the
   existing chime's behavior under Focus/Do Not Disturb.

## Permission and settings

The new row reads "System notifications", with hint "When Kaze is in the
background. Click a notification to open its session." It saves immediately and
uses the existing Settings controls and font.

On macOS, after a normal visible bundled launch with the setting enabled, check
authorization and request alert permission once if undetermined. This is the
standard OS prompt, without a second custom confirmation. Do not request sound
or badge permissions. If denied, show "Blocked in macOS notification settings"
on the row; do not repeatedly prompt. Check again on a normal focus return or
opening Agents settings, never on a timer. An unbundled development binary shows
"Available in the Kaze app bundle" and must never call an API that can abort for
missing bundle identity. Permission errors are retained for Settings rather
than producing a status message for every hook.

Windows uses the OS's per-app notification policy. A registration or native
delivery failure is shown in this row without stopping cards, chimes or hooks.
No automatic retries while idle; retry on a later normal launch or explicitly
turning the row off and on. No native setup or permission calls run in tests or
bench mode.

## Event and threading model

`onNotify` already handles chime and flash before a frame because Gio provides
no frames while minimized. Native dispatch must likewise happen before a frame.
Do not put it solely in `deliver`, `drainAlerts` or drawing code.

Publish a small immutable snapshot of each session's card metadata whenever its
sidebar name, branch or command changes. The notify goroutine reads that snapshot,
the existing atomic focus state and synchronized notification settings. It must
never read UI-owned maps or invoke `sessionMeta`/`cardOf` concurrently. Missing
metadata uses the existing safe session label, never a shell title presented as
the agent's task. Do not perform Git, transcript or filesystem reads per alert.

A per-window controller owns a sleeping event worker and a bounded latest-per-tab
queue (at most 256 pending entries; remove the oldest if exhausted). Posting is
nonblocking. Recheck enabled/focus/live-tab generation immediately before native
submission, so pending work is not replayed after a focus return. Permission
completion may release the latest pending alerts only if still eligible. Remove
operations supersede pending posts for that tab. Shutdown invalidates the window
generation and drops all work before detaching callbacks.

The native interface supports post/replace, remove, permission/status and close;
tests inject a fake. Native click callbacks enqueue a small activation record.
They must never call Gio synchronously, especially through AppKit's main queue.
Restore/raise the native window on its proper native thread; enqueue session
selection to the window loop. Ensure the activation queue is drained without
requiring a pre-existing FrameEvent when minimized, then invalidate for drawing.

## Windows backend and updates

Use `Windows.UI.Notifications.ToastNotificationManager` through a small Go COM/
WinRT bridge and the existing Windows syscall support. Set a stable Kaze
AppUserModelID, for example `com.patrickgogo.kaze`, and pass it to the notifier.
Create/repair a Kaze-owned per-user Start menu shortcut with that ID once during
backend initialization. Repository inspection found no existing installer or
shortcut implementation, so this one-time registration is needed for current
zip installs and existing downloaded copies. Never overwrite a shortcut that
points to an unrelated application.

Use the running toast's `Activated` callback, with `Dismissed` and `Failed`
cleanup. Retain callback registrations while the notification is live. Give each
instance an opaque short group and each tab a short tag, so replacing or removing
one instance's notifications cannot affect another. Use tag/group lengths valid
on supported Windows releases and XML-escape all content. Do not put hook tokens,
commands to execute, or paths into activation arguments.

Restore an iconic HWND before requesting foreground activation on a user click.
Use the native HWND APIs, never injected keystrokes. Click behavior must be proved
on a real Windows install, including from Notification Center. If the in-process
Activated path is insufficient on supported Windows, stop and revise this design
for a COM activator instead of silently losing session activation.

The shortcut target is the canonical `kaze.exe`, never `kaze-old.exe` or a temporary
download. Existing update swap preserves that canonical path. Clean up live toasts
and callbacks before update shutdown and register again after relaunch. Do not
introduce generated helper executables or change Smart App Control policy.

## macOS backend and signing

Use a small Objective-C/cgo bridge linked to UserNotifications and AppKit, modeled
on the existing traffic-lights bridge's asynchronous main-thread scheduling.
Retain a `UNUserNotificationCenterDelegate`; assign it during setup before
posting. Use a window-instance-plus-tab request identifier and remove matching
pending/delivered requests before replacing. Suppress foreground presentation in
the delegate as a second guard against a focus race. Activation validates the
opaque controller/tab generation before scheduling native raise and UI selection.

The existing bundle ID is `com.patrickgogo.kaze`; `tools/package-macos.sh` already
ad-hoc signs the complete bundle after copying Info.plist. Verify the sealed code
signing identifier matches the bundle ID and that the bundle validates. Do not
claim a paid Apple Developer account is required for local notifications. Apple's
public API documentation does not establish that every ad-hoc build will be
accepted by notification authorization. Therefore packaged authorization, delivery,
clicks and permission survival across an update are explicit native validation
gates. If current ad-hoc packaging fails, report the exact error and propose the
smallest signing change separately; do not fall back to Script Editor notifications
or promise macOS support without evidence. No APNs, push entitlements, sandbox
changes or network service are needed by this local-notification design.

Read-only feasibility evidence on 2026-11-08: the installed
`/Applications/Kaze.app` passes `codesign --verify --strict`; `codesign -d
--verbose=2` reports `Identifier=com.patrickgogo.kaze`, `Signature=adhoc` and sealed
Info.plist entries. This confirms the current release has a matching bundle
signature. No authorization request or real notification was performed, so this
does not prove delivery or permission continuity.

## Acceptance criteria and limits

- Unfocused and minimized needs/done alerts post promptly without a frame.
- Focused, disabled, bench, unknown/closed-tab and non-alert messages never post.
- Replacements, removals, permission completion and click races remain bounded.
- Native clicks select the right live session, with no global input injection.
- Native API work has zero periodic wakes and no filesystem work on still frames.
- All automated tests use fakes; none posts a real banner or requests permission.
- Windows and macOS require real installed-app checks before claiming complete
  native verification. The design is not evidence those checks have passed.

## Sources checked

- Microsoft: [desktop toast quickstart](https://learn.microsoft.com/en-us/windows/win32/shell/quickstart-sending-desktop-toast),
  [shortcut AppUserModelID](https://learn.microsoft.com/en-us/windows/win32/shell/enable-desktop-toast-with-appusermodelid),
  [Activated for running apps](https://learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.toastnotification.activated),
  [tag identity and size](https://learn.microsoft.com/en-us/uwp/api/windows.ui.notifications.toastnotification.tag).
- Apple: [request authorization](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/requestauthorization(options:completionhandler:)),
  [notification center and delegate](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter),
  [request identifier](https://developer.apple.com/documentation/usernotifications/unnotificationrequest/identifier),
  [code signing identity and subsystem-specific policy](https://developer.apple.com/library/archive/technotes/tn2206/_index.html).

The running-app Windows callback choice follows Microsoft's documented API; its
behavior in Kaze's exact packaging remains to be proved. The ad-hoc macOS signing
assessment is an inference from the current packaging and platform identity rules,
not a tested notification result.
