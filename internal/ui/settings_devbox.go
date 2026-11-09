package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/devbox"
)

// The Devbox page (docs/superpowers/specs/2026-11-09-kaze-devbox-page-design.md):
// status of the person's own devbox jobs. It exists only on a paired machine,
// asks only when shown or on Refresh, and animates only while it can be seen.

// loadDevboxCreds finds this machine's pairing; tests replace it (see TestMain).
var loadDevboxCreds = devbox.Load

const (
	devboxTimeout  = 10 * time.Second
	devboxFrame    = 50 * time.Millisecond // about 20 frames a second
	devboxFinished = 5                     // finished jobs shown before "Show older"
	devboxDots     = 26
)

type devboxPhase int

const (
	devboxNone devboxPhase = iota
	devboxLoaded
	devboxUnreachable
	devboxRejected
)

// devboxPage is the page's state. creds is nil on a machine that isn't paired.
type devboxPage struct {
	load func() (devbox.Creds, bool)                               // nil means loadDevboxCreds
	list func(context.Context, devbox.Creds) ([]devbox.Job, error) // nil means devbox.List

	creds     *devbox.Creds
	fetching  bool
	phase     devboxPhase
	groups    devbox.Groups
	checkedAt time.Time
	more      bool
	wakes     int // animation frames asked for, for tests
}

func devboxSection() settingsSection {
	return settingsSection{
		title: "Devbox", hint: "Your laps and jobs, and whether they finished.",
		icon: "chip",
		draw: drawDevbox,
		show: func(win *window) { win.devboxCheck() },
		act: func(win *window, id string, _ int) {
			switch id {
			case "dbx_refresh":
				win.devboxCheck()
			case "dbx_more":
				win.set.devbox.more = true
			}
		},
	}
}

// devboxOpen looks for this machine's pairing as the Settings tab opens, before
// the menu is drawn, so the menu keeps its shape for as long as the tab is open.
func (win *window) devboxOpen() {
	d := &win.set.devbox
	load := d.load
	if load == nil {
		load = loadDevboxCreds
	}
	c, ok := load()
	if !ok {
		*d = devboxPage{load: d.load, list: d.list, fetching: d.fetching}
		return
	}
	if d.creds == nil || *d.creds != c {
		d.phase, d.groups, d.checkedAt, d.more = devboxNone, devbox.Groups{}, time.Time{}, false
	}
	d.creds = &c
}

// withDevbox adds the page to the menu on a paired machine, above the Kaze group.
func (win *window) withDevbox(secs []settingsSection) []settingsSection {
	if win.set.devbox.creds == nil {
		return secs
	}
	at := slices.IndexFunc(secs, func(s settingsSection) bool { return s.title == "Updates" })
	if at < 0 {
		return secs
	}
	return slices.Insert(secs, at, devboxSection())
}

// devboxCheck asks the devbox for the jobs in the background, once at a time.
func (win *window) devboxCheck() {
	d := &win.set.devbox
	if d.creds == nil || d.fetching {
		return
	}
	d.fetching = true
	creds, list := *d.creds, d.list
	if list == nil {
		list = func(ctx context.Context, c devbox.Creds) ([]devbox.Job, error) { return devbox.List(ctx, c, nil) }
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), devboxTimeout)
		defer cancel()
		jobs, err := list(ctx, creds)
		win.post(func(win *window) { win.devboxDone(creds, jobs, err) })
	}()
}

// devboxDone applies a check's result. Errors are never drawn as they came:
// the page says what happened in its own words.
func (win *window) devboxDone(asked devbox.Creds, jobs []devbox.Job, err error) {
	d := &win.set.devbox
	d.fetching = false
	if d.creds == nil || *d.creds != asked {
		return // the machine's pairing changed while it ran
	}
	switch {
	case err == nil:
		d.phase, d.groups, d.checkedAt = devboxLoaded, devbox.Group(jobs), time.Now()
	case errors.Is(err, devbox.ErrRejected):
		d.phase, d.groups = devboxRejected, devbox.Groups{}
	default:
		d.phase, d.groups = devboxUnreachable, devbox.Groups{}
	}
}

// devboxLook is how a status is shown: its pill and its spirit's color.
func (p *page) devboxLook(s devbox.Status) (text, tone string, col color.NRGBA) {
	switch s {
	case devbox.Running:
		return "Running", "run", p.col.accent
	case devbox.NeedsYou:
		return "Needs you", "warn", p.col.needs
	case devbox.Done:
		return "Done", "ok", p.col.done
	case devbox.Failed:
		return "Failed", "err", p.col.err
	case devbox.Stopped:
		return "Stopped", "off", p.col.muted
	case devbox.Queued:
		return "Queued", "off", p.col.muted
	}
	return "Unknown", "off", p.col.muted
}

// drawDevbox is the page: the header, then what the last check found.
func drawDevbox(p *page, r image.Rectangle, win *window) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	d := &win.set.devbox
	sec := devboxSection()
	y := p.heading(r, sec.title, sec.hint)

	label := "Refresh"
	if d.fetching {
		label = "Checking"
	}
	call, size, from := p.ctrl(ctrl{kind: ctlButton, id: "dbx_refresh", text: label, ghost: true, dead: d.fetching}, 0)
	at := image.Pt(r.Max.X-size.X, r.Min.Y+gtx.Dp(8))
	p.st.shift(from, len(p.st.controls), at)
	place(ops, call, at)
	if r.Dx() > gtx.Dp(620) && !d.checkedAt.IsZero() {
		tc, ts := p.text("Checked "+ago(time.Since(d.checkedAt)), 11.5, col.muted)
		place(ops, tc, image.Pt(at.X-gtx.Dp(12)-ts.X, at.Y+(size.Y-ts.Y)/2))
	}

	switch {
	case d.phase == devboxRejected:
		return p.devboxHero(r, y, devbox.Stopped, "This laptop isn't paired any more",
			"Run /devbox:setup with a new code from the devbox Setup tab.", nil)
	case d.phase == devboxUnreachable:
		return p.devboxHero(r, y, devbox.Stopped, "Couldn't reach the devbox",
			"No connection right now. Statuses are hidden because they may be out of date.",
			[]ctrl{{kind: ctlButton, id: "dbx_refresh", text: "Try again", dead: d.fetching}})
	case d.phase == devboxNone:
		return p.devboxPlaceholders(r, y)
	}

	g := d.groups
	y = p.devboxSummary(r, y, g, d.creds)
	lanes := false
	drawJobs := func(heading string, jobs []devbox.Job) {
		if len(jobs) == 0 {
			return
		}
		y = p.sectionLabel(r, y+gtx.Dp(22), heading)
		for i, j := range jobs {
			if i > 0 {
				y += gtx.Dp(10)
			}
			var seen bool
			y, seen = p.devboxJob(r, y, j)
			lanes = lanes || seen
		}
	}
	drawJobs("Needs you", g.NeedsYou)
	drawJobs("Running", g.Running)
	y = p.devboxLineRows(r, y, "Queued", g.Queued, true)
	if len(g.NeedsYou) == 0 && len(g.Running) == 0 && len(g.Queued) == 0 {
		sub := "All your jobs have finished."
		if len(g.Finished) == 0 {
			sub = "No jobs yet."
		}
		y = p.devboxHero(r, y+gtx.Dp(22), devbox.Done, "Nothing is running", sub, nil)
	}
	y = p.devboxLineRows(r, y, "Recently finished", g.Finished, d.more)

	if lanes && win.focused {
		d.wakes++
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(devboxFrame)})
	}
	return y
}

// devboxCard draws a card's background under what body drew and records it.
func (p *page) devboxCard(box image.Rectangle, radius unit.Dp, fill, edge color.NRGBA, body op.CallOp) {
	r := p.gtx.Dp(radius)
	paint.FillShape(p.gtx.Ops, fill, clip.UniformRRect(box, r).Op(p.gtx.Ops))
	body.Add(p.gtx.Ops)
	stroke(p.gtx.Ops, box, r, edge)
	p.st.cards = append(p.st.cards, box)
}

// fitName shortens s at its end so it fits in width.
func (p *page) fitName(s string, size unit.Sp, width int) string {
	fits := func(t string) bool {
		_, sz := p.c.labelW(p.gtx, t, size, p.col.title, font.Normal)
		return sz.X <= width
	}
	if fits(s) {
		return s
	}
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		n := (lo + hi + 1) / 2
		if fits(string(r[:n]) + "…") {
			lo = n
		} else {
			hi = n - 1
		}
	}
	return string(r[:lo]) + "…"
}

// devboxSummary is the strip under the header: who, and the counts.
func (p *page) devboxSummary(r image.Rectangle, y int, g devbox.Groups, creds *devbox.Creds) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	pad, mascot := gtx.Dp(18), gtx.Dp(46)
	type tally struct {
		n    int
		what string
		c    color.NRGBA
	}
	tallies := []tally{
		{len(g.Running), "running", col.accent},
		{len(g.NeedsYou), "need you", col.needs},
		{g.FailedToday, "failed", col.err},
	}
	gap := gtx.Dp(22)
	type laid struct {
		num, word op.CallOp
		nz, wz    image.Point
	}
	var cells []laid
	tw, th := 0, 0
	for i, t := range tallies {
		nc, nz := p.text(fmt.Sprint(t.n), 20, t.c)
		wc, wz := p.text(t.what, 11, col.muted)
		cells = append(cells, laid{nc, wc, nz, wz})
		if i > 0 {
			tw += gap
		}
		tw += max(nz.X, wz.X)
		th = max(th, nz.Y+wz.Y)
	}
	stacked := r.Dx() < gtx.Dp(460)
	h := max(mascot, th) + 2*pad
	if stacked {
		h = mascot + th + 3*pad
	}
	box := image.Rect(r.Min.X, y, r.Max.X, y+h)
	rec := op.Record(ops)

	wash := clip.UniformRRect(box, gtx.Dp(14)).Push(ops)
	paint.LinearGradientOp{
		Stop1: f32.Pt(float32(box.Min.X), 0), Color1: tint(col.accent, 0x20),
		Stop2: f32.Pt(float32(box.Min.X)+float32(box.Dx())*0.6, 0), Color2: tint(col.accent, 0),
	}.Add(ops)
	paint.PaintOp{}.Add(ops)
	wash.Pop()

	status := devbox.Done
	switch {
	case len(g.NeedsYou) > 0 && len(g.Running) == 0:
		status = devbox.NeedsYou
	case len(g.Running) > 0:
		status = devbox.Running
	}
	_, _, c := p.devboxLook(status)
	top := y + pad
	mid := top + mascot/2
	if !stacked {
		mid = y + h/2
	}
	drawSpirit(gtx, image.Pt(r.Min.X+pad+mascot/2, mid), mascot, status, c, spiritMotion{})

	name := "Your devbox"
	if creds != nil && creds.Name != "" {
		name = creds.Name
	}
	tx := r.Min.X + pad + mascot + gtx.Dp(16)
	room := r.Max.X - pad - tx
	if !stacked {
		room -= tw + gap
	}
	nc, nz := p.text(p.fitName(name, 15, max(room, 1)), 15, col.title)
	sc, sz := p.text("Your devbox space", 12, col.muted)
	ny := mid - (nz.Y+sz.Y)/2
	place(ops, nc, image.Pt(tx, ny))
	place(ops, sc, image.Pt(tx, ny+nz.Y))

	x, ty := r.Max.X-pad-tw, y+(h-th)/2
	if stacked {
		x, ty = r.Min.X+pad, y+pad+mascot+pad
	}
	for _, cell := range cells {
		w := max(cell.nz.X, cell.wz.X)
		place(ops, cell.num, image.Pt(x+w-cell.nz.X, ty))
		place(ops, cell.word, image.Pt(x+w-cell.wz.X, ty+cell.nz.Y))
		x += w + gap
	}
	p.devboxCard(box, 14, col.card, col.line2, rec.Stop())
	return box.Max.Y
}

// devboxJob draws a job with its lane; seen says whether the lane is on screen.
func (p *page) devboxJob(r image.Rectangle, y int, j devbox.Job) (int, bool) {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	pad := gtx.Dp(14)
	inner := r.Inset(0)
	inner.Min.X, inner.Max.X = r.Min.X+pad, r.Max.X-pad
	rec := op.Record(ops)
	yy := p.devboxHeadLine(inner, y+gtx.Dp(12), j, nil)
	if j.Question != "" {
		yy += gtx.Dp(4)
		for _, l := range p.wrap(j.Question, 11.5, max(inner.Dx(), 1)) {
			call, size := p.text(l, 11.5, col.needs)
			place(ops, call, image.Pt(inner.Min.X, yy))
			yy += size.Y
		}
	}
	lane := image.Rect(inner.Min.X, yy+gtx.Dp(10), inner.Max.X, yy+gtx.Dp(60))
	p.devboxLane(lane, j)
	box := image.Rect(r.Min.X, y, r.Max.X, lane.Max.Y+gtx.Dp(14))
	body := rec.Stop()
	fill, edge := col.card, col.line
	if j.Status == devbox.NeedsYou {
		edge = tint(col.needs, 0x60)
		fill = mix(col.card, col.needs, 0.05)
	}
	p.devboxCard(box, 12, fill, edge, body)
	screen := lane.Sub(image.Pt(0, p.st.scroll))
	return box.Max.Y, screen.Overlaps(p.st.pane)
}

// devboxHeadLine draws name, age and pill on one line and returns where it ends.
// spiritAt, when set, is the small spirit at the line's start.
func (p *page) devboxHeadLine(r image.Rectangle, y int, j devbox.Job, spirit func(mid int) int) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	text, tone, _ := p.devboxLook(j.Status)
	pc, ps := p.pill(text, tone)
	gap := gtx.Dp(12)
	x1 := r.Max.X - ps.X
	var ac op.CallOp
	var as image.Point
	if j.AgeText != "" && r.Dx() > gtx.Dp(300) {
		ac, as = p.text(j.AgeText, 11.5, col.muted)
		x1 -= gap + as.X
	}
	x0 := r.Min.X
	mid := y + ps.Y/2
	if spirit != nil {
		x0 = spirit(mid)
	}
	nc, ns := p.text(p.fitName(j.Name, 12.5, max(x1-gap-x0, 1)), 12.5, col.title)
	place(ops, nc, image.Pt(x0, mid-ns.Y/2))
	if as.X > 0 {
		place(ops, ac, image.Pt(x1, mid-as.Y/2))
	}
	place(ops, pc, image.Pt(r.Max.X-ps.X, y))
	return y + ps.Y
}

// devboxLane is the track: dots ahead, a trail behind, the spirit at the job's progress.
func (p *page) devboxLane(box image.Rectangle, j devbox.Job) {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	rad := gtx.Dp(9)
	_, _, c := p.devboxLook(j.Status)
	paint.FillShape(ops, col.nav, clip.UniformRRect(box, rad).Op(ops))
	stroke(ops, box, rad, col.line)

	inset := gtx.Dp(30)
	x0, w := box.Min.X+inset, max(box.Dx()-2*inset, 1)
	mid := box.Min.Y + box.Dy()/2
	pct := j.Progress
	if pct < 0 {
		pct = 0
		if j.Status == devbox.NeedsYou {
			pct = 100
		}
	}
	at := x0 + w*pct/100
	n := min(devboxDots, max(w/gtx.Dp(12), 2))
	d := gtx.Dp(5)
	for i := range n {
		x := x0 + w*i/(n-1)
		dot := image.Rect(x-d/2, mid-d/2, x-d/2+d, mid-d/2+d)
		dc := col.muted
		if x <= at {
			dc = tint(col.muted, 0x28)
		}
		paint.FillShape(ops, dc, clip.Ellipse(dot).Op(ops))
	}
	trail := image.Rect(box.Min.X, mid-gtx.Dp(1), at, mid+gtx.Dp(1))
	cl := clip.Rect(trail).Push(ops)
	paint.LinearGradientOp{
		Stop1: f32.Pt(float32(trail.Min.X), 0), Color1: tint(c, 0),
		Stop2: f32.Pt(float32(trail.Max.X), 0), Color2: c,
	}.Add(ops)
	paint.PaintOp{}.Add(ops)
	cl.Pop()

	var m spiritMotion
	now := gtx.Now
	switch j.Status {
	case devbox.Running:
		s := wave(now, 1500*time.Millisecond)
		m.bob, m.gust = float32(s)*float32(gtx.Dp(2)), float32(0.55+0.45*s)
	case devbox.NeedsYou:
		m.breath = float32(0.05 * wave(now, 3200*time.Millisecond))
	}
	drawSpirit(gtx, image.Pt(at, mid), gtx.Dp(36), j.Status, c, m)
}

// wave is a sine of the time with the given period.
func wave(now time.Time, period time.Duration) float64 {
	return math.Sin(2 * math.Pi * float64(now.UnixMilli()%period.Milliseconds()) / float64(period.Milliseconds()))
}

// devboxLineRows lists jobs one line each; unless more is set, the older ones wait behind a button.
func (p *page) devboxLineRows(r image.Rectangle, y int, heading string, jobs []devbox.Job, more bool) int {
	if len(jobs) == 0 {
		return y
	}
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	y = p.sectionLabel(r, y+gtx.Dp(22), heading)
	shown := jobs
	if !more && len(jobs) > devboxFinished {
		shown = jobs[:devboxFinished]
	}
	pad, rowH := gtx.Dp(14), gtx.Dp(46)
	inner := image.Rect(r.Min.X+pad, y, r.Max.X-pad, y)
	rec := op.Record(ops)
	yy := y
	for i, j := range shown {
		if i > 0 {
			paint.FillShape(ops, col.line, clip.Rect(image.Rect(r.Min.X, yy, r.Max.X, yy+1)).Op())
		}
		pillH := gtx.Dp(22)
		_, _, c := p.devboxLook(j.Status)
		p.devboxHeadLine(inner, yy+(rowH-pillH)/2, j, func(mid int) int {
			s := gtx.Dp(24)
			drawSpirit(gtx, image.Pt(inner.Min.X+s/2, mid), s, j.Status, c, spiritMotion{})
			return inner.Min.X + s + gtx.Dp(12)
		})
		yy += rowH
	}
	box := image.Rect(r.Min.X, y, r.Max.X, yy)
	p.devboxCard(box, 12, col.card, col.line, rec.Stop())
	yy += gtx.Dp(10)
	if left := len(jobs) - len(shown); left > 0 {
		word := "jobs"
		if left == 1 {
			word = "job"
		}
		call, size, from := p.ctrl(ctrl{kind: ctlButton, id: "dbx_more", text: fmt.Sprintf("Show %d older %s", left, word), ghost: true}, 0)
		at := image.Pt(r.Min.X, yy)
		p.st.shift(from, len(p.st.controls), at)
		place(ops, call, at)
		yy += size.Y
	}
	return yy
}

// devboxHero is a card with a spirit and a plain message.
func (p *page) devboxHero(r image.Rectangle, y int, status devbox.Status, title, sub string, buttons []ctrl) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	pad, s := gtx.Dp(18), gtx.Dp(40)
	_, _, c := p.devboxLook(status)
	rec := op.Record(ops)
	x := r.Min.X + pad + s + gtx.Dp(14)
	right := r.Max.X - pad
	type btn struct {
		call     op.CallOp
		size     image.Point
		from, to int
	}
	var bs []btn
	bw := 0
	for _, k := range buttons {
		call, size, from := p.ctrl(k, 0)
		bs = append(bs, btn{call, size, from, len(p.st.controls)})
		bw += size.X
	}
	narrow := r.Dx() < gtx.Dp(460)
	room := right - x
	if !narrow && bw > 0 {
		room -= bw + gtx.Dp(14)
	}
	tc, ts := p.text(title, 13.5, col.title)
	tw := ts.Y
	lines := p.wrap(sub, 12, max(room, 1))
	h := tw
	type line struct {
		call op.CallOp
		y    int
	}
	var ls []line
	for _, l := range lines {
		call, size := p.text(l, 12, col.muted)
		ls = append(ls, line{call, h})
		h += size.Y
	}
	content := max(h, s)
	textTop := y + pad + (content-h)/2
	place(ops, tc, image.Pt(x, textTop))
	for _, l := range ls {
		place(ops, l.call, image.Pt(x, textTop+l.y))
	}
	drawSpirit(gtx, image.Pt(r.Min.X+pad+s/2, y+pad+content/2), s, status, c, spiritMotion{})
	bottom := y + pad + content
	if bw > 0 {
		bx, by := right-bw, y+pad+(content-bs[0].size.Y)/2
		if narrow {
			bx, by = x, bottom+gtx.Dp(10)
			bottom = by + bs[0].size.Y
		}
		for _, b := range bs {
			at := image.Pt(bx, by)
			p.st.shift(b.from, b.to, at)
			place(ops, b.call, at)
			bx += b.size.X
		}
	}
	box := image.Rect(r.Min.X, y, r.Max.X, bottom+pad)
	p.devboxCard(box, 12, col.card, col.line, rec.Stop())
	return box.Max.Y
}

// devboxPlaceholders stand in for the jobs during the first check.
func (p *page) devboxPlaceholders(r image.Rectangle, y int) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	y = p.sectionLabel(r, y, "Running")
	for i := range 3 {
		if i > 0 {
			y += gtx.Dp(10)
		}
		box := image.Rect(r.Min.X, y, r.Max.X, y+gtx.Dp(96))
		rec := op.Record(ops)
		pad := gtx.Dp(14)
		bar := image.Rect(box.Min.X+pad, y+gtx.Dp(18), box.Min.X+pad+box.Dx()*2/5, y+gtx.Dp(28))
		paint.FillShape(ops, col.raised, clip.UniformRRect(bar, bar.Dy()/2).Op(ops))
		lane := image.Rect(box.Min.X+pad, y+gtx.Dp(38), box.Max.X-pad, y+gtx.Dp(84))
		paint.FillShape(ops, col.nav, clip.UniformRRect(lane, gtx.Dp(9)).Op(ops))
		p.devboxCard(box, 12, col.card, col.line, rec.Stop())
		y = box.Max.Y
	}
	return y
}

// spiritMotion is one frame of a spirit's animation; zero is still.
type spiritMotion struct {
	bob    float32 // pixels up or down
	gust   float32 // 0..1, the wind lines behind it; 0 draws none
	breath float32 // added to its scale
}

// spiritInk is the face, dark on every status color in every theme.
var spiritInk = color.NRGBA{R: 0x0c, G: 0x0e, B: 0x14, A: 0xff}

// drawSpirit draws Kaze's wind spirit, a curl of wind with a face, centered at
// c and size pixels across, from a 32-unit drawing.
func drawSpirit(gtx layout.Context, c image.Point, size int, status devbox.Status, col color.NRGBA, m spiritMotion) {
	ops := gtx.Ops
	k := float32(size) / 32 * (1 + m.breath)
	ox, oy := float32(c.X)-16*k, float32(c.Y)-16*k+m.bob
	pt := func(x, y float32) f32.Point { return f32.Pt(ox+x*k, oy+y*k) }

	strokeLines := func(ink color.NRGBA, width float32, lines ...[]float32) {
		var p clip.Path
		p.Begin(ops)
		for _, l := range lines {
			p.MoveTo(pt(l[0], l[1]))
			for i := 2; i+1 < len(l); i += 2 {
				p.LineTo(pt(l[i], l[i+1]))
			}
		}
		paint.FillShape(ops, ink, clip.Stroke{Path: p.End(), Width: max(width*k, 1)}.Op())
	}
	if m.gust > 0 {
		wind := tint(col, uint8(m.gust*0xff))
		strokeLines(wind, 1.6, []float32{-7, 11, -1, 11}, []float32{-10, 16, -3, 16}, []float32{-7, 21, -2, 21})
	}

	var body clip.Path
	body.Begin(ops)
	body.MoveTo(pt(21, 4.5))
	for _, s := range [][6]float32{
		{27, 4.5, 31, 9.4, 31, 15.5},
		{31, 22.5, 25.8, 27, 19, 27},
		{13, 27, 8.5, 24.4, 3.4, 26.4},
		{1.6, 27, .6, 25.4, 1.8, 24.1},
		{4.2, 21.7, 5.5, 19.7, 5.6, 17.4},
		{3.8, 17.3, 2, 16.3, .9, 14.6},
		{.2, 13.4, 1.3, 12.2, 2.6, 12.8},
		{4.2, 13.5, 5.7, 13.4, 6.7, 12.7},
		{8, 7.8, 13.9, 4.5, 21, 4.5},
	} {
		body.CubeTo(pt(s[0], s[1]), pt(s[2], s[3]), pt(s[4], s[5]))
	}
	body.Close()
	paint.FillShape(ops, col, clip.Outline{Path: body.End()}.Op())

	ellipse := func(cx, cy, rx, ry float32, fill color.NRGBA) {
		a, b := pt(cx-rx, cy-ry), pt(cx+rx, cy+ry)
		paint.FillShape(ops, fill, clip.Ellipse{Min: image.Pt(int(a.X), int(a.Y)), Max: image.Pt(int(math.Ceil(float64(b.X))), int(math.Ceil(float64(b.Y))))}.Op(ops))
	}
	switch status {
	case devbox.Stopped, devbox.Unknown:
		strokeLines(spiritInk, 1.9, []float32{16.5, 15, 20.1, 15}, []float32{22.8, 15, 26.4, 15})
	case devbox.Failed:
		strokeLines(spiritInk, 1.9,
			[]float32{16.8, 13, 19.8, 16.4}, []float32{19.8, 13, 16.8, 16.4},
			[]float32{23, 13, 26, 16.4}, []float32{26, 13, 23, 16.4})
	case devbox.Done:
		strokeLines(spiritInk, 1.9,
			[]float32{16.5, 15.6, 17.4, 14.2, 18.3, 13.8, 19.2, 14.2, 20.1, 15.6},
			[]float32{22.8, 15.6, 23.7, 14.2, 24.6, 13.8, 25.5, 14.2, 26.4, 15.6})
	default:
		ellipse(18.4, 14.6, 1.7, 2.3, spiritInk)
		ellipse(24.6, 14.6, 1.7, 2.3, spiritInk)
	}
	if status == devbox.NeedsYou {
		ellipse(28, 4.5, 6, 6, spiritInk)
		ellipse(28, 4.5, 4.6, 4.6, col)
		strokeLines(spiritInk, 1.5, []float32{28, 2, 28, 5}, []float32{28, 7, 28, 7.3})
	}
}
