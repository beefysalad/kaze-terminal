// Package ui is kaze's main window: the tab bar, the active tab's terminal and
// the status bar, plus keyboard, mouse, clipboard and focus handling.
package ui

import (
	"fmt"
	"image"
	"io"
	"log"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/patrick-gogo/kaze/internal/bench"
	"github.com/patrick-gogo/kaze/internal/config"
	"github.com/patrick-gogo/kaze/internal/input"
	"github.com/patrick-gogo/kaze/internal/ptyx"
	"github.com/patrick-gogo/kaze/internal/render"
	"github.com/patrick-gogo/kaze/internal/screen"
	"github.com/patrick-gogo/kaze/internal/tabs"
	"github.com/patrick-gogo/kaze/internal/term"
)

// Options are what main resolved at startup.
type Options struct {
	Config   config.Config
	Warnings []string        // shown in the first tab
	Shell    []string        // argv, already resolved to the platform default when the config has none
	Dir      string          // working directory for new shells
	Bench    *bench.Recorder // nil unless KAZE_BENCH is set
}

const (
	allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand
	// maxPaste bounds a clipboard read so a giant clipboard can't exhaust memory.
	maxPaste = 64 << 20
	// memEvery limits how often the status bar reads memory stats (ReadMemStats stops the world).
	memEvery = 2 * time.Second
	// noticeFor is how long an error stays in the status bar.
	noticeFor = 15 * time.Second
)

type window struct {
	w   *app.Window
	opt Options
	pal render.Palette
	km  keymap

	mgr       *tabs.Manager
	rend      *render.Renderer
	rendPx    float32
	ui        *chrome
	termTag   *int
	barTag    *int
	defTitle  string
	winTitle  string
	lastTab   *tabs.Tab
	focused   bool
	wantFocus bool

	wheel     wheel
	gate      textGate
	composing bool
	gridMin   image.Point // top-left of the grid in window pixels

	memMB   float64
	memRead time.Time

	notice      string // a recent error for the status bar
	noticeUntil time.Time

	sels       map[tabs.TabID]*tabSel
	clicks     clicks
	held       pointer.Buttons // buttons down, to tell which one a press or release changed
	toApp      bool            // the current mouse gesture is reported to the app, not selecting
	gestureTab *tabs.Tab       // the tab the held gesture started in
	gestureCol int             // last cell reported to the app, for a release sent by endGesture
	gestureRow int
	dragPos    image.Point // last drag position relative to the grid, for auto-scroll
	scrolledAt time.Time

	toClipboard func(string) // replaces the Gio clipboard write in tests

	bench *bench.Recorder // nil unless KAZE_BENCH is set; every hook checks it first
	bf    benchFrame
}

// Run drives the window until it closes. It returns after every shell is closed.
func Run(w *app.Window, opt Options) error {
	km, warns := buildKeymap(runtime.GOOS, opt.Config.Keys)
	input.SetReserved(runtime.GOOS, km.shortcuts())
	opt.Warnings = append(opt.Warnings, warns...)
	win := &window{
		w: w, opt: opt, km: km,
		pal:      applyColors(render.DefaultPalette(), opt.Config.Colors),
		ui:       newChrome(),
		termTag:  new(int),
		barTag:   new(int),
		defTitle: shellName(opt.Shell),
		focused:  true, wantFocus: true,
		sels:  map[tabs.TabID]*tabSel{},
		bench: opt.Bench,
	}
	return win.loop()
}

func shellName(argv []string) string {
	if len(argv) == 0 {
		return "shell"
	}
	b := filepath.Base(argv[0])
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func (win *window) loop() error {
	var ops op.Ops
	for {
		switch e := win.w.Event().(type) {
		case app.DestroyEvent:
			if win.bench != nil {
				win.benchScreen()
				win.bench.Close(privateMB())
			}
			if win.mgr != nil {
				win.mgr.CloseAll()
			}
			return e.Err
		case app.ConfigEvent:
			win.setFocused(e.Config.Focused)
		case app.FrameEvent:
			start := time.Now()
			gtx := app.NewContext(&ops, e)
			if err := win.frame(gtx); err != nil {
				return err
			}
			e.Frame(gtx.Ops)
			if win.bench != nil {
				win.bench.FrameDone(start, time.Now(), win.bf.frame)
				win.bf = benchFrame{}
				if win.bench.ShouldQuit() {
					win.w.Perform(system.ActionClose)
				}
			}
			if win.mgr != nil && win.mgr.Empty() {
				win.w.Perform(system.ActionClose)
			}
		}
	}
}

func (win *window) setFocused(f bool) {
	if f == win.focused {
		return
	}
	win.focused = f
	if win.rend != nil {
		win.rend.SetFocused(f)
	}
	if win.mgr != nil {
		if t := win.mgr.Active(); t != nil {
			sendFocus(t, f)
		}
	}
	win.w.Invalidate()
}

func sendFocus(t *tabs.Tab, focused bool) {
	if b := input.EncodeFocus(focused, modes(t.Session)); b != nil {
		t.Session.Write(b)
	}
}

func modes(s *term.Session) term.Modes {
	s.Lock()
	defer s.Unlock()
	return s.Applier().Modes()
}

func (win *window) start(env []string) (ptyx.Session, error) {
	cols, rows := win.mgr.Size()
	return ptyx.Start(win.opt.Shell, cols, rows, env, win.opt.Dir)
}

func (win *window) newTab() {
	if _, err := win.mgr.New(); err != nil {
		log.Printf("kaze: new tab: %v", err)
		// A windowsgui build has no console, so the status bar is where this can be seen.
		win.notice = "new tab failed: " + err.Error()
		win.noticeUntil = time.Now().Add(noticeFor)
		win.w.Invalidate()
	}
}

func (win *window) frame(gtx layout.Context) error {
	barH, statusH, pad := gtx.Dp(38), gtx.Dp(24), gtx.Dp(8)
	px := gtx.Metric.PxPerSp * win.opt.Config.FontSize
	if win.rend == nil || px != win.rendPx {
		// A DPI change (another monitor) needs glyphs at the new size.
		win.rend = render.NewRenderer(px, win.pal)
		win.rend.SetFocused(win.focused)
		win.rendPx = px
	}
	cw, ch := win.rend.CellSize()
	size := gtx.Constraints.Max
	termRect := image.Rect(0, barH, size.X, max(size.Y-statusH, barH))
	grid := image.Rect(termRect.Min.X+pad, termRect.Min.Y+pad/2, termRect.Max.X-pad, termRect.Max.Y-pad/2)
	win.gridMin = grid.Min
	cols, rows := gridSize(grid.Dx(), grid.Dy(), cw, ch)

	if win.mgr == nil {
		// The first shell starts at the real window size, not a guess.
		notify := win.w.Invalidate
		if win.bench != nil {
			notify = func() { win.bench.Output(time.Now()); win.w.Invalidate() }
		}
		win.mgr = tabs.NewManager(win.start, cols, rows, notify)
		win.mgr.SetScrollback(win.opt.Config.ScrollbackLines)
		var def []string
		if len(win.opt.Config.Shell) > 0 {
			def = ptyx.Resolve(ptyx.DefaultShell())
		}
		if _, _, err := openFirst(win.mgr, &win.opt.Shell, def, win.opt.Warnings); err != nil {
			return fmt.Errorf("start %q: %w", win.opt.Shell, err)
		}
		if win.bench != nil {
			for range win.bench.ExtraTabs() {
				win.newTab()
			}
			win.bench.Start(win.w.Invalidate, privateMB)
		}
	}
	win.mgr.ResizeAll(cols, rows)

	win.barEvents(gtx)
	active := win.mgr.Active()
	if active != nil {
		win.termEvents(gtx, active, cw, ch)
	}
	if win.mgr.Empty() {
		return nil
	}
	active = win.mgr.Active()
	win.switchedTo(active)
	list := win.mgr.Tabs()
	win.pruneSels(list)
	win.autoScroll(gtx, active, cw, ch)
	sel := win.checkSel(active)

	ops := gtx.Ops

	// Tab bar.
	bar := gtx
	bar.Constraints = layout.Exact(image.Pt(size.X, barH))
	win.ui.layoutTabBar(bar, list, active.ID, win.defTitle, barH)
	area := clip.Rect{Max: image.Pt(size.X, barH)}.Push(ops)
	event.Op(ops, win.barTag)
	area.Pop()

	// Terminal: only the active tab is drawn.
	paint.FillShape(ops, win.pal.BG, clip.Rect(termRect).Op())
	area = clip.Rect(termRect).Push(ops)
	event.Op(ops, win.termTag)
	area.Pop()
	g := clip.Rect(grid).Push(ops)
	off := op.Offset(grid.Min).Push(ops)
	tg := gtx
	tg.Constraints = layout.Exact(grid.Size())
	if win.bench != nil {
		win.benchBeforeDraw(active)
	}
	win.rend.Layout(tg, active.Session, &sel)
	if win.bench != nil {
		win.benchAfterDraw(active)
	}
	off.Pop()
	g.Pop()

	// Status bar.
	st := op.Offset(image.Pt(0, size.Y-statusH)).Push(ops)
	sg := gtx
	sg.Constraints = layout.Exact(image.Pt(size.X, statusH))
	win.ui.layoutStatus(sg, win.statusText(len(list)), statusH)
	if at, ok := win.noticeWake(time.Now()); ok {
		gtx.Execute(op.InvalidateCmd{At: at}) // so the notice goes away on an idle window
	}
	st.Pop()

	if win.wantFocus {
		gtx.Execute(key.FocusCmd{Tag: win.termTag})
		win.wantFocus = false
	}
	return nil
}

// switchedTo redraws a newly shown tab in full and moves focus reports to it.
func (win *window) switchedTo(t *tabs.Tab) {
	if t == win.lastTab {
		win.updateTitle(t)
		return
	}
	if win.lastTab != nil && win.focused {
		sendFocus(win.lastTab, false) // a no-op once that tab is closed
	}
	if win.focused {
		sendFocus(t, true)
	}
	win.lastTab = t
	win.endGesture()
	// One renderer serves every tab: dropping its strips frees the hidden tab's
	// buffers and makes the shown tab draw in full.
	if win.rend != nil {
		win.rend.Release()
	}
	win.updateTitle(t)
}

func (win *window) updateTitle(t *tabs.Tab) {
	title := "kaze · " + tabLabel(t, win.defTitle)
	if title != win.winTitle {
		win.winTitle = title
		win.w.Option(app.Title(title))
	}
}

// privateMB is the process's private memory, or the Go runtime's own count where the OS number is unavailable.
func privateMB() float64 {
	if mb, ok := processMB(); ok {
		return mb
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.Sys-ms.HeapReleased) / (1 << 20)
}

func (win *window) statusText(n int) string {
	if now := time.Now(); now.Sub(win.memRead) > memEvery {
		win.memMB = privateMB()
		win.memRead = now
	}
	unit := "tabs"
	if n == 1 {
		unit = "tab"
	}
	s := fmt.Sprintf("kaze %.0fMB · %d %s", win.memMB, n, unit)
	if _, ok := win.noticeWake(time.Now()); ok {
		s += " · " + win.notice
	}
	return s
}

func (win *window) barEvents(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: win.barTag, Kinds: pointer.Press})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Kind != pointer.Press || win.bench.IgnoreInput() {
			continue
		}
		win.wantFocus = true
		id, onClose, onPlus := win.ui.hits.at(e.Position.Round())
		switch {
		case onPlus && e.Buttons.Contain(pointer.ButtonPrimary):
			win.newTab()
		case id == 0:
		case onClose && e.Buttons.Contain(pointer.ButtonPrimary), e.Buttons.Contain(pointer.ButtonTertiary):
			win.mgr.Close(id)
		case e.Buttons.Contain(pointer.ButtonPrimary):
			win.mgr.SetActive(id)
		}
	}
}

func (win *window) termEvents(gtx layout.Context, t *tabs.Tab, cw, ch int) {
	m := modes(t.Session)
	kinds := pointer.Press | pointer.Release | pointer.Scroll | pointer.Drag | pointer.Cancel
	if m.MouseAny {
		kinds |= pointer.Move // only then, so hovering never wakes the idle window
	}
	var batch []event.Event
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: win.termTag},
			key.Filter{Focus: win.termTag, Optional: allMods},
			// Tab is a focus key in Gio and is only delivered to filters that name it.
			key.Filter{Focus: win.termTag, Name: key.NameTab, Optional: allMods},
			transfer.TargetFilter{Target: win.termTag, Type: "application/text"},
			pointer.Filter{Target: win.termTag, Kinds: kinds, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		)
		if !ok {
			break
		}
		batch = append(batch, ev)
	}
	if win.bench != nil {
		if win.bench.IgnoreInput() {
			batch = batch[:0]
		}
		batch = win.benchInput(gtx, t, batch, ch)
	}
	win.route(gtx, batch, cw, ch)
}

// route delivers one frame's terminal input to the active tab.
func (win *window) route(gtx layout.Context, batch []event.Event, cw, ch int) {
	t := win.mgr.Active()
	if t == nil {
		return
	}
	m := modes(t.Session)
	edited := false
	for i, ev := range batch {
		switch e := ev.(type) {
		case key.Event:
			win.onKey(gtx, t, e, m)
			if a := win.mgr.Active(); a != t {
				// The key opened, closed or switched tabs; the rest of the frame's input goes to the new one.
				win.endGesture()
				if a == nil {
					return
				}
				t, m = a, modes(a.Session)
			}
		case key.CompositionEvent:
			win.composing = compositionActive(e)
		case key.EditEvent:
			edited = true
			if win.gate.allow() && editCommits(batch, i, win.composing) && e.Text != "" {
				win.benchKey(t)
				t.Session.Write(input.EncodeText(e.Text))
				win.snapToBottom(t.Session)
			}
		case transfer.DataEvent:
			win.paste(t, e, m)
		case pointer.Event:
			win.onPointer(gtx, t, e, m, cw, ch)
		}
	}
	if edited && !win.composing {
		// Gio keeps typed text as editor state; clear it so it doesn't grow forever.
		gtx.Execute(key.SnippetCmd{Tag: win.termTag})
		gtx.Execute(key.SelectionCmd{Tag: win.termTag})
	}
}

func (win *window) onKey(gtx layout.Context, t *tabs.Tab, e key.Event, m term.Modes) {
	if e.State != key.Press {
		win.gate.onKey(false, keyOut{})
		return
	}
	out := routeKey(runtime.GOOS, win.km, e.Name, e.Modifiers, m)
	win.gate.onKey(true, out)
	switch out.act {
	case actNewTab:
		win.newTab()
	case actCloseTab:
		win.mgr.Close(t.ID)
	case actNextTab:
		win.mgr.Next()
	case actPrevTab:
		win.mgr.Prev()
	case actPaste:
		gtx.Execute(clipboard.ReadCmd{Tag: win.termTag})
	case actCopy:
		win.copySel(gtx, t)
	default:
		if out.data != nil {
			win.benchKey(t)
			t.Session.Write(out.data)
			win.snapToBottom(t.Session)
		}
	}
}

func (win *window) paste(t *tabs.Tab, e transfer.DataEvent, m term.Modes) {
	r := e.Open()
	b, err := io.ReadAll(io.LimitReader(r, maxPaste))
	r.Close()
	if err != nil || len(b) == 0 {
		return
	}
	t.Session.Write(input.EncodePaste(string(b), m))
	win.snapToBottom(t.Session)
}

func mouseOn(m term.Modes) bool { return m.MouseX10 || m.MouseButton || m.MouseAny }

func (win *window) onPointer(gtx layout.Context, t *tabs.Tab, e pointer.Event, m term.Modes, cw, ch int) {
	p := e.Position.Round().Sub(win.gridMin)
	cols, rows := win.mgr.Size()
	col, row := gridCell(p, cw, ch, cols, rows)
	switch e.Kind {
	case pointer.Press:
		win.wantFocus = true
		win.w.Invalidate()
		btn := pressedButton(win.held, e.Buttons)
		first := win.held == 0
		win.held = e.Buttons
		if first {
			// Shift lets the user select even when the app has the mouse.
			win.toApp = mouseOn(m) && e.Modifiers&key.ModShift == 0
			win.gestureTab = t
		}
		win.gestureCol, win.gestureRow = col, row
		if win.toApp {
			t.Session.Write(input.EncodeMouse(btn, true, e.Modifiers, col, row, false, m))
			return
		}
		if btn == 0 {
			win.selectPress(t, p, e.Time, cw, ch)
		}
	case pointer.Release:
		btn := releasedButton(win.held, e.Buttons)
		win.held = e.Buttons
		if win.held == 0 {
			win.gestureTab = nil
		}
		if win.toApp {
			if mouseOn(m) {
				t.Session.Write(input.EncodeMouse(btn, false, e.Modifiers, col, row, false, m))
			}
			return
		}
		if btn == 0 {
			win.selectRelease(gtx, t)
		}
	case pointer.Cancel:
		win.held, win.toApp, win.gestureTab = 0, false, nil
		if ts := win.sels[t.ID]; ts != nil {
			ts.end()
		}
	case pointer.Drag, pointer.Move:
		if win.toApp || e.Kind == pointer.Move {
			win.gestureCol, win.gestureRow = col, row
			if mouseOn(m) {
				t.Session.Write(input.EncodeMouse(xtermButton(e.Buttons), true, e.Modifiers, col, row, true, m))
			}
			return
		}
		win.selectDrag(t, p, cw, ch)
	case pointer.Scroll:
		unit, perNotch := float32(120), 3 // Windows reports 120 per wheel notch
		if runtime.GOOS == "darwin" {
			unit, perNotch = float32(ch), 1 // macOS reports pixels
		}
		n := win.wheel.notches(e.Scroll.Y, unit)
		if n == 0 {
			return
		}
		if b := input.EncodeWheel(n, col, row, m); b != nil {
			t.Session.Write(b)
			return
		}
		if m.AltScreen {
			return // full-screen apps have no history to scroll
		}
		s := t.Session
		s.Lock()
		s.Screen().SetViewOffset(s.Screen().ViewOffset() + n*perNotch)
		s.Unlock()
		win.w.Invalidate()
	}
}

// snapToBottom returns a scrolled-back view to the live screen when the user types.
func (win *window) snapToBottom(s *term.Session) {
	s.Lock()
	moved := s.Screen().ViewOffset() > 0
	if moved {
		s.Screen().SetViewOffset(0)
	}
	s.Unlock()
	if moved {
		win.w.Invalidate()
	}
}

func (win *window) selOf(id tabs.TabID) *tabSel {
	ts := win.sels[id]
	if ts == nil {
		ts = &tabSel{}
		win.sels[id] = ts
	}
	return ts
}

// pruneSels forgets the selections of closed tabs.
func (win *window) pruneSels(list []*tabs.Tab) {
	if len(win.sels) <= len(list) {
		return
	}
	keep := make(map[tabs.TabID]*tabSel, len(list))
	for _, t := range list {
		if ts := win.sels[t.ID]; ts != nil {
			keep[t.ID] = ts
		}
	}
	win.sels = keep
}

// pointAt maps a grid pixel to an absolute point; call with the session locked.
func (win *window) pointAt(scr *screen.Screen, p image.Point, cw, ch int) render.Point {
	cols, rows := scr.Size()
	return absPoint(p, cw, ch, cols, rows, scr.ScrollbackLen()-scr.ViewOffset())
}

func (win *window) selectPress(t *tabs.Tab, p image.Point, at time.Duration, cw, ch int) {
	s := t.Session
	s.Lock()
	scr := s.Screen()
	ap := win.pointAt(scr, p, cw, ch)
	win.selOf(t.ID).begin(scr, ap, win.clicks.press(at, ap))
	s.Unlock()
	win.dragPos = p
	win.w.Invalidate()
}

func (win *window) selectDrag(t *tabs.Tab, p image.Point, cw, ch int) {
	ts := win.sels[t.ID]
	if ts == nil || !ts.dragging {
		return
	}
	s := t.Session
	s.Lock()
	ts.extend(s.Screen(), win.pointAt(s.Screen(), p, cw, ch))
	s.Unlock()
	win.dragPos = p
	win.w.Invalidate()
}

func (win *window) selectRelease(gtx layout.Context, t *tabs.Tab) {
	ts := win.sels[t.ID]
	if ts == nil || !ts.dragging {
		return
	}
	ts.end()
	if win.opt.Config.CopyOnSelect && ts.sel.Active {
		win.copySel(gtx, t)
	}
}

// autoScroll scrolls the view while a selection drag is held above or below the
// grid. It reports whether it asked for another frame.
func (win *window) autoScroll(gtx layout.Context, t *tabs.Tab, cw, ch int) bool {
	ts := win.sels[t.ID]
	if ts == nil || !ts.dragging || win.toApp || ch <= 0 {
		return false
	}
	_, rows := win.mgr.Size()
	dir := edgeDir(win.dragPos.Y, rows*ch)
	if dir == 0 {
		return false
	}
	if gtx.Now.Sub(win.scrolledAt) >= autoScrollEvery {
		win.scrolledAt = gtx.Now
		s := t.Session
		s.Lock()
		scr := s.Screen()
		if !scr.OnAltScreen() { // full-screen apps have no history to scroll
			scr.SetViewOffset(scr.ViewOffset() - dir)
			ts.extend(scr, win.pointAt(scr, win.dragPos, cw, ch))
		}
		s.Unlock()
	}
	gtx.Execute(op.InvalidateCmd{At: win.scrolledAt.Add(autoScrollEvery)})
	return true
}

// checkSel drops or shifts the active tab's selection when the screen moved
// under it, and returns what to draw.
func (win *window) checkSel(t *tabs.Tab) render.Selection {
	ts := win.sels[t.ID]
	if ts == nil {
		return render.Selection{}
	}
	t.Session.Lock()
	ts.check(t.Session.Screen())
	t.Session.Unlock()
	return ts.sel
}

func (win *window) copySel(gtx layout.Context, t *tabs.Tab) {
	ts := win.sels[t.ID]
	if ts == nil || !ts.sel.Active {
		return
	}
	t.Session.Lock()
	text := render.SelectedText(t.Session.Screen(), ts.sel)
	t.Session.Unlock()
	if text == "" {
		return
	}
	if win.toClipboard != nil {
		win.toClipboard(text)
		return
	}
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
}

// openFirst opens the first tab with the warnings written in before the shell
// starts. When the configured shell fails and def is set, it retries with def
// and adds a warning that says so.
func openFirst(m *tabs.Manager, shell *[]string, def, warns []string) (*tabs.Tab, string, error) {
	prefeed := func(w []string) func(*term.Session) {
		return func(s *term.Session) { injectWarnings(s, w) }
	}
	t, err := m.NewWith(prefeed(warns))
	if err == nil || len(def) == 0 || slices.Equal(*shell, def) {
		return t, "", err
	}
	warn := fmt.Sprintf("shell %q failed: %v; using the default shell", strings.Join(*shell, " "), err)
	*shell = def
	t, err = m.NewWith(prefeed(append(slices.Clip(warns), warn)))
	if err != nil {
		err = fmt.Errorf("%s; the default shell failed too: %w", warn, err)
	}
	return t, warn, err
}

// noticeWake is when the status-bar notice expires, if one is showing; an expired one is dropped.
func (win *window) noticeWake(now time.Time) (time.Time, bool) {
	if win.notice != "" && !now.Before(win.noticeUntil) {
		win.notice = ""
	}
	return win.noticeUntil, win.notice != ""
}

// endGesture ends a held mouse gesture when the tab changes under it: an app
// that saw the press gets its release, and a selection drag stops, so the
// release landing on another tab can't leave either one stuck.
func (win *window) endGesture() {
	if g := win.gestureTab; g != nil {
		if win.toApp {
			m := modes(g.Session)
			for _, b := range []pointer.Buttons{pointer.ButtonPrimary, pointer.ButtonTertiary, pointer.ButtonSecondary} {
				if win.held.Contain(b) {
					g.Session.Write(input.EncodeMouse(xtermButton(b), false, 0, win.gestureCol, win.gestureRow, false, m))
				}
			}
		}
		if ts := win.sels[g.ID]; ts != nil {
			ts.end()
		}
	}
	win.held, win.toApp, win.gestureTab = 0, false, nil
}
