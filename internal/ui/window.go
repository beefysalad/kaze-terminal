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

	"github.com/patrick-gogo/kaze/internal/config"
	"github.com/patrick-gogo/kaze/internal/input"
	"github.com/patrick-gogo/kaze/internal/ptyx"
	"github.com/patrick-gogo/kaze/internal/render"
	"github.com/patrick-gogo/kaze/internal/tabs"
	"github.com/patrick-gogo/kaze/internal/term"
)

// Options are what main resolved at startup.
type Options struct {
	Config   config.Config
	Warnings []string // shown in the first tab
	Shell    []string // argv, already resolved to the platform default when the config has none
	Dir      string   // working directory for new shells
}

const (
	allMods = key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper | key.ModCommand
	// maxPaste bounds a clipboard read so a giant clipboard can't exhaust memory.
	maxPaste = 64 << 20
	// memEvery limits how often the status bar reads memory stats (ReadMemStats stops the world).
	memEvery = 2 * time.Second
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
			if win.mgr != nil {
				win.mgr.CloseAll()
			}
			return e.Err
		case app.ConfigEvent:
			win.setFocused(e.Config.Focused)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if err := win.frame(gtx); err != nil {
				return err
			}
			e.Frame(gtx.Ops)
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
		win.mgr = tabs.NewManager(win.start, cols, rows, win.w.Invalidate)
		t, err := win.mgr.New()
		if err != nil {
			return fmt.Errorf("start %q: %w", win.opt.Shell, err)
		}
		injectWarnings(t.Session, win.opt.Warnings)
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

	ops := gtx.Ops
	list := win.mgr.Tabs()

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
	win.rend.Layout(tg, active.Session, nil)
	off.Pop()
	g.Pop()

	// Status bar.
	st := op.Offset(image.Pt(0, size.Y-statusH)).Push(ops)
	sg := gtx
	sg.Constraints = layout.Exact(image.Pt(size.X, statusH))
	win.ui.layoutStatus(sg, win.statusText(len(list)), statusH)
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
	// The renderer's cells belong to the previous tab; the cell diff then redraws only what differs.
	t.Session.Lock()
	t.Session.Screen().MarkAllDirty()
	t.Session.Unlock()
	win.updateTitle(t)
}

func (win *window) updateTitle(t *tabs.Tab) {
	title := "kaze · " + tabLabel(t, win.defTitle)
	if title != win.winTitle {
		win.winTitle = title
		win.w.Option(app.Title(title))
	}
}

func (win *window) statusText(n int) string {
	if now := time.Now(); now.Sub(win.memRead) > memEvery {
		mb, ok := processMB()
		if !ok {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			mb = float64(ms.Sys-ms.HeapReleased) / (1 << 20)
		}
		win.memMB = mb
		win.memRead = now
	}
	unit := "tabs"
	if n == 1 {
		unit = "tab"
	}
	return fmt.Sprintf("kaze %.0fMB · %d %s", win.memMB, n, unit)
}

func (win *window) barEvents(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: win.barTag, Kinds: pointer.Press})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok || e.Kind != pointer.Press {
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
	kinds := pointer.Press | pointer.Release | pointer.Scroll
	if m.MouseButton || m.MouseAny {
		kinds |= pointer.Drag
	}
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
	edited := false
	for i, ev := range batch {
		switch e := ev.(type) {
		case key.Event:
			win.onKey(gtx, t, e, m)
			if win.mgr.Active() != t { // the key switched or closed tabs; drop the rest of this frame's input
				return
			}
		case key.CompositionEvent:
			win.composing = compositionActive(e)
		case key.EditEvent:
			edited = true
			if win.gate.allow() && editCommits(batch, i, win.composing) && e.Text != "" {
				t.Session.Write(input.EncodeText(e.Text))
				win.snapToBottom(t.Session)
			}
		case transfer.DataEvent:
			win.paste(t, e, m)
		case pointer.Event:
			win.onPointer(t, e, m, cw, ch)
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
		// TODO(Task 12): copy the selection once selection lands.
	default:
		if out.data != nil {
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

func buttonOf(b pointer.Buttons) int {
	switch {
	case b.Contain(pointer.ButtonPrimary):
		return 0
	case b.Contain(pointer.ButtonTertiary):
		return 1
	case b.Contain(pointer.ButtonSecondary):
		return 2
	}
	return -1
}

func (win *window) onPointer(t *tabs.Tab, e pointer.Event, m term.Modes, cw, ch int) {
	p := e.Position.Round().Sub(win.gridMin)
	col, row := 0, 0
	if cw > 0 && ch > 0 {
		cols, rows := win.mgr.Size()
		col = min(max(p.X/cw, 0), cols-1)
		row = min(max(p.Y/ch, 0), rows-1)
	}
	switch e.Kind {
	case pointer.Press:
		win.wantFocus = true
		win.w.Invalidate()
		if mouseOn(m) {
			t.Session.Write(input.EncodeMouse(buttonOf(e.Buttons), true, e.Modifiers, col, row, false, m))
		}
	case pointer.Release:
		if mouseOn(m) {
			// Gio reports the buttons still held; the released one is unknown, so report it as left.
			t.Session.Write(input.EncodeMouse(0, false, e.Modifiers, col, row, false, m))
		}
	case pointer.Drag, pointer.Move:
		if mouseOn(m) {
			t.Session.Write(input.EncodeMouse(buttonOf(e.Buttons), true, e.Modifiers, col, row, true, m))
		}
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
