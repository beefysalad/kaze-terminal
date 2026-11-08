# Codex tab isolation implementation plan (#180)

Design: `../specs/2026-11-08-kaze-codex-tab-isolation-design.md`.

Status: Task 1 is implemented. The owner approved the Kaze-only launch wrapper
and its documented bypasses on 2026-11-08. Implementation and independent review
are complete. Native runtime verification remains open as recorded in the audit.
The owner authorized committing, pushing and opening a review PR on 2026-11-09.
The owner merges; the release workflow follows the merge.

## Task 1: restore isolated Codex sessions

1. RED: extend the exact resume-command test for zsh and PowerShell to require
   `--no-daemon`. Extend the native zsh fake-agent tests for shell PATH setup and
   alias expansion to require that exact argument. Observe bounded failures.
2. GREEN: insert the flag in `window.resumeLine` only for Codex.
3. Verify focused tests and record that ordinary manual `codex` remains affected.

## Task 2: approved command shim boundary

1. Owner approved the scoped wrapper and documented bypasses on 2026-11-08.
   Use conservative pass-through for unknown single-word positionals/options.
2. Review supported Codex CLI versions. If older installed CLIs lack
   `--no-daemon`, display a clear compatibility error; do not silently join a
   shared daemon after claiming isolation.
3. Windows uses an independent private PE console copy, with format/bounds and
   signed-image rejection tests; the installed image must remain unchanged.

## Task 3: pure launch planning, strict TDD

1. RED bounded table tests for interactive, resume, fork, duplicate flag,
   `--remote` in both syntaxes, queue/daemon administration, help/version,
   literal `--`, option values, prompts, unknown commands and missing executable.
2. GREEN a pure argv planner and current-PATH real-executable resolver.
3. RED/GREEN tests for recursive shim entries, executable permissions, spaces,
   custom `CODEX_HOME`, Windows PATHEXT and npm command resolution.

## Task 4: proxy execution and private directory lifetime

1. RED isolated fake executable integration tests for exact argv, cwd, inherited
   tab environment, stdin/stdout/stderr and nonzero exit code. Use explicit
   timeouts. No real Codex execution or network traffic.
2. GREEN macOS process replacement and Windows child wait/exit propagation.
3. RED/GREEN official npm `.cmd` recognition and direct Node resolution. Unknown
   wrappers fail clearly. A bounded native Windows fixture checks exact empty,
   Unicode, quote, `%`, `!`, `&`, `|`, parentheses and caret arguments without
   shell interpolation. Keep actual execution pending until a Windows runner.
4. RED/GREEN per-run directory creation, fallback copies, failed initialization,
   cleanup ordering and independent Kaze instances. Add the directory to Kaze
   shell environments only; preserve inherited current PATH and avoid recursion.
5. Report whether PTYs fully released at shutdown and preserve the private
   directory if they did not. Explicit cleanup completes before `os.Exit`.

## Task 5: owner behavior and integration

1. RED isolated two-client fixture: a stale preexisting daemon receives no
   proxied local interactive launches; each fake client reports its own tab ID.
2. GREEN wire shim initialization and teardown around tab/session lifetime.
3. Full test/vet runs; native Mac and Windows builds; Windows runtime fixtures.
4. Independent whole-change review and regression-first fixes for real findings.
5. Update README, audit and handoff with evidence and remaining native checks.
   Owner checks two real Codex tabs and a daemon started outside Kaze.
6. Prepare a PR only after explicit commit/push authorization. Follow the
   shipping and release workflow when the owner merges it.
