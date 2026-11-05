package ui

import (
	"context"
	"image"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/beefysalad/kaze-terminal/internal/branchsession"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

type branchRepo struct{ key, name, dir string }
type branchReply struct {
	epoch  uint64
	branch string
	info   branchsession.Info
	result branchsession.Result
	err    error
	start  bool
}
type branchLauncher struct {
	shown, open, repoOpen, busy, previewBusy bool
	version, epoch                           uint64
	repoScroll                               int
	repoCall                                 op.CallOp
	ed                                       widget.Editor
	filters                                  []event.Filter
	repos                                    []branchRepo
	repo                                     int
	agent, hint, err, previewBranch          string
	replies                                  chan branchReply
	store                                    *branchsession.Store
	work                                     worktreeUI
}

func (l *branchLauncher) changed() { l.version++ }
func (win *window) branchStore() *branchsession.Store {
	l := &win.side.launch
	if l.store == nil {
		path := ""
		if win.cfgPath != "" {
			path = filepath.Join(filepath.Dir(win.cfgPath), "worktrees.json")
		}
		l.store = &branchsession.Store{Path: path}
	}
	return l.store
}
func (win *window) openBranchLauncher() {
	if win.side.launch.busy {
		return
	}
	if !win.opt.Config.Sidebar {
		win.opt.Config.Sidebar = true
		win.saveSetting("sidebar", true)
	}
	if !win.opt.Config.SidebarLauncher {
		win.opt.Config.SidebarLauncher = true
		win.saveSetting("sidebar_launcher", true)
	}
	win.dropSideEdit()
	l := &win.side.launch
	l.repos = l.repos[:0]
	win.cachedSide(time.Now())
	selected := ""
	if t := win.mgr.Active(); t != nil {
		selected = win.locOf(t).repo
	}
	l.repo = 0
	for _, g := range win.side.cache.base.Groups {
		dir := ""
		for _, r := range g.Rows {
			if t := win.mgr.Lookup(r.ID); t != nil {
				dir = win.locOf(t).cwd
				break
			}
		}
		if g.Key == selected {
			l.repo = len(l.repos)
		}
		l.repos = append(l.repos, branchRepo{g.Key, g.Name, dir})
	}
	if l.agent == "" {
		l.agent = win.opt.Config.SidebarLauncherAgent
		if l.agent == "" {
			l.agent = "claude"
		}
	}
	win.initBranchShortcut()
	l.open = true
	l.repoOpen = false
	l.repoScroll = 0
	l.err = ""
	l.epoch++
	l.previewBranch = ""
	l.hint = "Enter a branch name"
	if len(l.repos) == 0 {
		l.hint = "Open a session in a Git repo first."
	}
	l.ed.SingleLine, l.ed.Submit, l.ed.MaxLen = true, true, 200
	l.changed()
	win.wantFocus = true
	win.sideRedraw()
}
func (win *window) closeBranchLauncher() {
	l := &win.side.launch
	if l.busy {
		return
	}
	l.open = false
	l.repoOpen = false
	l.epoch++
	l.changed()
	win.wantFocus = true
	win.sideRedraw()
}
func (win *window) branchEvents(gtx layout.Context) {
	l := &win.side.launch
	if !l.open || win.side.hidden {
		return
	}
	if l.filters == nil {
		l.filters = win.edKeys(&l.ed)
	}
	if l.repoOpen {
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &l.ed, Name: key.NameUpArrow}, key.Filter{Focus: &l.ed, Name: key.NameDownArrow}, key.Filter{Focus: &l.ed, Name: key.NameReturn})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				switch e.Name {
				case key.NameUpArrow:
					l.repo = (l.repo + len(l.repos) - 1) % len(l.repos)
					l.repoScroll = max(0, l.repo*gtx.Dp(28)-gtx.Dp(56))
				case key.NameDownArrow:
					l.repo = (l.repo + 1) % len(l.repos)
					l.repoScroll = max(0, l.repo*gtx.Dp(28)-gtx.Dp(56))
				case key.NameReturn:
					l.repoOpen = false
					l.epoch++
					win.previewBranch()
				}
				l.changed()
				win.sideRedraw()
			}
		}
	}
	if win.edShortcuts(gtx, &l.ed, l.filters, win.closeBranchLauncher) {
		return
	}
	for {
		ev, ok := l.ed.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			l.err = ""
			l.changed()
			win.previewBranch()
		case widget.SubmitEvent:
			win.submitBranch()
		}
	}
}
func (win *window) branchChannel() chan branchReply {
	l := &win.side.launch
	if l.replies == nil {
		l.replies = make(chan branchReply, 2)
	}
	return l.replies
}
func (win *window) previewBranch() {
	l := &win.side.launch
	if !l.open || l.busy || l.previewBusy || len(l.repos) == 0 {
		return
	}
	b := strings.TrimSpace(l.ed.Text())
	if b == "" {
		l.hint = "Enter a branch name"
		return
	}
	l.previewBusy = true
	l.previewBranch = b
	epoch, dir, replies, w := l.epoch, l.repos[l.repo].dir, win.branchChannel(), win.w
	l.hint = "Checking branch…"
	l.changed()
	go func() {
		in, e := branchsession.Inspect(context.Background(), dir, b)
		replies <- branchReply{epoch: epoch, branch: b, info: in, err: e}
		w.Invalidate()
	}()
}
func (win *window) submitBranch() {
	l := &win.side.launch
	if l.busy || l.work.removing || len(l.repos) == 0 {
		return
	}
	if workspaces.KindOf(win.opt.Shell) == workspaces.ShellUnknown && l.agent != "shell" {
		l.err = "Choose Shell, or use PowerShell, zsh, bash or sh."
		l.changed()
		return
	}
	l.busy = true
	l.ed.ReadOnly = true
	l.err = ""
	l.hint = "Creating worktree…"
	l.repoOpen = false
	l.changed()
	epoch, dir, b, replies, w, store := l.epoch, l.repos[l.repo].dir, l.ed.Text(), win.branchChannel(), win.w, win.branchStore()
	go func() {
		r, e := store.Start(context.Background(), dir, b)
		replies <- branchReply{epoch: epoch, branch: b, result: r, err: e, start: true}
		w.Invalidate()
	}()
}
func (win *window) drainBranchWork() {
	l := &win.side.launch
	for {
		select {
		case r := <-l.replies:
			if r.start {
				l.busy = false
				l.ed.ReadOnly = false
				if r.err != nil {
					l.err = branchError(r.err)
					l.hint = ""
				} else {
					win.finishBranchSession(r.result)
				}
				l.changed()
				continue
			}
			l.previewBusy = false
			if r.epoch == l.epoch && r.branch == strings.TrimSpace(l.ed.Text()) {
				if r.err != nil {
					l.hint = branchError(r.err)
				} else {
					l.hint = r.info.Hint
				}
				l.changed()
			} else {
				win.previewBranch()
			}
		default:
			win.drainWorktreeChecks()
			return
		}
	}
}
func branchError(e error) string {
	s := strings.Join(strings.Fields(cleanText(e.Error())), " ")
	return cutRunes(s, 160)
}
func (win *window) finishBranchSession(r branchsession.Result) {
	l := &win.side.launch
	t, e := win.mgr.NewOpts(tabs.StartOpts{Dir: r.Folder})
	if e != nil {
		l.err = "Could not open session: " + e.Error()
		return
	}
	if l.agent != "shell" {
		line := "claude"
		if l.agent == "codex" {
			line = "codex --no-daemon"
		}
		launch := &wsMember{tab: t.ID, title: r.Branch, run: workspaces.TabRun{Line: line, Started: win.wsNow()}}
		launch.run.Event(workspaces.StartedEvent{})
		if win.launches == nil {
			win.launches = map[tabs.TabID]*wsMember{}
		}
		win.launches[t.ID] = launch
		win.mgr.Watch(t.ID, true)
	}
	win.opt.Config.SidebarLauncherAgent = l.agent
	win.saveSetting("sidebar_launcher_agent", l.agent)
	l.open = false
	l.ed.SetText("")
	l.changed()
	win.hideSettings()
	win.selectSession(t.ID)
	win.wantFocus = true
	win.worktreeOpened(r.Folder)
	win.requestWorktreeCheck(false)
}
func (win *window) branchPress(p image.Point) bool {
	l, h := &win.side.launch, &win.side.hits
	if l.repoOpen {
		for i, r := range h.launchRepos {
			if p.In(r) {
				l.repo = i
				l.repoOpen = false
				l.epoch++
				l.err = ""
				l.changed()
				win.previewBranch()
				return true
			}
		}
		if !p.In(h.launchRepo) {
			l.repoOpen = false
			l.changed()
			return true
		}
	}
	switch {
	case p.In(h.launchToggle):
		if l.busy {
			return true
		}
		win.opt.Config.SidebarLauncher = !win.opt.Config.SidebarLauncher
		win.saveSetting("sidebar_launcher", win.opt.Config.SidebarLauncher)
		if !win.opt.Config.SidebarLauncher {
			win.closeBranchLauncher()
		}
		l.changed()
		return true
	case p.In(h.launchButton):
		win.openBranchLauncher()
		return true
	case p.In(h.launchCancel):
		win.closeBranchLauncher()
		return true
	case p.In(h.launchStart):
		win.submitBranch()
		return true
	case p.In(h.launchBranch):
		return true
	case p.In(h.launchRepo):
		if !l.busy && len(l.repos) > 1 {
			l.repoOpen = !l.repoOpen
			l.changed()
		}
		return true
	}
	for i, r := range h.launchAgents {
		if p.In(r) {
			if !l.busy {
				l.agent = []string{"claude", "codex", "shell"}[i]
				l.changed()
			}
			return true
		}
	}
	return win.worktreePress(p)
}
func (c *chrome) launcherHeight(gtx layout.Context, l *branchLauncher) int {
	if l == nil {
		return gtx.Dp(28)
	}
	if !l.shown {
		return gtx.Dp(28)
	}
	if l.open {
		return gtx.Dp(242)
	}
	return gtx.Dp(92)
}
func (c *chrome) drawBranchLauncher(gtx layout.Context, h *sideHits, l *branchLauncher, static bool, r image.Rectangle) {
	if l == nil || r.Empty() {
		return
	}
	ops, th := gtx.Ops, c.th
	cl := clip.Rect(r).Push(ops)
	defer cl.Pop()
	gap := gtx.Dp(8)
	paint.FillShape(ops, th.Border, clip.Rect(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1)).Op())
	h.launchToggle = image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+gtx.Dp(28))
	title, tail := "▸ NEW SESSION", "show"
	if l.shown {
		title, tail = "▾ NEW SESSION", "hide"
	}
	put := func(s string, box image.Rectangle, col bool) {
		fg := th.ActiveFG
		if !col {
			fg = th.Muted
		}
		call, sz := c.fitSide(gtx, s, 11, max(box.Dx()-2*gap, 0), fg)
		place(ops, call, image.Pt(box.Min.X+gap, box.Min.Y+(box.Dy()-sz.Y)/2))
	}
	put(title, h.launchToggle, false)
	call, sz := c.label(gtx, tail, 10, th.Muted)
	place(ops, call, image.Pt(r.Max.X-gap-sz.X, r.Min.Y+(gtx.Dp(28)-sz.Y)/2))
	if !l.shown {
		return
	}
	button := func(s string, b image.Rectangle, on bool) {
		border := th.Border
		if on {
			border = th.Accent
		}
		paint.FillShape(ops, border, clip.Stroke{Path: clip.UniformRRect(b, gtx.Dp(6)).Path(ops), Width: 1}.Op())
		if on {
			paint.FillShape(ops, tint(th.Accent, 0x22), clip.UniformRRect(b, gtx.Dp(6)).Op(ops))
		}
		put(s, b, true)
	}
	y := h.launchToggle.Max.Y
	if !l.open {
		h.launchButton = image.Rect(r.Min.X, y, r.Max.X, r.Max.Y-gtx.Dp(6))
		button("+ New session on a branch", h.launchButton, false)
		call, sz := c.label(gtx, l.work.shortcut, 10, th.Muted)
		place(ops, call, image.Pt(r.Max.X-gap-sz.X, h.launchButton.Max.Y-sz.Y-gtx.Dp(5)))
		return
	}
	panel := image.Rect(r.Min.X, y, r.Max.X, r.Max.Y-gtx.Dp(4))
	paint.FillShape(ops, th.Active, clip.UniformRRect(panel, gtx.Dp(7)).Op(ops))
	stroke(ops, panel, gtx.Dp(7), th.Accent)
	x0, x1 := panel.Min.X+gap, panel.Max.X-gap
	y += gtx.Dp(3)
	put("Repo", image.Rect(x0, y, x1, y+gtx.Dp(22)), false)
	y += gtx.Dp(22)
	h.launchRepo = image.Rect(x0, y, x1, y+gtx.Dp(28))
	name := "No repositories open"
	if len(l.repos) > 0 {
		name = l.repos[l.repo].name + " ▾"
	}
	button(name, h.launchRepo, false)
	y += gtx.Dp(32)
	put("Branch", image.Rect(x0, y, x1, y+gtx.Dp(20)), false)
	y += gtx.Dp(20)
	h.launchBranch = image.Rect(x0, y, x1, y+gtx.Dp(30))
	stroke(ops, h.launchBranch, gtx.Dp(5), th.Border)
	if !static {
		c.lineEditor(gtx, &l.ed, h.launchBranch, "fix/sidebar-rows", th.Muted, gap)
	}
	y += gtx.Dp(34)
	hint := l.hint
	if l.err != "" {
		hint = l.err
	}
	// Two bounded lines retain git's reason without widening the picker.
	for i, line := range []string{cutRunes(hint, 30), ""} {
		if i == 1 && len([]rune(hint)) > 30 {
			line = string([]rune(hint)[29:])
		}
		put(line, image.Rect(x0, y, x1, y+gtx.Dp(16)), false)
		y += gtx.Dp(16)
	}
	y += gtx.Dp(3)
	w := (x1 - x0 - 2*gtx.Dp(4)) / 3
	for i, name := range []string{"Claude", "Codex", "Shell"} {
		b := image.Rect(x0+i*(w+gtx.Dp(4)), y, x0+i*(w+gtx.Dp(4))+w, y+gtx.Dp(26))
		h.launchAgents[i] = b
		button(name, b, l.agent == strings.ToLower(name))
	}
	y += gtx.Dp(32)
	h.launchCancel = image.Rect(x0, y, x0+(x1-x0)/2, y+gtx.Dp(26))
	h.launchStart = image.Rect(h.launchCancel.Max.X+gtx.Dp(4), y, x1, y+gtx.Dp(26))
	button("Esc Cancel", h.launchCancel, false)
	start := "Enter Start"
	if l.busy {
		start = "Creating…"
	}
	button(start, h.launchStart, true)
	if l.repoOpen {
		rec := op.Record(ops)
		bounds := image.Rect(x0, h.launchRepo.Max.Y, x1, panel.Max.Y-gap)
		rowH := gtx.Dp(28)
		l.repoScroll = min(max(l.repoScroll, 0), max(0, len(l.repos)*rowH-bounds.Dy()))
		menuClip := clip.Rect(bounds).Push(ops)
		y = bounds.Min.Y - l.repoScroll
		for i, repo := range l.repos {
			b := image.Rect(x0, y, x1, y+rowH)
			if b.Overlaps(bounds) {
				paint.FillShape(ops, th.Chrome, clip.Rect(b).Op())
				button(repo.name, b, i == l.repo)
			}
			h.launchRepos = append(h.launchRepos, b.Intersect(bounds))
			y += rowH
		}
		menuClip.Pop()
		l.repoCall = rec.Stop()
		l.repoCall.Add(ops)
	}
}

func (win *window) initBranchShortcut() {
	w := &win.side.launch.work
	if w.shortcutReady {
		return
	}
	w.shortcutReady = true
	w.shortcut = win.keyText("new_branch_session")
}
