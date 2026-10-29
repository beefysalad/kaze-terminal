package ui

import (
	"cmp"
	"image/color"
	"slices"
	"sync"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
	"github.com/patrick-gogo/kaze-terminal/internal/usage"
)

var colUsageHigh = rgb(0xff6b6b)

// usageBox hands usage reports from notify goroutines to the UI goroutine. It
// is apart from the alert inbox, which keeps one message per tab, so a report
// can never replace a pending alert. Only the latest report per tab matters.
type usageBox struct {
	mu    sync.Mutex
	seq   uint64
	byTab map[tabs.TabID]usageReport
}

type usageReport struct {
	tab tabs.TabID
	u   notify.Usage
	seq uint64
}

func (b *usageBox) put(tab tabs.TabID, u notify.Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	if b.byTab == nil {
		b.byTab = map[tabs.TabID]usageReport{}
	}
	b.byTab[tab] = usageReport{tab, u, b.seq}
}

// take empties the box, oldest report first.
func (b *usageBox) take() []usageReport {
	b.mu.Lock()
	all := b.byTab
	b.byTab = nil
	b.mu.Unlock()
	out := make([]usageReport, 0, len(all))
	for _, r := range all {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b usageReport) int { return cmp.Compare(a.seq, b.seq) })
	return out
}

// ctxEntry is a tab's context use and the folder-report count when it came;
// a later report means the shell prompt is back, so Claude has exited.
type ctxEntry struct {
	pct    float64
	cwdSeq uint64
}

// drainUsage applies queued reports; UI goroutine only. Plan limits are
// account-wide, so the newest report that has them wins whichever tab sent it.
func (win *window) drainUsage() {
	for _, r := range win.usageIn.take() {
		if r.u.FiveHour != nil || r.u.Week != nil {
			win.acct = notify.Usage{FiveHour: r.u.FiveHour, Week: r.u.Week}
		}
		t := win.tabByID(r.tab)
		if t == nil {
			continue
		}
		if r.u.Ctx == nil {
			delete(win.ctx, r.tab)
			continue
		}
		if win.ctx == nil {
			win.ctx = map[tabs.TabID]ctxEntry{}
		}
		win.ctx[r.tab] = ctxEntry{pct: *r.u.Ctx, cwdSeq: cwdSeq(t)}
	}
}

// usageSegments is the account limits plus the active tab's context while Claude still runs there.
func (win *window) usageSegments(active *tabs.Tab, now time.Time) []usage.Segment {
	u := win.acct
	if active != nil {
		if e, ok := win.ctx[active.ID]; ok && e.cwdSeq == cwdSeq(active) {
			u.Ctx = &e.pct
		}
	}
	return usage.Segments(u, now)
}

func (win *window) tabByID(id tabs.TabID) *tabs.Tab {
	for _, t := range win.mgr.Tabs() {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func cwdSeq(t *tabs.Tab) uint64 {
	s := t.Session
	s.Lock()
	defer s.Unlock()
	return s.Applier().CwdSeq()
}

func usageColor(l usage.Level) color.NRGBA {
	switch l {
	case usage.High:
		return colUsageHigh
	case usage.Warn:
		return colNeeds
	}
	return colFolder
}
