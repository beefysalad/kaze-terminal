# Control Space Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A tab calling the control CLI sees and reaches only tabs in its own space, unless `[control] cross_space = true`.

**Architecture:** One scoping helper in `internal/control` decides which tabs a caller may see. `ls` returns that list and every tab-addressed op (`read`, `send`, `keys`, `interrupt`, `wait`, `name`, `label`) resolves against it, refusing a tab that exists only in another space with a new `cross_space` error code. The host and wire protocol stay unchanged. A config bool flows through `ctl.Options`.

**Tech Stack:** Go 1.27, existing `internal/control`, `internal/config`, `internal/ui`, `internal/agentdocs`.

**Spec:** `docs/superpowers/specs/2026-11-27-control-space-isolation-design.md`

## Global Constraints

- Strict TDD: RED test first, watch it fail, then GREEN. RED tests must be bounded (no unbounded loops, memory or waits; use short timeouts).
- No emojis or Unicode symbol icons anywhere, including docs and commits. No em dashes in prose.
- Commits: `type(scope): Verb ...`, no AI attribution lines of any kind (this repo's CLAUDE.md overrides any default).
- Never push to `main`. Work on a branch (`fix/control-space-isolation`); the owner merges.
- Requests with no `from` tab stay unrestricted. Workers share their parent's space and must keep working.
- Refusal text must not reveal the other tab's name, agent, folder or space.
- Do not launch or drive the native app, and do not inject keyboard or mouse input.
- Test with `go test ./internal/control ./internal/config ./internal/agentdocs`, then `go vet ./...`; run `./internal/ui` last and alone.

## Review Focus

- A caller whose own tab id is unknown or missing from the list (`from` set but not found): behave as unrestricted, as `crossSpace` does today, never panic.
- Two tabs with an empty `Space`: they count as the same space.
- A name that exists in two spaces: the caller gets its own space's tab, not an ambiguity error.
- Agent name (`codex`) present only in another space: refused with `cross_space`, never routed there.
- `wait` on a tab that moves to another space or closes mid-wait: no panic, ends with an error reply.

---

### Task 1: Config switch `[control] cross_space`

**Files:**
- Modify: `internal/config/config.go:26-28` (Control struct), `:298-302` (parse loop)
- Test: `internal/config/write_test.go` (or the file that tests the `control` table; follow where `control.enabled` is tested)
- Modify: `README.md` (the `[control]` examples near lines 711 and 778, and the sentence at 715)

**Interfaces:**
- Produces: `config.Control.CrossSpace bool` (toml `cross_space`, default false).

- [ ] **Step 1: Write failing tests** `TestControlCrossSpaceDefaultsOff` (default config has `Control.CrossSpace == false`), `TestControlCrossSpaceParses` (`[control]\ncross_space = true` gives true, no warnings), `TestControlCrossSpaceRejectsNonBool` (`cross_space = 3` keeps false and emits one warning naming `control.cross_space`, same style as `control.enabled`).
- [ ] **Step 2: Run** `go test ./internal/config -run ControlCrossSpace -v`. Expected: FAIL (unknown setting warning or missing field).
- [ ] **Step 3: Add the field and parse it** with `loadBool(table[key], "control.cross_space", &cfg.Control.CrossSpace, warn)` beside `enabled`. Keep the unknown-key warning for everything else.
- [ ] **Step 4: Document** in the README: add `cross_space = false` to the `[control]` examples and one sentence: by default an agent sees and reaches only tabs in its own space; `cross_space = true` allows other spaces.
- [ ] **Step 5: Run** `go test ./internal/config`. Expected: PASS.
- [ ] **Step 6: Commit** `feat(config): Add control cross_space switch`.

### Task 2: Scope `ls`, `read`, `send`, `keys`, `interrupt`

**Files:**
- Modify: `internal/control/metadata.go:13` (Options), `internal/control/protocol.go` (error code), `internal/control/handler.go` (new helper, ls/read/send/keys/interrupt, delete `crossSpace` use in scoped mode)
- Test: `internal/control/space_scope_test.go` (new); update `internal/control/resolve_agent_test.go` and any other existing test that expects a cross-space warning to set `Options{Enabled: true, AgentTag: true, CrossSpace: true}`.

**Interfaces:**
- Produces: `Options.CrossSpace bool`; `CodeCrossSpace = "cross_space"`;
  `func resolveScoped(tabs []TabInfo, address, from string, cross bool) (TabInfo, []TabInfo, *Error)` returning the target, the tabs visible to the caller, and an error. Unscoped cases (`cross` true, `from` empty, or `from` not among `tabs`) behave exactly like `ResolveFrom` and return all tabs as visible.
- Consumes: existing `ResolveFrom`, `describeTabs`, `TabInfo.Space`.

- [ ] **Step 1: Write failing tests** in `space_scope_test.go` with a fake host holding t1 (space "a", claude), t2 (space "a", codex), t3 (space "b", codex, name "other"):
  - `TestLsScopedToCallerSpace`: from t1 returns exactly t1 and t2; from "" returns all three; with `CrossSpace: true` returns all three.
  - `TestExplicitIdAcrossSpacesRefused`: `send`, `read`, `keys`, `interrupt` from t1 to t3 return `Error.Code == CodeCrossSpace`, and the host records zero submits, keys, screen reads.
  - `TestCrossSpaceErrorDoesNotLeak`: the message contains none of "other", "codex" or "b", and equals the fixed text `that tab is in another space; cross-space control is off`.
  - `TestAgentNameNeverFallsBack`: with t2 removed, `read codex` from t1 returns `CodeCrossSpace`, not t3's screen.
  - `TestNameInTwoSpacesPrefersOwn`: add t4 (space "a", name "other"); `read other` from t1 returns t4, no ambiguity error.
  - `TestUnknownTabListsOnlyOwnSpace`: unknown address from t1 lists only t1 and t2 in the message.
  - `TestUnknownCallerIsUnrestricted` and `TestEmptySpaceTabsShareScope` (two tabs with `Space == ""` reach each other).
  - `TestCrossSpaceOnKeepsWarning`: with `CrossSpace: true`, send t1 to t3 succeeds with the existing "not your space" warning.
- [ ] **Step 2: Run** `go test ./internal/control -run 'Scope|CrossSpace|Unknown|NameInTwo|EmptySpace|NeverFallsBack|DoesNotLeak' -v -timeout 30s`. Expected: FAIL (compile error: `CrossSpace`/`CodeCrossSpace` undefined).
- [ ] **Step 3: Implement** `Options.CrossSpace`, `CodeCrossSpace`, and `resolveScoped`. Approach: find the sender; if scoping applies, build `visible` (same `Space`), call `ResolveFrom(visible, ...)` with the sender; on `CodeUnknownTab` retry `ResolveFrom` on all tabs and, if that finds a tab, return `CodeCrossSpace` with the fixed text, otherwise return the unknown-tab error built from `visible`. Make `ls` return `visible`. Replace the `ResolveFrom(...)` calls and `crossSpace` warnings in `read`, `send`, `keys`, `interrupt` with `resolveScoped`, keeping the warning only when `opt.CrossSpace` is true.
- [ ] **Step 4: Fix existing tests** that relied on the old warning or fallback by giving them `CrossSpace: true`; do not weaken their assertions.
- [ ] **Step 5: Run** `go test ./internal/control -timeout 120s`. Expected: PASS.
- [ ] **Step 6: Commit** `fix(control): Refuse tabs in other spaces by default`.

### Task 3: Scope `wait`, `name` and `label`

**Files:**
- Modify: `internal/control/wait.go:65` (and the signature of `wait`), `internal/control/metadata.go` (`metadata` uses `Resolve`), `internal/control/handler.go` (pass `opt` to both)
- Test: `internal/control/space_scope_test.go`

**Interfaces:**
- Consumes: `resolveScoped` and `Options` from Task 2.
- Produces: `wait(ctx, host, req, opt Options)` and `metadata(ctx, host, req, opt Options)`.

- [ ] **Step 1: Write failing tests:** `TestWaitAcrossSpacesRefused` (wait from t1 on t3 returns `CodeCrossSpace` immediately, bounded by a 2 s context), `TestWaitSameSpaceUnchanged`, `TestNameAndLabelAcrossSpacesRefused` (host records no rename or label), `TestWaitTargetClosesMidWait` (target removed from the fake host's tabs on the second `StatusChanges`: returns an error reply, no panic, within the timeout), `TestWaitChildrenUnaffected` (parent waits on its own worker, same space, still works).
- [ ] **Step 2: Run** `go test ./internal/control -run 'Wait|NameAndLabel' -v -timeout 30s`. Expected: FAIL.
- [ ] **Step 3: Implement** by threading `opt` into `wait` and `metadata` and replacing their `ResolveFrom`/`Resolve` calls with `resolveScoped` (metadata must now pass `req.From`).
- [ ] **Step 4: Run** `go test ./internal/control -timeout 120s`. Expected: PASS.
- [ ] **Step 5: Commit** `fix(control): Scope wait, name and label to the caller's space`.

### Task 4: Wire the option and update the agent text

**Files:**
- Modify: `internal/ui/window.go:309` (add `CrossSpace: opt.Config.Control.CrossSpace`), `internal/agentdocs/docs.go` (managed block text and history), `docs/HANDOFF.md`
- Test: `internal/agentdocs/docs_test.go`, `internal/ui/` control integration test (the one that drives `ls` through the real listener with fake PTYs; find it with `grep -rn '"ls"' internal/ui/*_test.go`)

**Interfaces:**
- Consumes: `config.Control.CrossSpace` (Task 1), `ctl.Options.CrossSpace` (Task 2).

- [ ] **Step 1: Write failing tests:** a UI integration test with two spaces where an agent tab's `ls` shows only its space and a send to the other space's id returns `cross_space`, then the same with `Config.Control.CrossSpace = true` allowing it; and an agentdocs test that the managed block states cross-space tabs are refused by default and mentions `cross_space`, and that the previous shipped block text is recognised as outdated (State returns the outdated status).
- [ ] **Step 2: Run** `go test ./internal/agentdocs ./internal/ui -run 'CrossSpace|Peer|Block' -v -timeout 120s`. Expected: FAIL.
- [ ] **Step 3: Wire the option** at `window.go:309`.
- [ ] **Step 4: Update the block.** Move the current `Block` text into a new history constant (follow the `peer045`/`peer046` pattern and add it to `peerBlock.history`), then change the line "Never send to, read or press keys in a tab of another space ..." to say that `ls` shows only your space and other spaces are refused unless the owner enabled `cross_space`. Keep the rest identical.
- [ ] **Step 5: Run** `go test ./internal/agentdocs ./internal/control ./internal/config`, then `go vet ./...`, then `go test ./internal/ui -timeout 10m`. Expected: PASS.
- [ ] **Step 6: Note it** in `docs/HANDOFF.md` (one short paragraph: behavior, switch, release pending) and update README's peer-messaging block quote to match the new text.
- [ ] **Step 7: Commit** `fix(ui): Apply control cross_space and update the peer instructions`.

---

## After the plan

Open a PR from `fix/control-space-isolation`; after merge, a minor release per CLAUDE.md (version bump, changelog, news strip, README). Owner check (local build, never the installed app): two spaces each with an agent, confirm `"$KAZE_BIN" ls` shows only its own space and a send to the other space is refused.
