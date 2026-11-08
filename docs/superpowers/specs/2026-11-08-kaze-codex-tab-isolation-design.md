# Codex hooks belong to their terminal tab (#180)

Status: the owner approved the Kaze-only launch wrapper on 2026-11-08 after
reviewing its alias, absolute-path and remote-session bypasses. Implementation
and independent review are complete; native runtime checks remain as recorded
in the audit, alongside the completed restore-command regression fix.

## Problem and evidence

Codex 0.167.0 connects interactive clients to a shared app-server daemon. Hooks
run in that daemon with the environment of its first client. Both `SessionStart`
and later hook events therefore carry the wrong `KAZE_TAB_ID` for a second tab.
The same folder, branch and agent type cannot identify the client safely.

The upstream [daemon README](https://github.com/openai/codex/blob/main/codex-rs/app-server-daemon/README.md)
explicitly states that per-client environment isolation is not provided.
The locally installed `codex --help` documents `--no-daemon` as running without
the shared server even when one already exists. The current CLI source exposes
this as a command-line flag, with no equivalent environment switch. Setting
`CODEX_EXEC_SERVER_URL` also avoids implicit attachment, but changes executor
selection and is unsuitable for Kaze.

The existing README already requires `codex --no-daemon`; the earlier sidebar
design explicitly deferred adding it to session restoration. Fix restoration
first. Do not claim this alone resolves manually started daemon-backed clients.

## Existing behavior bugfix

Kaze resumes a saved Codex UUID using `codex --no-daemon resume '<uuid>'` in the
ready interactive shell. Keep the command name bare so shell aliases and a PATH
populated by the shell profile keep working. Claude's resume command is unchanged.
No global Codex configuration or running daemon is changed.

## Approved automatic isolation

A Kaze-scoped PATH shim intercepts normal `codex` starts, with an explicit
documented bypass for administration and custom aliases. It makes plain
`codex`, `codex resume ...`, and `codex fork ...` run locally so hooks inherit
the owning PTY's environment. A daemon started outside Kaze would then have no
effect on these local sessions.

1. Create a private per-Kaze-run directory before starting shells. Put a `codex`
   executable there on macOS and `codex.exe` on Windows. On macOS prefer a hard
   link to the current Kaze executable, falling back to a copy. On Windows use
   an independent private copy and set its PE subsystem to Console so interactive
   PowerShell waits for it; never modify a hardlink or the installed executable.
   Validate PE format, bounds, architecture and subsystem, and reject signed
   images rather than invalidate their signature. Smart App Control remains on;
   it may block this generated unsigned executable, a native verification gate.
   The shim dispatches by
   executable basename and a private Kaze environment marker. Do not rename or
   replace the user's installed Codex.
2. Prepend only that directory to each Kaze shell's PATH. Do not modify shell
   profiles, Codex configuration, `CODEX_HOME`, or the parent process's PATH.
   A profile that replaces PATH entirely can bypass the shim; preserve that
   user choice and document it.
3. At shim invocation resolve the real command using the child's current PATH,
   excluding this shim directory and any executable identical to the shim/Kaze.
   This honors changes to PATH made by nvm and shell startup. Honor Windows
   PATHEXT order, including npm `codex.cmd`, and executable permissions on macOS.
   Refuse a recursion loop with a bounded clear error. A custom `CODEX_HOME`
   passes through unchanged.
4. Forward every original argument exactly, adding `--no-daemon` once for local
   interactive start/resume/fork. Preserve explicit `--remote` invocations and
   daemon management/queue commands. Do not edit command strings by substring.
   Unknown future command forms pass through unchanged. Recommend conservative
   pass-through for an unknown single-word first positional, since it could be
   a prompt or future subcommand. A single-word prompt then requires an explicit
   `--no-daemon`. A positional containing whitespace is a prompt, not a CLI
   subcommand. Unknown options also pass through instead of guessing whether
   the following word is that option's value. Help/version pass through.
5. macOS uses process replacement (`exec`) with inherited stdin/stdout/stderr.
   Windows starts the real executable and waits, forwarding its exit code and
   inherited handles. For the official npm `codex.cmd` layout, resolve the sibling
   `node_modules/@openai/codex/bin/codex.js` and invoke Node directly, preserving
   argv without command-shell interpolation. Reject unknown `.cmd` wrappers with
   a clear error. Isolated Windows argv/console behavior still needs native
   verification before shipping.
6. Existing aliases/functions keep shell precedence and may bypass a PATH shim.
   Document adding `--no-daemon` to those definitions; do not rewrite or remove
   them. Absolute executable paths likewise require the explicit flag. A scoped
   opt-out is removing the shim directory from that shell's PATH.
7. Remove the directory after all Kaze-owned shells have exited. If Windows
   still holds its executable open, leave only that private directory for later
   cleanup; never remove another Kaze process's active directory. No timer,
   process scanning or transcript polling is added.
8. A nested shell or workspace child inherits its owning tab's environment and
   may use the same shim. An independently launched nested Kaze must create its
   own marker and directory, remove stale shim directories from its child PATH,
   and never resolve back to another shim. Validate that the marker identifies
   the private shim directory and that tab/notification identity is present
   before adding isolation arguments. Bounded recursion detection is required.

This design deliberately avoids guessing ownership from a hook's session ID.
It also avoids a claimed full fix for explicit remote sessions: those remain
shared-server sessions and cannot provide per-terminal hook isolation today.

## Acceptance criteria

- Restore submits `codex --no-daemon resume` with the exact saved UUID through
  zsh aliases, shell PATH setup and PowerShell command generation.
- Two local isolated Codex clients in separate tabs retain distinct tab IDs for
  helpers, needs/done cards, labels and running state, even with a preexisting
  unrelated daemon.
- Command proxy tests prove exact argv, environment, cwd, stdio and exit status;
  explicit remote and administrative commands retain their original behavior.
  Signals and interrupted process lifetime must also retain normal behavior.
- Closed/unknown tabs keep existing rejection behavior. Claude remains unchanged.
- No global input injection, real notifications, daemon restarts, user profile
  changes, credential reads, idle timers or unbounded test work.

## Verification and release

Use fake Codex executables and temporary profiles for automated tests. A real
two-tab Codex check is an owner check after review. Run focused tests, full tests,
vet, native Mac build, and Windows cross-build. A Windows runtime runner is
required before promising npm `.cmd` proxy compatibility. Update README and
handoff with the exact implemented scope. Follow CLAUDE.md's release workflow
after the owner merges the feature PR; do not version a partial implementation.
