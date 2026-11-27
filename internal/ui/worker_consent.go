package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/op"
	ctl "github.com/beefysalad/kaze-terminal/internal/control"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/orchestrate"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

const workerConsentTimeout = time.Minute

type workerConsent struct {
	request                      *controlWorkerRequest
	deadline                     time.Time
	attention                    bool
	title, task, question, allow string
}
type workerConsentState struct {
	pending map[tabs.TabID]*workerConsent
	allowed map[tabs.TabID]bool
	tag     int
	shown   tabs.TabID
	hits    []bannerHit
	drawOps op.Ops
	call    op.CallOp
	cache   consentDrawKey
}

// holdWorkerConsent is called only by the UI drain. Returning true means this
// method owns the reply, including refusals. The normal apply path validates
// again after Allow, using current folders, names, limits and feature state.
func (win *window) holdWorkerConsent(r *controlWorkerRequest) bool {
	mode := win.opt.Config.Orchestrator.Confirm
	if r.req.Op != "spawn" || mode == "never" || (mode != "always" && win.consent.allowed[parseWorkerID(r.req.From)]) {
		return false
	}
	if !win.orchestratorEnabled || win.mgr == nil {
		return false
	}
	parent := win.mgr.Lookup(parseWorkerID(r.req.From))
	if parent == nil || workerID(parent.ID) != r.req.From || parent.Parent != 0 || parent.Role != "" {
		return false
	}
	win.consentTick(win.wsNow())
	if win.consent.pending[parent.ID] != nil || len(win.consent.pending) >= 4 {
		r.done <- workerFailure(ctl.CodeBusy, "worker consent is already pending; retry later")
		return true
	}
	folder := r.req.CWD
	if folder == "" {
		folder = parent.Dir
		parent.Session.Lock()
		if cwd := parent.Session.Applier().Cwd(); cwd != "" {
			folder = cwd
		}
		parent.Session.Unlock()
	}
	if folder == "" {
		folder = win.opt.Dir
	}
	agent := cleanReason(r.req.Agent)
	resolved, _, err := orchestrate.ResolveTemplate(win.templateFolder(), orchestrate.Request{Agent: r.req.Agent})
	if err == nil {
		if resolved.Agent == "codex" {
			agent = "Codex"
		} else {
			agent = "Claude Code"
		}
		if resolved.Template != "" {
			agent += " (" + resolved.Template + ")"
		}
	}
	fromAgent := win.rowAgent(parent.ID).agentName()
	if fromAgent == "" {
		fromAgent = "shell"
	}
	first, _, _ := strings.Cut(r.req.Task, "\n")
	c := &workerConsent{request: r, deadline: win.wsNow().Add(workerConsentTimeout),
		title: fmt.Sprintf("%s (%s) wants to start a %s worker in %s", r.req.From, fromAgent, agent, cleanReason(filepath.Base(folder))),
		task:  cleanReason(first), question: "Allow workers from " + r.req.From + " for this run?", allow: "Allow for this session"}
	if mode == "always" {
		c.question = "Allow this worker from " + r.req.From + "?"
		c.allow = "Allow"
	}
	if win.consent.pending == nil {
		win.consent.pending = make(map[tabs.TabID]*workerConsent)
	}
	win.consent.pending[parent.ID] = c
	if r.consenting != nil {
		close(r.consenting)
	}
	// Workspace banners use the in-app alert path, with no separate system
	// notification mechanism. Preserve agent status; consent is not a hook.
	var active tabs.TabID
	if t := win.shownTab(); t != nil {
		active = t.ID
	}
	_, c.attention = win.alerts.apply(notify.Msg{Kind: notify.KindNeeds, Tab: int(parent.ID), Reason: "Worker consent: " + c.task}, active, win.focused, win.mgr.Has, win.wsNow())
	win.publishControl()
	return true
}
func (win *window) finishWorkerConsent(id tabs.TabID, reply ctl.Reply) {
	c := win.consent.pending[id]
	if c == nil {
		return
	}
	delete(win.consent.pending, id)
	// The toast may have been evicted from the bounded list. Ownership of
	// the mark lasts until a real hook supersedes it, independent of the card.
	if c.attention {
		newer := false
		for _, alert := range win.alerts.toasts {
			if alert.Tab == id && alert.Run == 0 && alert.Text != cleanReason("Worker consent: "+c.task) {
				newer = true
			}
		}
		if !newer {
			win.alerts.visited(id)
		}
	}
	win.consent.cache = consentDrawKey{}
	if win.mgr != nil {
		win.publishControl()
	}
	c.request.done <- reply
}
func (win *window) closeWorkerConsent(id tabs.TabID) {
	win.finishWorkerConsent(id, workerFailure(ctl.CodeNotAllowed, "parent tab is closed"))
	delete(win.consent.allowed, id)
}
func (win *window) answerWorkerConsent(id tabs.TabID, allow bool) {
	win.consentTick(win.wsNow())
	c := win.consent.pending[id]
	if c == nil {
		return
	}
	if !allow {
		win.finishWorkerConsent(id, workerFailure("consent_denied", "worker request denied"))
		return
	}
	if win.opt.Config.Orchestrator.Confirm != "always" {
		if win.consent.allowed == nil {
			win.consent.allowed = make(map[tabs.TabID]bool)
		}
		win.consent.allowed[id] = true
	}
	// No stale validation result is carried across the owner's decision.
	ctx, cancel := context.WithTimeout(c.request.ctx, controlWorkerTimeout)
	defer cancel()
	win.finishWorkerConsent(id, win.applyWorkerContext(ctx, c.request.req))
}
func (win *window) consentTick(now time.Time) (wake time.Time, ok bool) {
	for id, c := range win.consent.pending {
		if win.mgr == nil || !win.mgr.Has(id) {
			win.closeWorkerConsent(id)
			continue
		}
		if c.request.ctx.Err() != nil {
			win.finishWorkerConsent(id, workerFailure("timeout", "worker request canceled"))
			continue
		}
		if !now.Before(c.deadline) {
			win.finishWorkerConsent(id, workerFailure("consent_timeout", "worker consent was not answered within 60 seconds"))
			continue
		}
		if !ok || c.deadline.Before(wake) {
			wake, ok = c.deadline, true
		}
	}
	return
}

// Hooks can replace a consent mark even when their toast is later evicted.
func (win *window) consentHook(id tabs.TabID, kind notify.Kind) {
	if c := win.consent.pending[id]; c != nil {
		switch kind {
		case notify.KindNeeds, notify.KindDone, notify.KindPrompt:
			c.attention = false
		}
	}
}
