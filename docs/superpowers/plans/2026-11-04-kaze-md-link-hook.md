# Open the .md file the agent touched: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clicking a `.md` link the agent printed opens the file the agent really wrote or read, wherever it lives, using paths the existing `notify tool` hook reports.

**Architecture:** The `kaze notify tool` hook (already run on every tool call) pulls `.md` paths out of `tool_input` and sends them in a new `Docs` field, on a new `KindDoc` message for main-conversation calls or on the existing `KindTool` message for subagent calls. The window keeps a mutex-guarded, in-memory list of recent doc paths per tab. The click code checks that list (by path suffix) before today's `mdoc.Resolve` guessing.

**Tech Stack:** Go 1.27, Gio. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-11-04-kaze-md-link-hook-design.md` (read it first; the plan argues from it).

## Global Constraints

- Strict TDD: every test is seen failing (RED) before the code that passes it. RED tests are bounded: temp dirs, small inputs, timeouts of 1s on channel waits. No unbounded loops or allocations.
- Never inject global keyboard or mouse input. Never edit `~/.claude/settings.json`, `~/.codex/hooks.json` or `~/.codex/config.toml`, and never touch the owner's installed Kaze at `C:\Users\john\AppData\Local\Programs\Kaze`.
- No new hook event and no setup change: this rides on the existing `PreToolUse` `notify tool` hook.
- Nothing on the typing, drawing or scrolling path. No polling, no file watching. Nothing saved to disk; the list dies with the tab or the process.
- Per-tab list: at most **64** paths, newest first, deduplicated by `mdoc.PathKey` (seen again = moved to front).
- Per hook call: at most **8** paths, each absolute, at most `notify.MaxPath` (1024) bytes, no control characters, `mdoc.IsMarkdown` true.
- Commits: `type(scope): Verb ...`, **no AI attribution** of any kind (no `Co-Authored-By`, no "Generated with").
- Never push to `main`. Branch, push, PR; the owner merges. Release (0.28.1) follows the merge per `CLAUDE.md`, "Shipping a feature".
- Windows: if Smart App Control blocks a test binary, rerun with `-ldflags "-B 0x<random hex>"`.

## Review Focus

1. **A recorded file deleted or moved before the click**: the click must fall through to the next step, never open a missing file or stop at a miss. (Task 5, `TestDocSeenSkipsMissingFile`.)
2. **Bare `plan.md` when several tickets wrote a `plan.md`**: the newest one in the clicked tab wins. (Task 4, `TestDocSeenNewestWins`.)
3. **Case and slash differences** (`Tickets\v3-3154\Plan.md` recorded, `tickets/V3-3154/plan.md` clicked) match on Windows and macOS but not Linux. (Task 4, `TestDocMatchCaseAndSlashes`.)
4. **A partial folder name must not match** (`xtickets/a.md` recorded, `tickets/a.md` clicked), and a `..` in the ref never matches. (Task 4, `TestDocMatchWholeComponents`.)
5. **A `KindDoc` message must not wake any agent, alert, chime or restore state**, otherwise every Read of a `.md` would flicker the tab as "Claude running". (Task 5, `TestDocMsgTouchesNoAgentState`.)

---

### Task 1: `KindDoc` and `Docs` on the notify wire

**Files:**
- Modify: `internal/notify/notify.go` (Kind consts near :37, `Msg` near :57, `handle` :203-268)
- Test: `internal/notify/notify_test.go`

**Interfaces:**
- Produces: `notify.KindDoc Kind = "doc"`; `Msg.Docs []string \`json:"docs,omitempty"\``; `notify.MaxDocs = 8`.

- [ ] **Step 1: Write the failing tests** in `internal/notify/notify_test.go`, using the file's existing send/receive helpers:
  - `TestDocMessagePassesCleaned`: send `{Kind: KindDoc, Tab: 1, Docs: [abs1.md, "rel/x.md", abs2.txt, abs3.md + "\x1b", strings.Repeat("a", 2000)+".md"]}` (abs paths built with `filepath.Join(t.TempDir(), ...)`). Received `Docs` == `[abs1.md]` only: relative, non-markdown, control-character and over-`MaxPath` entries are dropped (not cleaned and kept).
  - `TestDocMessageCappedAtEight`: 12 valid abs `.md` paths in, the first 8 out, in order.
  - `TestDocMessageWithoutDocsDropped`: a `KindDoc` with empty `Docs` (or all dropped) never reaches `onMsg` (wait 200ms, expect nothing).
  - `TestDocsOnlyOnDocAndTool`: a `KindDone` carrying `Docs` arrives with `Docs == nil`; a `KindTool` with `AgentID: "a1"` keeps its valid `Docs`.
  - `TestDocMessageStripsOtherFields`: a `KindDoc` with `Reason`, `Usage`, `Command`, `AgentID`, `Report` set arrives with all of them empty or nil.

- [ ] **Step 2: Run to see them fail**
  Run: `go test ./internal/notify/ -run 'TestDoc' -v`
  Expected: FAIL (compile error: `KindDoc` and `Docs` undefined).

- [ ] **Step 3: Implement**: add the kind, field and constant. Add `cleanDocs(in []string) []string` in `notify.go`: keep an entry only if `filepath.IsAbs`, `mdoc`-style `.md` extension (use `strings.EqualFold(filepath.Ext(p), ".md")`; do not import `internal/mdoc` into `notify`), `len(p) <= MaxPath`, and `CleanLine(p, MaxPath) == p` (so any control character drops it). Stop at `MaxDocs`. In `handle`, after the existing cleaning: `if m.Kind == KindDoc || m.Kind == KindTool { m.Docs = cleanDocs(m.Docs) } else { m.Docs = nil }`. Add `case KindDoc:` to the switch, clearing `Usage, Reason, Command, Args, AgentID, AgentType, Report, TaskPrompt` and `return` when `len(m.Docs) == 0`.

- [ ] **Step 4: Run to see them pass**
  Run: `go test ./internal/notify/ -v`
  Expected: PASS, including every existing test.

- [ ] **Step 5: Commit**
  `git commit -m "feat(notify): Carry the markdown paths a tool call touched"`

---

### Task 2: Capture a real Codex `apply_patch` hook payload

The existing `apply_patch` parsing (`cmd/kaze/agents.go:44-51`) and its tests use hand-made fixtures. Before writing Codex tests, check what Codex really sends. **No product code in this task** unless step 3 says so.

**Files:**
- Create: `cmd/kaze/testdata/codex_apply_patch_pretooluse.json` (the captured payload, with any personal paths replaced by `C:\\work\\repo`-style placeholders; no tokens or secrets)

- [ ] **Step 1: Capture in a throwaway setup.** In a temp folder, make a scratch `CODEX_HOME` (copy only `auth.json` from `~/.codex` if Codex needs it to log in; **do not modify anything under `~/.codex`**). Give it a `hooks.json` whose `PreToolUse` hook runs a tiny script that appends stdin to a temp file. Make a temp work folder with a `docs/` subfolder and run, with `CODEX_HOME` set to the scratch folder: `codex exec "Create docs/plan.md containing '# Plan'"`. Delete the scratch `CODEX_HOME` after (it holds a copy of `auth.json`).

- [ ] **Step 2: Record what arrived**: for the `apply_patch` call, note in the commit message: the `tool_name`; whether the patch text is in `tool_input.command` (or another field); whether the `*** Add File:` path is relative or absolute; and what `cwd` is. Save the payload to the testdata file (placeholders for personal paths).

- [ ] **Step 3: Decide**:
  - Patch in `tool_input.command`, relative to `cwd`: Task 3 goes ahead as written.
  - Patch in another field or with a different tool name: Task 3 reads that field or name instead (and fix `toolSummary` to match in the same task).
  - No patch in the payload, or no `PreToolUse` for file edits at all: drop the Codex cases from Task 3, and note "Claude only" for the release notes in the PR description.

- [ ] **Step 4: Commit**
  `git commit -m "test(notify): Keep a real Codex apply_patch hook payload"`

---

### Task 3: The hook reports the `.md` paths of every tool call

**Files:**
- Create: `cmd/kaze/docs.go`
- Modify: `cmd/kaze/notify.go` (`runNotifyOutput` around :107-110)
- Test: `cmd/kaze/notify_docs_test.go`

**Interfaces:**
- Consumes: `notify.KindDoc`, `Msg.Docs`, `notify.MaxDocs` (Task 1); Task 2's verdict on the Codex field.
- Produces: `func docPaths(tool string, input json.RawMessage, cwd string) []string` in `cmd/kaze/docs.go`.

- [ ] **Step 1: Write the failing tests** in `cmd/kaze/notify_docs_test.go` (use `startServer`, `envOf` and `runNotify` from the existing tests; paths built from `t.TempDir()`):
  - `TestDocPathsClaudeTools`: table over `Write`, `Edit`, `MultiEdit`, `Read` with `{"file_path": <abs>/plan.md}` → `[<abs>/plan.md]`; `Write` of `<abs>/main.go` → nil; `Grep` with `{"pattern":"x","path":<abs dir>}` → nil; an MCP tool name with `{"path": <abs>/notes.md}` → `[<abs>/notes.md]`; `Read` with `{"file_path":"docs/a.md"}` and cwd `C` → `[filepath.Join(C, "docs/a.md")]`; relative path with empty cwd → nil.
  - `TestDocPathsCodexPatch`: `apply_patch` with `command` holding `*** Add File: docs/a.md`, `*** Update File: <abs>/b.md`, `*** Move to: docs/c.md`, `*** Delete File: docs/d.md`, `*** Update File: x.go` and cwd `C` → `[C/docs/a.md, <abs>/b.md, C/docs/c.md]`. Use the Task 2 payload file as one of the inputs.
  - `TestDocPathsCapped`: 12 `*** Add File:` `.md` headers → 8 paths.
  - `TestMainCallSendsDoc`: `runNotify([]string{"tool"}, <Claude Write payload with file_path abs .md, cwd>, envOf(srv, "3"))` → one message: `Kind == notify.KindDoc`, `Tab == 3`, `Docs == [abs]`.
  - `TestMainCallWithoutDocSendsNothing`: a main `Bash` call → no message within 300ms.
  - `TestSubagentToolCarriesDocs`: a `Write` with `agent_id: "a1"` → one `KindTool` message with `AgentID == "a1"`, its usual `Summary`, and `Docs == [abs]`.
  - `TestMainAgentCallUnchanged`: the existing `Agent` main call still sends `KindTool` with `Tool == "Agent"` (no `Docs`).

- [ ] **Step 2: Run to see them fail**
  Run: `go test ./cmd/kaze/ -run 'TestDocPaths|TestMainCall|TestSubagentToolCarriesDocs|TestMainAgentCallUnchanged' -v`
  Expected: FAIL (`docPaths` undefined).

- [ ] **Step 3: Implement `docPaths`** in `cmd/kaze/docs.go`: unmarshal `file_path`, `path` and `command`; take `file_path` and `path` if `.md`; for `apply_patch` (or the name Task 2 found) take the paths after `*** Add File: `, `*** Update File: ` and `*** Move to: ` on trimmed lines; join relative paths to `cwd` (drop them if `cwd` is empty), `filepath.Clean`, drop anything over `notify.MaxPath`, stop at `notify.MaxDocs`. Then wire it in `runNotifyOutput`: when `kind == notify.KindTool && parsed`, compute `docs := docPaths(hook.ToolName, hook.ToolInput, hook.Cwd)` before `agentMsg`. If `agentMsg` returns false and `len(docs) > 0`, set `msg.Kind = notify.KindDoc`, `msg.Docs = docs` and fall through to the session/cwd fields and `notify.Send` (skip the prompt/needs/reason branches, which don't apply). If `agentMsg` returns true and `msg.AgentID != ""`, set `msg.Docs = docs`.

- [ ] **Step 4: Run to see them pass**
  Run: `go test ./cmd/kaze/ -v`
  Expected: PASS, every existing hook test included.

- [ ] **Step 5: Commit**
  `git commit -m "feat(notify): Report the markdown files an agent touches"`

---

### Task 4: The per-tab doc list and its matcher

**Files:**
- Create: `internal/ui/doc_seen.go`
- Test: `internal/ui/doc_seen_test.go`

**Interfaces:**
- Produces:
  - `type docSeen struct { mu sync.Mutex; by map[tabs.TabID][]string; goos string }` (goos defaults to `runtime.GOOS`; tests set it).
  - `func (s *docSeen) add(id tabs.TabID, paths []string)`: newest first, dedup by `mdoc.PathKey(p, s.goos)`, cap 64.
  - `func (s *docSeen) find(id tabs.TabID, ref string) string`: newest path in that tab whose tail matches `ref` and that `os.Stat`s as a regular file; `""` if none.
  - `func (s *docSeen) findOthers(id tabs.TabID, order []tabs.TabID, ref string) string`: the same over every tab in `order` except `id`.
  - `func (s *docSeen) prune(list []*tabs.Tab)`: forget tabs not in list.
  - `func docTailMatch(recorded, ref, goos string) bool`.
  - All methods safe from any goroutine (`onNotify` writes, the UI reads).

- [ ] **Step 1: Write the failing tests**:
  - `TestDocMatchWholeComponents`: `docTailMatch(`/v/tickets/V3-1/plan.md`, `tickets/V3-1/plan.md`, "linux")` true; `plan.md` true; `/v/xtickets/a.md` vs `tickets/a.md` false; `../plan.md`, `a/../plan.md`, `""` false; `./tickets/V3-1/plan.md` true; a ref longer than the recorded path false.
  - `TestDocMatchCaseAndSlashes`: recorded `C:\v\Tickets\v3-1\Plan.md`, ref `tickets/V3-1/plan.md`: true for `"windows"` and `"darwin"`, false for `"linux"`. (Build the recorded path with the separator the test OS uses, so the test runs on every OS.)
  - `TestDocSeenNewestWins`: two existing files `A/plan.md` then `B/plan.md` added to tab 1; `find(1, "plan.md")` is B's; adding A again makes it A's.
  - `TestDocSeenCapAndDedup`: 70 distinct paths → 64 kept, the 6 oldest gone; the same path twice is listed once.
  - `TestDocSeenSkipsMissingFile`: newest match deleted from disk → `find` returns the older existing match; all deleted → `""`.
  - `TestDocSeenOthersAndPrune`: path added to tab 2; `find(1, ...)` is `""`; `findOthers(1, []tabs.TabID{1, 2}, ...)` finds it; after `prune` with only tab 1 alive, `findOthers` is `""`.
  - `TestDocSeenConcurrent`: `add` from 4 goroutines (100 adds each, bounded) while `find` runs; run with `-race`.

- [ ] **Step 2: Run to see them fail**
  Run: `go test -race ./internal/ui/ -run 'TestDocMatch|TestDocSeen' -v`
  Expected: FAIL (undefined).

- [ ] **Step 3: Implement** in `internal/ui/doc_seen.go`. `docTailMatch` splits both on `/` and `\`, drops empty and leading `.` components, refuses a ref with `..` or no components, and compares the ref's components with the recorded path's last ones (`strings.EqualFold` when goos is `windows` or `darwin`, `==` otherwise). Do the `os.Stat` outside the lock: copy the tab's slice under the lock, then stat.

- [ ] **Step 4: Run to see them pass**
  Run: `go test -race ./internal/ui/ -run 'TestDocMatch|TestDocSeen' -v`
  Expected: PASS.

- [ ] **Step 5: Commit**
  `git commit -m "feat(ui): Remember the markdown files each tab's agent touched"`

---

### Task 5: Fill the list from hook messages

**Files:**
- Modify: `internal/ui/window.go` (add `seen *docSeen` next to `agents` near :169, initialised in the constructor near :231), `internal/ui/alerts_ui.go` (`onNotify` :129), `internal/ui/agent.go` (`pruneAgents` :132)
- Test: `internal/ui/doc_seen_test.go`

**Interfaces:**
- Consumes: `docSeen.add`, `docSeen.prune` (Task 4); `notify.KindDoc`, `Msg.Docs` (Task 1).

- [ ] **Step 1: Write the failing tests** (use `testWindow` and `win.mgr.New()` like `docs_open_test.go`; set `win.liveMgr` the way existing `onNotify` tests do):
  - `TestDocMsgRecordsPath`: `win.onNotify(notify.Msg{Tab: int(a.ID), Kind: notify.KindDoc, Docs: []string{p}})` with `p` an existing temp `.md` → `win.seen.find(a.ID, filepath.Base(p)) == p`.
  - `TestSubagentToolDocsRecorded`: a `KindTool` with `AgentID: "a1"` and `Docs` → recorded on the tab, and the subagent tool event still reaches the tracker as before.
  - `TestDocMsgTouchesNoAgentState`: after a `KindDoc` message, the tab has no `win.agents` entry, no alert, mark or toast, no `subIn` events, and `win.restore` got no note (check the same fields the existing `KindPrompt`/`KindUsage` onNotify tests check).
  - `TestDocMsgUnknownTabIgnored`: a `KindDoc` for a tab that doesn't exist records nothing and doesn't panic.
  - `TestClosedTabForgetsDocs`: close the tab, run the prune path → `findOthers` no longer finds its path.

- [ ] **Step 2: Run to see them fail**
  Run: `go test ./internal/ui/ -run 'TestDocMsg|TestSubagentToolDocsRecorded|TestClosedTabForgetsDocs' -v`
  Expected: FAIL.

- [ ] **Step 3: Implement**: at the top of `onNotify`, before `originIdentity`, if `len(m.Docs) > 0` and the tab exists (`mgr.Lookup`), call `win.seen.add(id, m.Docs)`. If `m.Kind == notify.KindDoc`, return right after (no invalidate needed: nothing on screen changed). Call `win.seen.prune(list)` from `pruneAgents`.

- [ ] **Step 4: Run to see them pass**
  Run: `go test ./internal/ui/ -v`
  Expected: PASS, all existing UI tests included.

- [ ] **Step 5: Commit**
  `git commit -m "feat(ui): Record doc paths from the agent hooks"`

---

### Task 6: Clicks check the list before guessing

**Files:**
- Modify: `internal/ui/docs_open.go` (`openDocRefs` :55-90), `internal/ui/doc_links.go` (`followDocLink`)
- Modify: `README.md` (the markdown reader section: one sentence that links to files the agent wrote or read anywhere now open), `docs/HANDOFF.md` (known gaps: shell-only writes fall back to guessing; the list is in memory only)
- Test: `internal/ui/docs_open_test.go`, `internal/ui/doc_links_test.go`

**Interfaces:**
- Consumes: `docSeen.find`, `docSeen.findOthers` (Task 4); tab order from `win.mgr` (whatever list `pruneAgents` receives).

- [ ] **Step 1: Write the failing tests**:
  - `TestClickOpensVaultPlanTheAgentWrote` (the owner's case): tab `a` with `a.Dir` = temp repo folder, `win.topFn` returning `""`; `vault := t.TempDir()`; `p := writeDoc(t, vault, "tickets/V3-3154/plan.md", "# Plan")`; `win.seen.add(a.ID, []string{p})`; `win.openDocRefs(a, false, []string{"tickets/V3-3154/plan.md"})` → `win.shownDoc().path == p`. Without the `add`, the notice stays `"Can't find plan.md in " + tildeHome(a.Dir, win.home)` (today's miss is unchanged).
  - `TestClickPrefersTouchedOverRepoCopy`: the repo has `docs/plan.md` and the list has `<vault>/docs/plan.md` → the vault one opens.
  - `TestClickAbsoluteStillFirst`: an absolute existing ref opens even when the list holds a different match.
  - `TestClickFallsBackToRepoWhenListMisses`: list holds an unrelated path → `docs/a.md` in the repo opens (as `TestDocPathFallsBackToTheRepoRoot`).
  - `TestClickUsesOtherTabsLast`: path recorded on tab `b`, clicked from tab `a`, repo has no such file → opens; if the repo has the file, the repo one opens.
  - `TestDocLinkUsesTouchedList`: in `doc_links_test.go`, a doc at `<x>/a.md` follows `tickets/V3-1/plan.md` that only exists at `<vault>/tickets/V3-1/plan.md` recorded on `d.session` → opens it; an unrecorded miss still notes `"Can't find ..."`.

- [ ] **Step 2: Run to see them fail**
  Run: `go test ./internal/ui/ -run 'TestClick|TestDocLink' -v`
  Expected: the new tests FAIL; the existing click tests still PASS.

- [ ] **Step 3: Implement** the order from spec 4.3 in `openDocRefs`: (1) absolute refs as today; (2) each relative ref via `win.seen.find(t.ID, ref)` (use the tab that was clicked, `t.ID`; the doc still opens under `docParent`'s id); (3) relative refs via `mdoc.Resolve` as today; (4) each relative ref via `win.seen.findOthers(t.ID, order, ref)`; (5) the existing miss note, unchanged. Each step goes through all refs before the next. In `followDocLink`, after the doc-relative `Resolve` miss: `find(d.session, t.Path)`, then `findOthers`, then the existing note. Update README and HANDOFF.

- [ ] **Step 4: Run everything**
  Run: `go test ./... && go vet ./...`
  Expected: PASS, no vet findings.

- [ ] **Step 5: Commit**
  `git commit -m "fix(ui): Open the markdown file the agent touched when a link is clicked"`

---

## After the tasks

- Whole-branch review, then push `feat/md-link-hook` and open a PR that references the issue. The owner merges.
- After the merge, the release is **0.28.1** (spec section 6) per `CLAUDE.md` "Shipping a feature": version bump RED first, changelog row and lander "What's new" entry ("Links to markdown files the agent wrote or read anywhere (a vault, another repo) now open in the reader."), no screenshot changes. If Task 2 found Codex sends no patch, say "Claude" instead of "the agent" in that line.
