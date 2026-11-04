# macOS battery in the status bar (#139): design

Owner approved 2026-11-04. Follow-up to #111 (Windows battery, 0.28).

## TL;DR

Add `internal/battery` darwin files that read the battery through IOKit and hear about changes through
the system notify alerts (`notify_register_dispatch`), not a run loop source. The status bar side
(`internal/ui/battery.go`) is unchanged: it already handles read, watch, polling fallback, pause and
the no-battery case.

## Reading

- `IOPSCopyPowerSourcesInfo`, `IOPSCopyPowerSourcesList`, `IOPSGetPowerSourceDescription`; use the
  first source whose type is `InternalBattery` and that is present.
- Decode in a pure Go function `fromIOPS(found, charging, onAC bool, cur, max int) Status` in
  `battery.go`, so it is tested on every OS:
  - not found, or `max <= 0`: no battery.
  - percent = `cur*100/max`, rounded, clamped to 0-100.
  - charging = below 100 and (IOKit says charging, or on AC). Same rule as Windows: a full battery shows
    no bolt.
- No Go allocations per read (the CF objects live and die in C).

## Change alerts (owner's choice over `IOPSNotificationCreateRunLoopSource`)

- `notify_register_dispatch` on `kIOPSNotifyPercentChange` and `kIOPSNotifyPowerSource`, handled on a
  global dispatch queue. The handler calls one exported Go function that does only what the Windows
  `dispatch` does: load the current watcher and call it. No read, no lock, no allocation.
- Nothing touches Gio's main run loop.
- Same contract as Windows: no battery means nothing registered and `ok` false; if either
  registration fails, cancel the other and report `ok` false (the UI then polls at most once a minute
  on frames that happen anyway); stop cancels both tokens once.

## Files

- `internal/battery/source_darwin.go` (cgo, `-framework IOKit -framework CoreFoundation`), plus
  `iops_darwin.c` for the C side (an exported Go function cannot be defined in the same preamble).
- `source_other.go` and its test narrow to `!windows && !darwin`.
- Tests: `fromIOPS` cases (all OSes); darwin: real read sanity, a read benchmark, watch with faked
  register/unregister (both registered, none without a battery, failure undoes, callback reaches the
  watcher and stops after stop), a real watch smoke with a 2 s bound.

## Must hold

No new timer or goroutine; idle CPU zero; bench frames unchanged (bench mode skips the battery).
Desktop Macs show nothing and are never asked again.

## Done when

The MacBook shows the battery at the far right with the bolt and low states, matching Windows; README
and HANDOFF no longer say Windows only.
