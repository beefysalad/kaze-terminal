package ui

import "testing"

func TestStatusBarMeasuresRealFace(t *testing.T) {
	c := newChrome()
	m := c.statusMeasure(pageContext(480, 320))
	if m("iiiiiiii", true) <= m("iiiiiiii", false) {
		t.Fatal("mono and prose need distinct measurements")
	}
}
