package ui

import (
	"image"
	"image/color"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/tabs"
)

// mark is a tab's attention dot.
type mark int

const (
	markNone mark = iota
	markDone
	markNeeds
)

const (
	doneToastFor = 6 * time.Second
	maxToasts    = 3
	maxReason    = 120
	// alertQueue bounds alerts waiting for the UI goroutine; more are dropped.
	alertQueue = 16
)

// toast is one alert card. Text is the cleaned reason; the tab title is read
// when it is drawn so a renamed tab shows its current name.
type toast struct {
	Tab   tabs.TabID
	Kind  notify.Kind
	Text  string
	Until time.Time // zero = stays until visited or dismissed
}

// alerts holds the tab marks and toasts. Used only on the UI goroutine.
type alerts struct {
	marks  map[tabs.TabID]mark
	toasts []toast // oldest first, at most maxToasts
}

// apply records one alert. It does nothing for a tab that is gone or for the
// tab the owner is looking at right now.
func (a *alerts) apply(m notify.Msg, active tabs.TabID, focused bool, exists func(tabs.TabID) bool, now time.Time) (chime notify.Kind, redraw bool) {
	id := tabs.TabID(m.Tab)
	if id <= 0 || !exists(id) || (id == active && focused) {
		return "", false
	}
	if m.Kind != notify.KindNeeds && m.Kind != notify.KindDone {
		return "", false
	}
	if a.marks == nil {
		a.marks = map[tabs.TabID]mark{}
	}
	// "needs" stays until the tab is seen, even if Claude finishes meanwhile.
	if m.Kind == notify.KindNeeds {
		a.marks[id] = markNeeds
	} else if a.marks[id] != markNeeds {
		a.marks[id] = markDone
	}
	t := toast{Tab: id, Kind: m.Kind, Text: cleanReason(m.Reason)}
	if m.Kind == notify.KindDone {
		t.Until = now.Add(doneToastFor)
	}
	a.dropToast(id) // one card per tab: the latest news replaces the old
	a.toasts = append(a.toasts, t)
	if n := len(a.toasts); n > maxToasts {
		a.toasts = slices.Delete(a.toasts, 0, n-maxToasts)
	}
	return m.Kind, true
}

func (a *alerts) dropToast(id tabs.TabID) bool {
	n := len(a.toasts)
	a.toasts = slices.DeleteFunc(a.toasts, func(t toast) bool { return t.Tab == id })
	return len(a.toasts) != n
}

// visited clears the tab's mark and its toast.
func (a *alerts) visited(id tabs.TabID) bool {
	_, had := a.marks[id]
	delete(a.marks, id)
	return a.dropToast(id) || had
}

// dismiss closes the tab's toast but keeps its mark: it was not seen yet.
func (a *alerts) dismiss(id tabs.TabID) { a.dropToast(id) }

// expire drops timed-out toasts and returns when the next one is due.
func (a *alerts) expire(now time.Time) (next time.Time, changed bool) {
	n := len(a.toasts)
	a.toasts = slices.DeleteFunc(a.toasts, func(t toast) bool { return !t.Until.IsZero() && !now.Before(t.Until) })
	for _, t := range a.toasts {
		if !t.Until.IsZero() && (next.IsZero() || t.Until.Before(next)) {
			next = t.Until
		}
	}
	return next, len(a.toasts) != n
}

// prune forgets tabs that were closed.
func (a *alerts) prune(exists func(tabs.TabID) bool) {
	for id := range a.marks {
		if !exists(id) {
			delete(a.marks, id)
		}
	}
	a.toasts = slices.DeleteFunc(a.toasts, func(t toast) bool { return !exists(t.Tab) })
}

func (a *alerts) markOf(id tabs.TabID) mark { return a.marks[id] }

// list is the toasts newest first.
func (a *alerts) list() []toast {
	out := slices.Clone(a.toasts)
	slices.Reverse(out)
	return out
}

// cleanReason makes the sender's text safe to draw on one line: no control,
// format (bidi) or line-separator runes, at most maxReason runes.
func cleanReason(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == maxReason {
			break
		}
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// Alert colors (design spec 6.7).
var (
	colNeeds      = rgb(0xf2b84b)
	colDone       = rgb(0x62d39a)
	colToastBG    = rgb(0x1a1f2c)
	colToastLine  = rgb(0x2a3142)
	colToastFG    = rgb(0xe7e9f0)
	colToastGo    = rgb(0xb9a6ff)
	colToastTitle = rgb(0xffffff)
)

// dotColor is the dot a tab shows: an alert mark wins over the active accent.
func dotColor(active bool, m mark) (color.NRGBA, bool) {
	switch {
	case m == markNeeds:
		return colNeeds, true
	case m == markDone:
		return colDone, true
	case active:
		return colAccent, true
	}
	return color.NRGBA{}, false
}

// toastHit is where a toast and its buttons were drawn, in window pixels.
type toastHit struct {
	id                      tabs.TabID
	rect, goRect, closeRect image.Rectangle
}

// onNotify runs on a notify connection goroutine: hand the message to the UI
// goroutine without ever blocking (a full queue drops it).
func (win *window) onNotify(m notify.Msg) {
	select {
	case win.alertCh <- m:
	default:
	}
	if win.w != nil {
		win.w.Invalidate()
	}
}

// drainAlerts applies every queued alert; UI goroutine only.
func (win *window) drainAlerts(now time.Time) {
	for {
		select {
		case m := <-win.alertCh:
			win.deliver(m, now)
		default:
			return
		}
	}
}

// deliver applies one alert and plays the chime when the config asks for it.
func (win *window) deliver(m notify.Msg, now time.Time) {
	var active tabs.TabID
	if t := win.mgr.Active(); t != nil {
		active = t.ID
	}
	list := win.mgr.Tabs()
	exists := func(id tabs.TabID) bool {
		return slices.ContainsFunc(list, func(t *tabs.Tab) bool { return t.ID == id })
	}
	kind, _ := win.alerts.apply(m, active, win.focused, exists, now)
	cfg := win.opt.Config
	if win.chime != nil && (kind == notify.KindNeeds && cfg.ChimeNeeds || kind == notify.KindDone && cfg.ChimeDone) {
		win.chime(now) // the player coalesces bursts into one chime
	}
}

// selectTab is the one tab switch for a tab click and a toast's "Go ›".
func (win *window) selectTab(id tabs.TabID) {
	win.mgr.SetActive(id)
	win.wantFocus = true
}

// visit clears the shown tab's alert, but only when the owner can see it.
func (win *window) visit(t *tabs.Tab) {
	if t != nil && win.focused {
		win.alerts.visited(t.ID)
	}
}

// toastEvent handles a press on a toast: "Go ›" switches, × dismisses.
func (win *window) toastEvent(e pointer.Event) {
	if e.Kind != pointer.Press || !e.Buttons.Contain(pointer.ButtonPrimary) || win.bench.IgnoreInput() {
		return
	}
	p := e.Position.Round()
	for _, h := range win.ui.toastHits {
		switch {
		case p.In(h.closeRect):
			win.alerts.dismiss(h.id)
			win.wantFocus = true
			win.w.Invalidate()
			return
		case p.In(h.goRect):
			win.selectTab(h.id)
			win.w.Invalidate()
			return
		case p.In(h.rect):
			return
		}
	}
}

// drawToasts draws the toasts bottom-right above the status bar, newest on
// top, and records where they are for clicks. height is the window height.
func (win *window) drawToasts(gtx layout.Context, height, statusH int) {
	c := win.ui
	c.toastHits = c.toastHits[:0]
	list := win.alerts.list()
	if len(list) == 0 {
		return
	}
	titles := map[tabs.TabID]string{}
	for _, t := range win.mgr.Tabs() {
		titles[t.ID] = c.tabLabelFit(t, win.defTitle)
	}
	ops := gtx.Ops
	width := gtx.Constraints.Max.X
	margin, gap := gtx.Dp(14), gtx.Dp(8)
	padX, padY, inner := gtx.Dp(12), gtx.Dp(10), gtx.Dp(10)
	dot, radius, line := gtx.Dp(7), gtx.Dp(10), max(gtx.Dp(1), 1)
	maxW := min(gtx.Dp(460), width-2*margin)

	type card struct {
		id              tabs.TabID
		kind            notify.Kind
		title, rest     op.CallOp
		titleSz, restSz image.Point
		w, h            int
	}
	goCall, goSz := c.label(gtx, "Go ›", 12, colToastGo)
	xCall, xSz := c.label(gtx, "×", 12, colMuted)
	cards := make([]card, 0, len(list))
	total := 0
	for _, t := range list {
		k := card{id: t.Tab, kind: t.Kind}
		k.title, k.titleSz = c.label(gtx, titles[t.Tab], 12, colToastTitle)
		rest := " is done"
		if t.Kind == notify.KindNeeds {
			rest = " needs you"
			if t.Text != "" {
				rest += " · " + t.Text
			}
		}
		fixed := padX + dot + inner + k.titleSz.X + inner + goSz.X + inner + xSz.X + padX
		k.rest, k.restSz = c.label(gtx, rest, 12, colToastFG)
		// Shorten the reason until the card fits; bounded so a bad measure cannot spin.
		for i := 0; i < 8 && fixed+k.restSz.X > maxW && rest != "…"; i++ {
			n := utf8.RuneCountInString(rest)
			keep := n * max(maxW-fixed, 0) / max(k.restSz.X, 1)
			if keep >= n {
				keep = n - 1
			}
			rest = truncateRight(rest, keep)
			k.rest, k.restSz = c.label(gtx, rest, 12, colToastFG)
		}
		k.w = min(fixed+k.restSz.X, maxW)
		k.h = max(k.titleSz.Y, k.restSz.Y, goSz.Y) + 2*padY
		total += k.h + gap
		cards = append(cards, k)
	}
	y := height - statusH - gtx.Dp(16) - (total - gap)
	for _, k := range cards {
		r := image.Rect(width-margin-k.w, y, width-margin, y+k.h)
		paint.FillShape(ops, colToastLine, clip.UniformRRect(r, radius).Op(ops))
		paint.FillShape(ops, colToastBG, clip.UniformRRect(r.Inset(line), max(radius-line, 0)).Op(ops))
		cl := clip.Rect(r).Push(ops)
		mid := r.Min.Y + k.h/2
		x := r.Min.X + padX
		dc := colDone
		if k.kind == notify.KindNeeds {
			dc = colNeeds
		}
		paint.FillShape(ops, dc, clip.Ellipse(image.Rect(x, mid-dot/2, x+dot, mid-dot/2+dot)).Op(ops))
		x += dot + inner
		place(ops, k.title, image.Pt(x, mid-k.titleSz.Y/2))
		x += k.titleSz.X
		place(ops, k.rest, image.Pt(x, mid-k.restSz.Y/2))
		// Buttons are pinned to the right edge so they stay clickable.
		xr := r.Max.X - padX - xSz.X
		gx := xr - inner - goSz.X
		place(ops, goCall, image.Pt(gx, mid-goSz.Y/2))
		place(ops, xCall, image.Pt(xr, mid-xSz.Y/2))
		cl.Pop()
		hp := gtx.Dp(4)
		c.toastHits = append(c.toastHits, toastHit{
			id:        k.id,
			rect:      r,
			goRect:    image.Rect(gx-hp, r.Min.Y+line, gx+goSz.X+hp/2, r.Max.Y-line),
			closeRect: image.Rect(xr-hp/2, r.Min.Y+line, min(xr+xSz.X+hp, r.Max.X-line), r.Max.Y-line),
		})
		y += k.h + gap
	}
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
