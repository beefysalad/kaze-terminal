package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

type consentDrawKey struct {
	consent *workerConsent
	rect    image.Rectangle
	metric  unit.Metric
	theme   theme.Chrome
}

// The banner shares workspace banner geometry, colors and hit kinds. Its
// stacked text and button row retain the complete decision at narrow widths.
// Cached drawing keeps an unchanged pending banner allocation free.
func (win *window) drawWorkerConsent(gtx layout.Context, t *tabs.Tab, termRect image.Rectangle) bool {
	s := &win.consent
	s.shown = 0
	if t == nil {
		return false
	}
	c := s.pending[t.ID]
	if c == nil {
		return false
	}
	s.shown = t.ID
	key := consentDrawKey{c, termRect, gtx.Metric, win.ui.th}
	if key != s.cache {
		s.drawOps.Reset()
		cached := gtx
		cached.Ops = &s.drawOps
		rec := op.Record(cached.Ops)
		win.buildConsentBanner(cached, c, termRect)
		s.call = rec.Stop()
		s.cache = key
	}
	s.call.Add(gtx.Ops)
	return true
}
func (win *window) buildConsentBanner(gtx layout.Context, c *workerConsent, termRect image.Rectangle) {
	s := &win.consent
	chrome, ops := win.ui, gtx.Ops
	s.hits = s.hits[:0]
	margin, pad, gap := gtx.Dp(8), gtx.Dp(12), gtx.Dp(8)
	line, radius := max(gtx.Dp(1), 1), gtx.Dp(8)
	r := image.Rect(termRect.Min.X+margin, termRect.Min.Y+margin, termRect.Max.X-margin, termRect.Min.Y+margin)
	if r.Dx() <= 2*pad {
		return
	}
	room := r.Dx() - 2*pad
	// Wrap the explanatory title at words so narrow windows keep the actor and
	// requested folder visible, while the first task line remains bounded.
	titleLines := consentTitleLines(c.title, room, gtx, chrome)
	type textLine struct {
		call op.CallOp
		size image.Point
	}
	var lines []textLine
	for _, str := range titleLines {
		call, size := chrome.fitLabel(gtx, str, room, chrome.th.ToastTitle)
		lines = append(lines, textLine{call, size})
	}
	task, ts := chrome.fitLabel(gtx, c.task, room, chrome.th.Muted)
	lines = append(lines, textLine{task, ts})
	question, qs := chrome.fitLabel(gtx, c.question, room, chrome.th.ToastTitle)
	lines = append(lines, textLine{question, qs})
	y := r.Min.Y + pad
	for _, l := range lines {
		y += l.size.Y + gtx.Dp(4)
	}
	allow, as := chrome.label(gtx, c.allow, 12, chrome.th.ToastGo)
	deny, ds := chrome.label(gtx, "Deny", 12, chrome.th.ToastGo)
	buttonH := max(as.Y, ds.Y) + gtx.Dp(12)
	y += gap
	r.Max.Y = y + buttonH + pad
	paint.FillShape(ops, chrome.th.ToastLine, clip.UniformRRect(r, radius).Op(ops))
	paint.FillShape(ops, chrome.th.ToastBG, clip.UniformRRect(r.Inset(line), max(radius-line, 0)).Op(ops))
	textClip := clip.Rect(r.Inset(pad)).Push(ops)
	ty := r.Min.Y + pad
	for _, l := range lines {
		place(ops, l.call, image.Pt(r.Min.X+pad, ty))
		ty += l.size.Y + gtx.Dp(4)
	}
	textClip.Pop()
	s.hits = append(s.hits, bannerHit{r, bannerBody})
	x := r.Min.X + pad
	for i, b := range []textLine{{allow, as}, {deny, ds}} {
		br := image.Rect(x, y, x+b.size.X+2*pad, y+buttonH)
		paint.FillShape(ops, chrome.th.ToastLine, clip.UniformRRect(br, radius/2).Op(ops))
		paint.FillShape(ops, chrome.th.ToastBG, clip.UniformRRect(br.Inset(line), max(radius/2-line, 0)).Op(ops))
		place(ops, b.call, image.Pt(br.Min.X+pad, br.Min.Y+(buttonH-b.size.Y)/2))
		kind := bannerRun
		if i == 1 {
			kind = bannerShell
		}
		s.hits = append(s.hits, bannerHit{br, kind})
		x = br.Max.X + gap
	}
	area := clip.Rect(r).Push(ops)
	event.Op(ops, &s.tag)
	area.Pop()
}

func consentTitleLines(text string, room int, gtx layout.Context, c *chrome) []string {
	// Titles are bounded on admission. At most six lines, even for long folder
	// names; fitLabel elides the remainder with the existing chrome convention.
	var lines []string
	for text != "" && len(lines) < 5 {
		_, sz := c.label(gtx, text, 12, c.th.ToastTitle)
		if sz.X <= room {
			break
		}
		cut := 0
		for i, ch := range text {
			if ch != ' ' {
				continue
			}
			_, sz := c.label(gtx, text[:i], 12, c.th.ToastTitle)
			if sz.X > room {
				break
			}
			cut = i
		}
		if cut == 0 {
			break
		}
		lines = append(lines, text[:cut])
		text = text[cut+1:]
	}
	if text != "" {
		lines = append(lines, text)
	}
	return lines
}
func (win *window) consentEvents(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &win.consent.tag, Kinds: pointer.Press})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok || !e.Buttons.Contain(pointer.ButtonPrimary) || win.bench.IgnoreInput() {
			continue
		}
		// Hidden overlays must not apply hits retained from a previous frame.
		t := win.mgr.Active()
		if t == nil || t.ID != win.consent.shown || win.set.shown || win.board.open || win.docs.shown != 0 {
			continue
		}
		for i := len(win.consent.hits) - 1; i >= 0; i-- {
			h := win.consent.hits[i]
			if !e.Position.Round().In(h.rect) {
				continue
			}
			if h.kind != bannerBody {
				win.answerWorkerConsent(t.ID, h.kind == bannerRun)
				win.wantFocus = true
				win.invalidate()
			}
			break
		}
	}
}
