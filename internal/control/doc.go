// Package control implements transport-independent tab control requests.
//
// A transport authenticates Request.Token and applies its own message size caps
// and deadlines before calling Handle. Handle is synchronous and supports ls,
// read, send, keys and interrupt. Unsupported operations return CodeUnknownOp.
//
// Host implementations provide immutable tab snapshots and safe terminal access.
// Screen owns conversion to plain text: read the live cells, include only the
// requested recent history, join rows marked as wrapped and trim trailing spaces.
// A session-backed implementation must hold the session lock while reading
// cells, history and wrap marks. The alternate screen excludes main history.
// Submit owns bracketed paste encoding and the bounded echo wait before Enter;
// Handle passes the original text without terminal encoding.
//
// The package has no transport, UI dependencies, background workers or timers.
// Screen conversion and terminal input sequencing belong to the Host, so fake
// Host tests here verify delegation rather than terminal rendering behavior.
package control
