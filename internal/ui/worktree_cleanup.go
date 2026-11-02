package ui

import (
	"context"
	"image"
	"image/color"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/patrick-gogo/kaze-terminal/internal/branchsession"
)

type worktreeResult struct {
	closeSeq uint64
	due      time.Time
	failures []string
	statuses []branchsession.Status
	err      string
	removed  bool
	checked  map[string]time.Time
	fresh    []branchsession.Status
}
type worktreeUI struct {
	checking                   bool
	due                        time.Time
	closeSeq                   uint64
	errors                     []string
	shortcut                   string
	shortcutReady              bool
	removing                   bool
	initialized, busy, pending bool
	last                       map[string]time.Time
	results                    chan worktreeResult
	statuses                   []branchsession.Status
	closed                     []string
	kept                       map[string]bool
	confirm                    []branchsession.Record
	confirmScroll              int
	tag                        int
	keep, remove, list         image.Rectangle
}

func inFolder(folder, dir string) bool {
	if folder == "" || dir == "" {
		return false
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		folder, dir = strings.ToLower(folder), strings.ToLower(dir)
	}
	rel, e := filepath.Rel(folder, dir)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (win *window) worktreeOpen(folder string) bool {
	for _, t := range win.mgr.Tabs() {
		t.Session.Lock()
		cwd := t.Session.Applier().Cwd()
		t.Session.Unlock()
		if cwd == "" {
			cwd = t.Dir
		}
		if inFolder(folder, cwd) {
			return true
		}
	}
	return false
}
func (win *window) worktreeOpened(folder string) {
	w := &win.side.launch.work
	for f := range w.kept {
		if inFolder(f, folder) {
			delete(w.kept, f)
		}
	}
}
func (win *window) worktreeClosing(idDir string) {
	w := &win.side.launch.work
	if idDir != "" {
		w.closeSeq++
		if len(w.closed) >= 256 {
			copy(w.closed, w.closed[len(w.closed)-255:])
			w.closed = w.closed[:255]
		}
		w.closed = append(w.closed, idDir)
	}
}
func (win *window) requestWorktreeCheck(closed bool) {
	if win.cfgPath == "" {
		return
	} // test/benchmark windows never touch owner state
	l, w := &win.side.launch, &win.side.launch.work
	if closed && len(w.closed) > 0 {
		w.checking = true
		win.wantFocus = true
	}
	if w.busy || l.busy {
		w.pending = true
		return
	}
	w.initialized = true
	w.busy = true
	w.pending = false
	if w.results == nil {
		w.results = make(chan worktreeResult, 1)
	}
	now := time.Now()
	last := make(map[string]time.Time, len(w.last))
	for k, v := range w.last {
		last[k] = v
	}
	previous := append([]branchsession.Status(nil), w.statuses...)
	closedDirs := append([]string(nil), w.closed...)
	closeSeq := w.closeSeq
	store, results, window := win.branchStore(), w.results, win.w
	go func() {
		records, e := store.Records()
		r := worktreeResult{checked: map[string]time.Time{}, closeSeq: closeSeq}
		if e != nil {
			r.err = branchError(e)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), branchsession.Timeout)
			defer cancel()
			for _, rec := range records {
				if now.Sub(last[rec.Repo]) < time.Minute {
					for _, dir := range closedDirs {
						if inFolder(rec.Folder, dir) {
							due := last[rec.Repo].Add(time.Minute)
							if r.due.IsZero() || due.Before(r.due) {
								r.due = due
							}
							break
						}
					}
					for _, st := range previous {
						if st.Folder == rec.Folder {
							r.statuses = append(r.statuses, st)
						}
					}
					continue
				}
				r.checked[rec.Repo] = now
				st, e := branchsession.Check(ctx, rec)
				if e != nil {
					if r.err == "" {
						r.err = branchError(e)
					}
					continue
				}
				r.statuses = append(r.statuses, st)
				r.fresh = append(r.fresh, st)
			}
		}
		results <- r
		window.Invalidate()
	}()
}
func (win *window) applyWorktreeStatuses(statuses []branchsession.Status) {
	w := &win.side.launch.work
	w.statuses = statuses
	for _, st := range statuses {
		if win.worktreeOpen(st.Folder) {
			delete(w.kept, st.Folder)
			continue
		}
		if !st.Clean || w.kept[st.Folder] {
			continue
		}
		for _, dir := range w.closed {
			if inFolder(st.Folder, dir) {
				found := false
				for _, r := range w.confirm {
					if r.Folder == st.Folder {
						found = true
					}
				}
				if !found {
					w.confirm = append(w.confirm, st.Record)
					win.wantFocus = true
				}
				break
			}
		}
	}
	remaining := w.closed[:0]
	for _, dir := range w.closed {
		checked := false
		for _, st := range statuses {
			if inFolder(st.Folder, dir) {
				checked = true
				break
			}
		}
		if !checked {
			remaining = append(remaining, dir)
		}
	}
	w.closed = remaining
	win.side.cache.valid = false
}
func (win *window) drainWorktreeChecks() {
	w := &win.side.launch.work
	for {
		select {
		case r := <-w.results:
			w.busy = false
			w.removing = false
			if r.err != "" {
				win.note("Worktree cleanup: " + r.err)
			}
			if r.removed {
				w.errors = r.failures
				win.wantFocus = true
				w.confirm = nil
				w.confirmScroll = 0
				w.last = nil
				win.requestWorktreeCheck(false)
			} else {
				if r.closeSeq == w.closeSeq {
					win.applyWorktreeStatuses(r.fresh)
					w.due = r.due
					w.checking = !w.due.IsZero()
				} else if len(w.closed) > 0 {
					w.pending = true
				}
				w.statuses = r.statuses
				if w.last == nil {
					w.last = map[string]time.Time{}
				}
				for repo, at := range r.checked {
					w.last[repo] = at
				}
			}
			if w.pending {
				win.requestWorktreeCheck(false)
			}
		default:
			return
		}
	}
}
func (win *window) finishedWorktrees(repo string) []branchsession.Record {
	var out []branchsession.Record
	for _, st := range win.side.launch.work.statuses {
		if st.Repo == repo && st.Finished && !win.worktreeOpen(st.Folder) {
			out = append(out, st.Record)
		}
	}
	return out
}
func (win *window) worktreePress(p image.Point) bool {
	for _, hit := range win.side.hits.cleanups {
		if p.In(hit.rect) {
			win.side.launch.work.confirm = win.finishedWorktrees(hit.key)
			win.side.launch.work.confirmScroll = 0
			win.wantFocus = true
			return true
		}
	}
	return false
}
func (win *window) keepWorktrees() {
	w := &win.side.launch.work
	if w.kept == nil {
		w.kept = map[string]bool{}
	}
	for _, r := range w.confirm {
		w.kept[r.Folder] = true
	}
	w.confirm = nil
	w.checking = false
	w.due = time.Time{}
	w.closed = nil
	w.pending = false
	w.closeSeq++
	w.errors = nil
	w.confirmScroll = 0
	win.wantFocus = true
	win.w.Invalidate()
}
func (win *window) removeWorktrees() {
	w := &win.side.launch.work
	if w.busy || win.side.launch.busy {
		return
	}
	var folders []string
	for _, r := range w.confirm {
		if !win.worktreeOpen(r.Folder) {
			folders = append(folders, r.Folder)
		}
	}
	if len(folders) == 0 {
		win.keepWorktrees()
		return
	}
	w.busy = true
	w.removing = true
	if w.results == nil {
		w.results = make(chan worktreeResult, 1)
	}
	store, results, window := win.branchStore(), w.results, win.w
	go func() {
		var failures []string
		for _, f := range folders {
			if e := store.Remove(context.Background(), f); e != nil {
				reason := strings.ReplaceAll(e.Error(), f, filepath.Base(f))
				reason = strings.ReplaceAll(reason, filepath.ToSlash(f), filepath.Base(f))
				failures = append(failures, filepath.Base(f)+": "+cutRunes(cleanText(reason), 320))
			}
		}
		results <- worktreeResult{removed: true, failures: failures}
		window.Invalidate()
	}()
}
func (win *window) worktreeDialogEvents(gtx layout.Context) bool {
	w := &win.side.launch.work
	if !win.worktreeDialogShown() {
		return false
	}
	for {
		e, ok := gtx.Event(key.FocusFilter{Target: &w.tag}, pointer.Filter{Target: &w.tag, Kinds: pointer.Press | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}}, key.Filter{Focus: &w.tag, Name: key.NameEscape}, key.Filter{Focus: &w.tag, Name: key.NameReturn})
		if !ok {
			break
		}
		if win.bench.IgnoreInput() {
			continue
		}
		switch e := e.(type) {
		case pointer.Event:
			if e.Kind == pointer.Scroll {
				w.confirmScroll = max(0, w.confirmScroll+int(e.Scroll.Y))
				win.w.Invalidate()
			} else if e.Buttons.Contain(pointer.ButtonPrimary) {
				p := e.Position.Round()
				if p.In(w.keep) && !w.removing {
					win.keepWorktrees()
				}
				if p.In(w.remove) {
					win.removeWorktrees()
				}
			}
		case key.Event:
			if e.State == key.Press {
				if e.Name == key.NameEscape && !w.removing {
					win.keepWorktrees()
				}
				if e.Name == key.NameReturn {
					if len(w.errors) > 0 || w.checking {
						win.keepWorktrees()
					} else {
						win.removeWorktrees()
					}
				}
			}
		}
	}
	return true
}
func (win *window) drawWorktreeDialog(gtx layout.Context, r image.Rectangle) {
	w := &win.side.launch.work
	if !win.worktreeDialogShown() {
		return
	}
	c, ops := win.ui, gtx.Ops
	paint.FillShape(ops, tint(c.th.Chrome, 0xe8), clip.Rect(r).Op())
	width := min(gtx.Dp(580), max(0, r.Dx()-gtx.Dp(24)))
	height := min(gtx.Dp(320), max(0, r.Dy()-gtx.Dp(24)))
	box := image.Rect(r.Min.X+(r.Dx()-width)/2, r.Min.Y+(r.Dy()-height)/2, r.Min.X+(r.Dx()+width)/2, r.Min.Y+(r.Dy()+height)/2)
	paint.FillShape(ops, c.th.Chrome, clip.UniformRRect(box, gtx.Dp(10)).Op(ops))
	stroke(ops, box, gtx.Dp(10), c.th.Border)
	x, y := box.Min.X+gtx.Dp(16), box.Min.Y+gtx.Dp(14)
	title, description := "Remove worktree folders?", "The branches stay. Git refuses uncommitted or untracked changes."
	if len(w.errors) > 0 {
		title = "Some cleanup steps could not finish"
		description = "Branches were kept. Details from Git or worktree memory:"
	}
	if w.checking {
		title = "Checking worktrees…"
		description = "Waiting for a fresh check. Keep skips this cleanup."
	}
	for _, s := range []string{title, description} {
		call, sz := c.wrappedSide(gtx, s, 12, width-gtx.Dp(32), c.th.ActiveFG)
		place(ops, call, image.Pt(x, y))
		y += sz.Y + gtx.Dp(8)
	}
	w.list = image.Rect(x, y, box.Max.X-gtx.Dp(16), box.Max.Y-gtx.Dp(60))
	labels := w.errors
	if w.checking {
		labels = w.closed
	}
	if len(labels) == 0 {
		for _, rec := range w.confirm {
			labels = append(labels, tildeHome(rec.Folder, win.home))
		}
	}
	type wrapped struct {
		call op.CallOp
		size image.Point
	}
	var lines []wrapped
	total := 0
	for _, label := range labels {
		call, sz := c.wrappedSide(gtx, label, 11, w.list.Dx(), c.th.Folder)
		lines = append(lines, wrapped{call, sz})
		total += sz.Y + gtx.Dp(8)
	}
	w.confirmScroll = min(w.confirmScroll, max(0, total-w.list.Dy()))
	cl := clip.Rect(w.list).Push(ops)
	at := y - w.confirmScroll
	for _, line := range lines {
		place(ops, line.call, image.Pt(x, at))
		at += line.size.Y + gtx.Dp(8)
	}
	cl.Pop()
	w.keep = image.Rect(box.Max.X-gtx.Dp(200), box.Max.Y-gtx.Dp(44), box.Max.X-gtx.Dp(110), box.Max.Y-gtx.Dp(14))
	w.remove = image.Rect(w.keep.Max.X+gtx.Dp(8), w.keep.Min.Y, box.Max.X-gtx.Dp(16), w.keep.Max.Y)
	for _, b := range []struct {
		label string
		rect  image.Rectangle
	}{{"Keep", w.keep}, {"Remove", w.remove}} {
		label := b.label
		if len(w.errors) > 0 || w.checking {
			if label == "Remove" {
				w.remove = image.Rectangle{}
				continue
			}
			label = "Close"
			if w.checking {
				label = "Keep"
			}
		}
		if w.busy && label == "Remove" {
			label = "Removing…"
		}
		stroke(ops, b.rect, gtx.Dp(5), c.th.Border)
		call, sz := c.label(gtx, label, 12, c.th.ActiveFG)
		place(ops, call, image.Pt(b.rect.Min.X+(b.rect.Dx()-sz.X)/2, b.rect.Min.Y+(b.rect.Dy()-sz.Y)/2))
	}
	area := clip.Rect(r).Push(ops)
	event.Op(ops, &w.tag)
	area.Pop()
}

func (win *window) worktreeDialogShown() bool {
	w := &win.side.launch.work
	return len(w.confirm) > 0 || len(w.errors) > 0 || w.checking
}

// The confirmation must show complete folder names and refusal reasons, even
// when they need several lines. Session rows stay single-line and clipped.
func (c *chrome) wrappedSide(gtx layout.Context, s string, size unit.Sp, width int, col color.NRGBA) (op.CallOp, image.Point) {
	rec := op.Record(gtx.Ops)
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	fg := mat.Stop()
	gtx.Constraints = layout.Constraints{Max: image.Pt(max(width, 1), 1<<20)}
	d := widget.Label{}.Layout(gtx, c.shaper, font.Font{Typeface: chromeFace}, size, c.fit(s), fg)
	return rec.Stop(), d.Size
}

// A close event can defer one fresh check until the repo's minute limit. This
// finishes the user's pending cleanup, with no recurring idle polling.
func (win *window) worktreeTick(now time.Time) (time.Time, bool) {
	w := &win.side.launch.work
	if !w.checking || w.due.IsZero() {
		return time.Time{}, false
	}
	if now.Before(w.due) {
		return w.due, true
	}
	w.due = time.Time{}
	win.requestWorktreeCheck(false)
	return time.Time{}, false
}
