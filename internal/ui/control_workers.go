package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/orchestrate"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const controlWorkerTimeout = 5 * time.Second

type worker struct {
	parent              tabs.TabID
	started, sessionAt  time.Time
	firstMessage, agent string
	hooked              bool
	exited              bool
	worked              bool
	needs               bool
	noted               map[string]bool
}
type controlWorkerRequest struct {
	ctx  context.Context
	req  ctl.Request
	done chan ctl.Reply
}

func (h *controlHost) Spawn(ctx context.Context, req ctl.Request) ctl.Reply {
	req.Op = "spawn"
	req.Labels = cloneLabels(req.Labels)
	reply := h.queueWorker(ctx, req)
	if !reply.OK || !req.WaitStart || len(reply.Tabs) == 0 {
		return reply
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	id := reply.Tabs[0].ID
	for {
		infos, changed := h.StatusChanges()
		found := false
		for _, tab := range infos {
			if tab.ID == id {
				found = true
				if tab.FirstMessage == "delivered" {
					reply.Tabs = []ctl.TabInfo{tab}
					return reply
				}
				if tab.FirstMessage == "failed" || tab.FirstMessage == "unconfirmed" || tab.Status == "exited" {
					return workerFailure(ctl.CodeSpawnFailed, "first message was not confirmed")
				}
			}
		}
		if !found {
			return workerFailure(ctl.CodeSpawnFailed, "worker closed before first message")
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return workerFailure("timeout", "first message was not confirmed within 30 seconds")
		}
	}
}
func (h *controlHost) Kill(ctx context.Context, from, id string) ctl.Reply {
	return h.queueWorker(ctx, ctl.Request{Op: "kill", From: from, Tab: id})
}
func workerFailure(code, message string) ctl.Reply {
	return ctl.Reply{Error: &ctl.Error{Code: code, Message: message}}
}
func workerError(err error) ctl.Reply {
	if e, ok := err.(*orchestrate.Error); ok {
		return workerFailure(e.Code, e.Message)
	}
	return workerFailure(ctl.CodeSpawnFailed, "could not start worker")
}

// queueWorker accesses only host-owned synchronization and immutable snapshots.
// The sole callback requests a frame; all window mutation happens in the drain.
func (h *controlHost) queueWorker(ctx context.Context, req ctl.Request) ctl.Reply {
	ctx, cancel := context.WithTimeout(ctx, controlWorkerTimeout)
	defer cancel()
	if ctx.Err() != nil {
		return workerFailure("timeout", "worker request canceled")
	}
	r := &controlWorkerRequest{ctx: ctx, req: req, done: make(chan ctl.Reply, 1)}
	h.spawnMu.Lock()
	if len(h.workerRequests) >= 8 {
		h.spawnMu.Unlock()
		return workerFailure(ctl.CodeBusy, "worker queue is full")
	}
	h.workerRequests = append(h.workerRequests, r)
	h.spawnMu.Unlock()
	defer func() {
		h.spawnMu.Lock()
		if i := slices.Index(h.workerRequests, r); i >= 0 {
			h.workerRequests = slices.Delete(h.workerRequests, i, i+1)
		}
		h.spawnMu.Unlock()
	}()
	if h.wake != nil {
		h.wake()
	}
	select {
	case reply := <-r.done:
		return reply
	case <-ctx.Done():
		return workerFailure("timeout", "worker request was not applied within 5 seconds")
	}
}
func (win *window) drainWorkers() {
	h := &win.control
	h.spawnMu.Lock()
	pending := h.workerRequests
	h.workerRequests = nil
	h.spawnMu.Unlock()
	for _, r := range pending {
		if r.ctx.Err() != nil {
			continue
		}
		r.done <- win.applyWorkerContext(r.ctx, r.req)
	}
}

type workerFS struct{}

func (workerFS) EvalSymlinks(p string) (string, error) { return filepath.EvalSymlinks(p) }
func (workerFS) Stat(p string) (os.FileInfo, error)    { return os.Stat(p) }
func workerID(id tabs.TabID) string                    { return "t" + strconv.Itoa(int(id)) }
func parseWorkerID(id string) tabs.TabID {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "t"))
	return tabs.TabID(n)
}

// workerRoot walks only ancestors of the parent's folder, never a home lookup.
// A .git file also identifies the root of a linked worktree.
func workerRoot(folder string) string {
	for dir := folder; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return folder
		}
	}
}
func (win *window) applyWorker(req ctl.Request) ctl.Reply {
	return win.applyWorkerContext(context.Background(), req)
}
func (win *window) applyWorkerContext(ctx context.Context, req ctl.Request) ctl.Reply {
	if ctx.Err() != nil {
		return workerFailure("timeout", "worker request canceled")
	}
	if win.mgr == nil {
		return workerFailure(ctl.CodeUnknownTab, "parent tab is closed")
	}
	parent := win.mgr.Lookup(parseWorkerID(req.From))
	if parent == nil || workerID(parent.ID) != req.From || parent.Parent != 0 || parent.Role != "" {
		return workerFailure(ctl.CodeNotAllowed, "only owner-opened tabs can manage workers")
	}
	if req.Op == "kill" {
		tab := win.mgr.Lookup(parseWorkerID(req.Tab))
		if tab == nil {
			return workerFailure(ctl.CodeUnknownTab, "unknown worker")
		}
		if tab.Role != "worker" || tab.Parent != parent.ID {
			return workerFailure(ctl.CodeNotAllowed, "only your own workers can be closed")
		}
		win.closeTab(tab.ID)
		return ctl.Reply{OK: true, Text: "closed " + req.Tab}
	}
	if !win.orchestratorEnabled {
		return workerFailure(ctl.CodeSpawnDisabled, "worker spawning is disabled")
	}
	folder := parent.Dir
	parent.Session.Lock()
	if cwd := parent.Session.Applier().Cwd(); cwd != "" {
		folder = cwd
	}
	parent.Session.Unlock()
	if folder == "" {
		folder = win.opt.Dir
	}
	canonicalFolder, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return workerFailure(ctl.CodeBadCWD, "cannot resolve parent folder")
	}
	folder = canonicalFolder
	r, err := orchestrate.Validate(orchestrate.Request{Agent: req.Agent, CWD: req.CWD, Name: req.Name, Task: req.Task, Mode: req.Mode}, orchestrate.Parent{Folder: folder, RepoRoot: workerRoot(folder)}, workerFS{})
	if err != nil {
		return workerError(err)
	}
	if len(req.Labels) > 8 {
		return workerFailure(ctl.CodeInvalidRequest, "at most 8 labels")
	}
	for k, v := range req.Labels {
		if e := ctl.ValidateLabel(k, v); e != nil {
			return ctl.Reply{Error: e}
		}
	}
	if r.Name != "" {
		if _, e := ctl.ValidateName(win.control.Tabs(), "", r.Name); e != nil {
			return ctl.Reply{Error: e}
		}
	}
	now := win.wsNow()
	usage := orchestrate.Usage{}
	for id, w := range win.workers {
		if t := win.mgr.Lookup(id); t != nil {
			exited, _ := t.Session.Exited()
			if !exited {
				usage.TotalWorkers++
				if w.parent == parent.ID {
					usage.ParentWorkers++
				}
			}
		}
	}
	starts := win.workerStarts[parent.ID]
	starts = slices.DeleteFunc(starts, func(t time.Time) bool { return !t.After(now.Add(-time.Minute)) })
	usage.Starts = starts
	if err := orchestrate.CheckLimits(orchestrate.Limits{}, usage, func() time.Time { return now }); err != nil {
		return workerError(err)
	}
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return workerError(err)
	}
	uuid[6] = (uuid[6] & 15) | 64
	uuid[8] = (uuid[8] & 63) | 128
	nextID := win.mgr.NextID()
	opt := orchestrate.LaunchOptions{GOOS: runtime.GOOS, SessionID: fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), Identity: orchestrate.Identity{ParentID: req.From, ParentName: win.names[parent.ID], WorkerID: workerID(nextID), WorkerName: r.Name}}
	if opt.Identity.ParentName == "" {
		opt.Identity.ParentName = parent.FixedTitle
	}
	if len(win.opt.Shell) > 0 {
		opt.Shell = win.opt.Shell[0]
	}
	if runtime.GOOS == "windows" {
		env := os.Environ()
		if win.opt.ShellEnv != nil {
			env = win.opt.ShellEnv(env)
		}
		if r.Agent == "claude" {
			opt.ClaudeExe, err = orchestrate.ResolveWindowsClaude(env, workerFS{})
			if err != nil {
				return workerError(err)
			}
		} else {
			for _, e := range env {
				if key, v, ok := strings.Cut(e, "="); ok && strings.EqualFold(key, "KAZE_CODEX_SHIM") {
					opt.CodexLauncher = filepath.Join(v, "codex.exe")
				}
			}
		}
	}
	plan, err := orchestrate.Plan(r, opt)
	if err != nil {
		return workerError(err)
	}
	plan.Env = append(plan.Env, "KAZE_PARENT="+workerID(parent.ID), "KAZE_ROLE=worker")
	if ctx.Err() != nil {
		return workerFailure("timeout", "worker request canceled")
	}
	tab, err := win.mgr.NewOpts(tabs.StartOpts{Argv: plan.Argv, Env: plan.Env, Dir: r.CWD, Title: r.Name, Background: true, Parent: parent.ID, Role: "worker"})
	if err != nil {
		return workerError(err)
	}
	if ctx.Err() != nil {
		win.closeTab(tab.ID)
		return workerFailure("timeout", "worker request canceled during start")
	}
	if tab.ID != nextID {
		win.closeTab(tab.ID)
		return workerFailure(ctl.CodeSpawnFailed, "worker identity changed during start")
	}
	if win.workers == nil {
		win.workers = map[tabs.TabID]*worker{}
	}
	win.workers[tab.ID] = &worker{parent: parent.ID, started: now, firstMessage: "pending", agent: r.Agent}
	if win.workerStarts == nil {
		win.workerStarts = map[tabs.TabID][]time.Time{}
	}
	win.workerStarts[parent.ID] = append(starts, now)
	win.control.labelsMu.Lock()
	if win.control.labels == nil {
		win.control.labels = map[string]map[string]string{}
	}
	win.control.labels[workerID(tab.ID)] = cloneLabels(req.Labels)
	win.control.labelsMu.Unlock()
	win.sideRedraw()
	win.publishControl()
	for _, info := range win.control.Tabs() {
		if info.ID == workerID(tab.ID) {
			return ctl.Reply{OK: true, Text: "started " + info.ID + " (" + r.Agent + ")", Tabs: []ctl.TabInfo{info}}
		}
	}
	return workerFailure(ctl.CodeSpawnFailed, "worker snapshot unavailable")
}

func (win *window) workerHook(id tabs.TabID, kind notify.Kind) {
	w := win.workers[id]
	if w == nil {
		return
	}
	// Honor an elapsed deadline or process exit before accepting a late hook.
	win.workerTick(win.wsNow())
	w.hooked = true
	switch kind {
	case notify.KindPrompt:
		w.worked = true
		w.needs = false
		clear(w.noted)
	case notify.KindNeeds:
		w.needs = true
		win.workerNote(id, w, "blocked")
	case notify.KindDone, notify.KindInterrupt:
		w.needs = false
		if w.worked {
			win.workerNote(id, w, "done")
		}
	}
	if w.firstMessage != "pending" {
		return
	}
	if kind == notify.KindPrompt {
		w.firstMessage = "delivered"
	} else if kind == notify.KindSession && w.sessionAt.IsZero() {
		w.sessionAt = win.wsNow()
	}
}

// workerTick returns a deadline only for a pending first message or inbox delivery.
func (win *window) workerTick(now time.Time) (wake time.Time, ok bool) {
	changed := false
	for id, w := range win.workers {
		tab := win.mgr.Lookup(id)
		exited := tab == nil
		if tab != nil {
			exited, _ = tab.Session.Exited()
		}
		if exited != w.exited {
			w.exited = exited
			if exited {
				win.workerNote(id, w, "exited")
			}
			changed = true
		}
		if w.firstMessage != "pending" {
			continue
		}
		if exited {
			w.firstMessage = "failed"
			win.workerNote(id, w, "not_started")
			changed = true
			continue
		}
		due := w.started.Add(30 * time.Second)
		state := "failed"
		if !w.sessionAt.IsZero() {
			due = w.sessionAt.Add(20 * time.Second)
			state = "unconfirmed"
		}
		if !now.Before(due) {
			w.firstMessage = state
			win.workerNote(id, w, "not_started")
			changed = true
			continue
		}
		if !ok || due.Before(wake) {
			wake, ok = due, true
		}
	}
	if at, ready := win.noteTick(now, nil); ready {
		if !at.After(now) {
			win.noteTick(now, win.control.submitNote)
		} else if !ok || at.Before(wake) {
			wake, ok = at, true
		}
	}
	if changed {
		win.publishControl()
	}
	return
}

// closeWorkers runs on the UI goroutine before the normal parent close path.
func (win *window) closeWorkers(parent tabs.TabID) {
	if w := win.workers[parent]; w != nil {
		win.workerNote(parent, w, "exited")
	}
	for id, w := range win.workers {
		if w.parent == parent {
			win.closeTab(id)
		}
	}
	delete(win.workers, parent)
	delete(win.workerStarts, parent)
}
