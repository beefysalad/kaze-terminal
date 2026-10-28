package screen_test

import (
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
)

func TestColorAccessors(t *testing.T) {
	c := screen.RGB(1, 2, 3)
	if !c.IsRGB() || c.IsDefault() {
		t.Fatal("RGB color not reported as truecolor")
	}
	if r, g, b := c.RGB(); r != 1 || g != 2 || b != 3 {
		t.Fatalf("RGB() = %d,%d,%d", r, g, b)
	}
	i := screen.Indexed(200)
	if i.IsRGB() || i.IsDefault() || i.Index() != 200 {
		t.Fatal("indexed color accessors wrong")
	}
	if !screen.DefaultColor.IsDefault() || screen.DefaultColor.IsRGB() {
		t.Fatal("default color accessors wrong")
	}
}
