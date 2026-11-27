package ui

import (
	"gioui.org/layout"
	"gioui.org/op"
	"image/color"
	"testing"
)

func TestChromeLabelIsProportional(t *testing.T) {
	c := newChrome()
	var ops op.Ops
	g := layout.Context{Ops: &ops}
	_, a := c.label(g, "iiiiiiii", 13, color.NRGBA{})
	_, b := c.label(g, "WWWWWWWW", 13, color.NRGBA{})
	if a.X >= b.X {
		t.Fatal(a, b)
	}
	_, a = c.labelMono(g, "iiiiiiii", 13, color.NRGBA{})
	_, b = c.labelMono(g, "WWWWWWWW", 13, color.NRGBA{})
	if a.X != b.X {
		t.Fatal(a, b)
	}
}
