# Codex tab isolation (#180)

## Implemented scope

Kaze's exact-conversation Codex restore command now includes `--no-daemon`.
This fixes restoration's departure from the documented isolated-launch
requirement, identified in the earlier sidebar design as a separate bug.
The bare `codex` command still resolves through the ready interactive shell's
PATH, aliases or functions. Claude's command is unchanged.

The owner approved the scoped launch wrapper on 2026-11-08. Normal local
`codex`, `codex resume` and `codex fork` launches now receive `--no-daemon`.
The implementation never guesses tab ownership from shared-daemon hooks.
Aliases, absolute paths and PATH overrides can bypass it; remote/administrative
and unknown forms pass through, including ambiguous single-word CLI prompts.
Scope: `docs/superpowers/specs/2026-11-08-kaze-codex-tab-isolation-design.md`.

Each app run creates its own private executable directory and child-only PATH
transform. Unix uses a hardlink/copy and process replacement. Windows makes an
independent unsigned PE console copy, rejecting signed/malformed images without
altering the installed executable. Known npm command shims resolve to Node plus
the official package script, avoiding shell interpolation. Unknown wrappers fail
clearly. Windows keeps the proxy waiting when Codex receives Ctrl+C.

Cleanup runs explicitly before `os.Exit` only after all PTYs report release.
A shutdown timeout preserves the private directory. Removal verifies ownership
and deletes only the known executable and marker; an occupied Windows executable
keeps its marker. No profile, Codex configuration or existing daemon is changed.

## Strict TDD evidence

Before implementation:

`go test ./internal/ui -run 'TestResumeUsesShellPathWithMinimalAppPath|TestResumeResolvesAgentFromZshStartup|TestReviewResumeExpandsZshAliases' -count=1`

All three tests failed because they received `resume` where the expected argv
was `--no-daemon`, `resume`, and the exact saved UUID. Failures were bounded;
native zsh tests used temporary fake agents and three-second subprocess limits.

After adding the flag, the same focused command passed. It verifies generated
zsh/PowerShell commands and actual zsh delivery through a startup PATH and alias,
including the existing visible error when an agent executable is missing.
`go test ./internal/ui -count=1` and `go vet ./internal/ui` also passed.

The launch package's argument, PE, resolution, materialization, ownership and
subprocess fixtures followed RED/GREEN. A runtime regression first printed Kaze's
version for copied `codex --version`; main now dispatches the shim before Kaze's
own CLI/GUI paths. Canonical temporary-directory spelling failed validation
before a same-directory identity correction. Native Unix fixtures check exact
argv, environment, cwd, stdin/stdout/stderr, exit 23 and two distinct tab IDs
despite a fake stale-daemon path. Race checks pass.

The UI/PTY environment regression first failed because the transform was not
called; it now preserves fake tab/listener values through the actual PTY.
Release-result regressions failed before the shutdown callback was added, then
passed for both completed cleanup and a bounded two-second blocked PTY.
Independent review checked command classification, npm recognition, console
signals, ownership/recursion and explicit cleanup, with no remaining blocker.

A Windows-only subprocess fixture builds a tiny fake CLI with a 45-second cap
and checks direct/native and npm Node paths, empty/quoted/Unicode/metacharacter
arguments, cwd, environment, stdio and exit status. It compiles for both Windows
architectures; execution, interactive console waiting/Ctrl+C and Smart App
Control remain Windows runtime gates. Two real Codex sessions remain an owner
check. No native Windows result is inferred from a cross-build.

After integration on `fix/issues-180-172`, `go test ./...`, `go vet ./...`,
the native macOS build and Windows amd64 cross-build passed. Builds were written
to temporary paths and never launched or installed over the owner's copy.

No real Codex sessions, native notifications or global keyboard input were used.
The owner authorized committing, pushing and opening a review PR on 2026-11-09.
No version bump, release or merge was performed.

Final integration on 2026-11-09: full `go test ./...`, `go vet ./...`, package
race tests and focused UI shell/notification race tests pass. Native macOS and
Windows amd64/arm64 application builds pass, as do both Windows native test
compilations. Native Windows execution and real-session checks remain open.
