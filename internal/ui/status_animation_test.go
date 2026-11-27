package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func animCtx(ops *op.Ops) layout.Context {
	return layout.Context{Ops: ops, Now: time.Unix(1000, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(100, 100))}
}

// The still mark never asks for a frame; only the animated one flags that it was drawn.
func TestRunningMarkFlagsOnlyWhenAnimated(t *testing.T) {
	c := newChrome()
	slot := image.Rect(0, 0, 14, 14)
	c.runningMark(animCtx(new(op.Ops)), slot)
	if c.animDrawn {
		t.Fatal("the still mark flagged an animation")
	}
	c.animate = true
	c.runningMark(animCtx(new(op.Ops)), slot)
	if !c.animDrawn {
		t.Fatal("the animated mark did not flag itself")
	}
}

func TestStatusAnimationWakesOnlyWhenNeeded(t *testing.T) {
	win := testWindow(t, &recStarter{})
	if win.ui == nil {
		win.ui = newChrome()
	}
	t0 := time.Unix(1000, 0)
	cases := []struct {
		name            string
		on, focus, drew bool
		wake            bool
	}{
		{"off", false, true, true, false},
		{"unfocused", true, false, true, false},
		{"nothing running on screen", true, true, false, false},
		{"on, focused, a running mark drawn", true, true, true, true},
	}
	for _, tc := range cases {
		win.opt.Config.AnimateStatus, win.focused, win.ui.animDrawn = tc.on, tc.focus, tc.drew
		next, ok := win.statusWake(t0)
		if ok != tc.wake || (ok && !next.Equal(t0.Add(statusAnimEvery))) {
			t.Fatalf("%s: wake %v %v", tc.name, next, ok)
		}
		if win.ui.animDrawn {
			t.Fatalf("%s: the flag was not cleared for the next frame", tc.name)
		}
	}
	win.opt.Config.AnimateStatus, win.focused = true, true
	win.beginStatusFrame()
	if !win.ui.animate {
		t.Fatal("the frame did not enable the animated mark")
	}
	win.focused = false
	win.beginStatusFrame()
	if win.ui.animate {
		t.Fatal("an unfocused window animated")
	}
}
