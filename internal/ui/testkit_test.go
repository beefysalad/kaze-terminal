package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
)

// Shared window fixtures and input helpers for the UI tests. They came from
// the floating pane tests, which went with the panes (issue #133).

// memberWindow has session a shown (pty 0), session b (pty 1) and two
// workspace members m1, m2 (ptys 2, 3); nothing is docked yet.
func memberWindow(t *testing.T) (win *window, st *recStarter, r *input.Router, a, b, m1, m2 *tabs.Tab) {
	t.Helper()
	win, st = sideWindow(t, true)
	a = openIn(t, win, osPath("/code/alpha/a"))
	b = openIn(t, win, osPath("/code/beta/b"))
	m1 = openMember(t, win, "api")
	m2 = openMember(t, win, "web")
	win.mgr.SetActive(a.ID)
	win.wantFocus = true
	r = new(input.Router)
	sideFrame(t, win, r)
	return
}

// twoFrames draws twice after the panel changes: the first frame lays out
// and draws it, the next one has its input filters up, as the window's own
// wake does.
func twoFrames(t *testing.T, win *window, r *input.Router) {
	t.Helper()
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

// frameAt runs one 1000x600 frame at now.
func frameAt(t *testing.T, win *window, r *input.Router, now time.Time) {
	t.Helper()
	gtx := pageContext(1000, 600)
	gtx.Source, gtx.Now = r.Source(), now
	if err := win.frame(gtx); err != nil {
		t.Fatal(err)
	}
	r.Frame(gtx.Ops)
}

func ptyOf(st *recStarter, i int) *recPTY {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.ptys[i]
}

func at(p image.Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }

// clickAt presses and releases the primary button at window point p.
func clickAt(t *testing.T, win *window, r *input.Router, p image.Point) {
	t.Helper()
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at(p)},
	)
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

// dragFrom drags with the primary button from p by d in steps, one frame each.
func dragFrom(t *testing.T, win *window, r *input.Router, p, d image.Point) {
	t.Helper()
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(p)})
	sideFrame(t, win, r)
	for i := 1; i <= 4; i++ {
		q := p.Add(d.Mul(i).Div(4))
		r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at(q)})
		sideFrame(t, win, r)
	}
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at(p.Add(d))})
	sideFrame(t, win, r)
	sideFrame(t, win, r)
}

func typeKeys(t *testing.T, win *window, r *input.Router, s string) {
	t.Helper()
	r.Queue(key.EditEvent{Text: s})
	sideFrame(t, win, r)
}

// emitTo writes s to pty i as the shell's output and waits, at most three
// seconds, until done says tab applied it.
func emitTo(t *testing.T, st *recStarter, i int, tab *tabs.Tab, s string, done func() bool) {
	t.Helper()
	go ptyOf(st, i).w.Write([]byte(s))
	deadline := time.Now().Add(3 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("output %q never applied", s)
		}
		time.Sleep(time.Millisecond)
	}
}

// clickMember clicks member id's line in the sidebar's Workspaces section.
func clickMember(t *testing.T, win *window, r *input.Router, id tabs.TabID) {
	t.Helper()
	sideClick(win, r, memberHit(t, win, id).rect, pointer.ButtonPrimary)
	twoFrames(t, win, r)
}

func historyOffset(tab *tabs.Tab) int {
	tab.Session.Lock()
	defer tab.Session.Unlock()
	return tab.Session.Screen().ViewOffset()
}
