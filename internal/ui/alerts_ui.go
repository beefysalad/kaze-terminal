package ui

import (
	"cmp"
	"image"
	"image/color"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/beefysalad/kaze-terminal/internal/notify"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// toastHit is where an alert card and its × were drawn, in window pixels.
type toastHit struct {
	id              tabs.TabID
	run             uint64
	rect, closeRect image.Rectangle
}

// inbox carries alerts from notify goroutines to the UI goroutine. It keeps
// only what changes the outcome per tab (its latest alert, its latest "needs"
// and its latest prompt), so a burst while minimized, when no frames run, can
// neither block the sender nor lose a tab's last word.
type inbox struct {
	mu    sync.Mutex
	seq   uint64
	byTab map[tabs.TabID]*pending
}

type queued struct {
	m     notify.Msg
	seq   uint64
	shell uint64    // shellSeq of the tab when the message arrived
	at    time.Time // when it arrived; zero means the frame's time
}

type pending struct {
	needs, prompt *queued
	last          queued
}

// put queues m; shell is the tab's shellSeq now, at arrival.
func (b *inbox) put(m notify.Msg, shell uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := tabs.TabID(m.Tab)
	if b.byTab == nil {
		b.byTab = map[tabs.TabID]*pending{}
	}
	p := b.byTab[id]
	if p == nil {
		p = &pending{}
		b.byTab[id] = p
	}
	q := queued{m, b.seq, shell, time.Now()}
	switch m.Kind {
	case notify.KindNeeds:
		p.needs = &q
	case notify.KindPrompt:
		if m.Command == "" && p.prompt != nil {
			// A plain prompt keeps the shown command, so carry it as delivering both would.
			if q.m.Agent == p.prompt.m.Agent {
				q.m.Command, q.m.Args = p.prompt.m.Command, p.prompt.m.Args
			}
		}
		p.prompt = &q
	}
	p.last = q
}

// items is the tab's kept messages in arrival order.
func (p *pending) items() []queued {
	out := []queued{p.last}
	for _, q := range []*queued{p.needs, p.prompt} {
		if q != nil && q.seq != p.last.seq {
			out = append(out, *q)
		}
	}
	slices.SortFunc(out, func(a, b queued) int { return cmp.Compare(a.seq, b.seq) })
	return out
}

// take empties the inbox and returns the alerts in arrival order.
func (b *inbox) take() []queued {
	b.mu.Lock()
	all := b.byTab
	b.byTab = nil
	b.mu.Unlock()
	var qs []queued
	for _, p := range all {
		qs = append(qs, p.items()...)
	}
	slices.SortFunc(qs, func(a, b queued) int { return cmp.Compare(a.seq, b.seq) })
	return qs
}

func (b *inbox) size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, p := range b.byTab {
		n += len(p.items())
	}
	return n
}

// publish mirrors the active tab and focus for onNotify, which runs off the
// UI goroutine. The UI goroutine calls it whenever either may have changed.
func (win *window) publish(active *tabs.Tab, focused bool) {
	var id tabs.TabID
	if active != nil {
		id = active.ID
	}
	win.snapActive.Store(int64(id))
	win.snapFocused.Store(focused)
}

// onNotify runs on a notify connection goroutine and must not block. The
// chime and taskbar flash are decided here, not on a frame: Gio sends no
// frames to a minimized window, which is exactly when the owner is away.
func (win *window) onNotify(m notify.Msg) {
	id := tabs.TabID(m.Tab)
	src := m.Tab // the tab the hook ran in: subagent events keep it as their source
	var t *tabs.Tab
	if mgr := win.liveMgr.Load(); mgr != nil {
		// The docs list comes first: a KindDoc is only that, never agent,
		// restore, subagent or alert state, and needs no frame.
		if len(m.Docs) > 0 && mgr.Lookup(id) != nil {
			win.seen.add(id, m.Docs)
		}
		if m.Kind == notify.KindDoc {
			return
		}
		if m.Kind != notify.KindUsage && !subKind(m.Kind) {
			if st := mgr.Lookup(id); st != nil {
				win.originIdentity(m, st)
				win.terms.note(id, m.Agent, shellSeq(st))
			}
		}
		t = mgr.Lookup(id)
	}
	if t == nil {
		return // closed or unknown tab
	}
	// Taken now, so a shell prompt that comes back before the next frame still
	// counts as after this message: it hides ctx and ends Claude.
	shell := shellSeq(t)
	if m.Kind != notify.KindUsage {
		win.subIn.put(eventsOf(m, src, time.Now())...)
	}
	if subKind(m.Kind) {
		if win.w != nil {
			win.w.Invalidate() // state only: never a chime, flash, toast or mark
		}
		return
	}
	if m.Kind == notify.KindUsage {
		// Never an alert: no chime, flash or mark.
		if m.Usage != nil {
			win.usageIn.put(id, *m.Usage, cwdSeq(t), shell)
		}
		if win.w != nil {
			win.w.Invalidate()
		}
		return
	}
	if m.Kind == notify.KindPrompt || m.Kind == notify.KindInterrupt || m.Kind == notify.KindSession {
		win.inbox.put(m, shell) // state only: never a chime, flash or toast
		if win.w != nil {
			win.w.Invalidate()
		}
		return
	}
	focused := win.snapFocused.Load()
	if id != tabs.TabID(win.snapActive.Load()) || !focused {
		// The settings page changes these on the UI goroutine.
		win.alertMu.Lock()
		cfg, chime := &win.opt.Config, win.chime // only these fields: others change unlocked
		ring := m.Kind == notify.KindNeeds && cfg.ChimeNeeds || m.Kind == notify.KindDone && cfg.ChimeDone
		flash := m.Kind == notify.KindNeeds && !focused && cfg.TaskbarFlash
		win.alertMu.Unlock()
		if chime != nil && ring {
			chime(time.Now()) // non-blocking; the player coalesces bursts
		}
		if flash && win.flash != nil {
			win.flash()
		}
	}
	win.inbox.put(m, shell)
	if win.w != nil {
		win.w.Invalidate()
	}
}

// drainAlerts applies every queued alert; UI goroutine only.
func (win *window) drainAlerts(now time.Time) {
	for _, q := range win.inbox.take() {
		win.deliver(q, now)
	}
}

// deliver updates the agent state, marks and toasts for one alert; the chime
// already happened in onNotify.
func (win *window) deliver(q queued, now time.Time) {
	m := q.m
	var active tabs.TabID // none while settings hides the terminal
	if t := win.shownTab(); t != nil {
		active = t.ID
	}
	var ran time.Duration
	if t := win.mgr.Lookup(tabs.TabID(m.Tab)); t != nil {
		win.noteAgent(m, t.ID, q.shell)
		ran = win.noteTurn(t.ID, m.Kind, cmp.Or(q.at, now))
		win.wantCount(t)
		win.staleBranch(t) // a background row has no other reason to re-read its branch
	}
	win.alerts.doneToast = win.opt.Config.DoneToast
	if _, shown := win.alerts.apply(m, active, win.focused, win.mgr.Has, now); shown && ran > 0 {
		win.alerts.toasts[len(win.alerts.toasts)-1].Ran = ran // apply appends the tab's card last
	}
}

// noteTurn keeps when the tab's turn began: a prompt starts it, a done or an
// interrupt ends it, and a done returns how long it ran (0 when unknown).
func (win *window) noteTurn(id tabs.TabID, k notify.Kind, at time.Time) (ran time.Duration) {
	a, ok := win.agents[id]
	if !ok {
		return 0
	}
	switch k {
	case notify.KindPrompt:
		a.since = at
	case notify.KindDone, notify.KindInterrupt:
		if !a.since.IsZero() && at.After(a.since) && k == notify.KindDone {
			ran = at.Sub(a.since)
		}
		a.since = time.Time{}
	default:
		return 0
	}
	win.agents[id] = a
	return ran
}

// selectTab is the one tab switch for a tab click and an alert card's "Open session".
func (win *window) selectTab(id tabs.TabID) {
	win.hideDoc()
	win.dropSideEdit()
	win.hideSettings()
	win.closeBoard()
	win.mgr.SetActive(id)
	win.wantFocus = true
}

// visit clears the shown tab's alert, but only when the owner can see it.
func (win *window) visit(t *tabs.Tab) {
	if t != nil && win.focused {
		win.alerts.visited(t.ID)
	}
}

// toastEvent handles a press on a card: × dismisses it, anywhere else on it
// opens the session, as its "Open session" says.
func (win *window) toastEvent(e pointer.Event) {
	if e.Kind != pointer.Press || !e.Buttons.Contain(pointer.ButtonPrimary) || win.bench.IgnoreInput() {
		return
	}
	p := e.Position.Round()
	for _, h := range win.ui.toastHits {
		switch {
		case p.In(h.closeRect):
			win.alerts.dismiss(h.id, h.run)
			win.wantFocus = true
			win.w.Invalidate()
			return
		case p.In(h.rect):
			win.toastGo(h.id)
			return
		}
	}
}

// fitLabel shortens s from the right until it fits room; empty when nothing
// does. Bounded so a bad measure cannot spin.
func (c *chrome) fitLabel(gtx layout.Context, s string, room int, col color.NRGBA) (op.CallOp, image.Point) {
	call, sz := c.label(gtx, s, 12, col)
	for i := 0; i < 8 && sz.X > room && s != ""; i++ {
		n := utf8.RuneCountInString(s)
		keep := min(n*max(room, 0)/max(sz.X, 1), n-1)
		if keep <= 1 {
			s = ""
		} else {
			s = truncateRight(s, keep)
		}
		call, sz = c.label(gtx, s, 12, col)
	}
	if sz.X > room {
		call, sz = c.label(gtx, "", 12, col)
		sz.X = 0
	}
	return call, sz
}

// truncateRight keeps the first max runes and ends with "…".
func truncateRight(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}
