package term

import (
	"testing"

	"github.com/patrick-gogo/kaze/internal/screen"
)

func TestSGRBasicAndReset(t *testing.T) {
	s := run(10, 1, "\x1b[1;31ma\x1b[0mb")
	a, b := s.Cell(0, 0), s.Cell(1, 0)
	if a.FG != screen.Indexed(1) || a.Attrs&screen.AttrBold == 0 {
		t.Fatalf("a %+v", a)
	}
	if b.FG != screen.DefaultColor || b.Attrs != 0 {
		t.Fatalf("b %+v", b)
	}
}

func TestSGR256AndTruecolor(t *testing.T) {
	s := run(10, 1, "\x1b[38;5;208ma\x1b[48;2;1;2;3mb")
	if s.Cell(0, 0).FG != screen.Indexed(208) {
		t.Fatal("256")
	}
	if s.Cell(1, 0).BG != screen.RGB(1, 2, 3) {
		t.Fatal("truecolor")
	}
}

func TestSGRBrightColors(t *testing.T) {
	s := run(10, 1, "\x1b[92;101ma")
	if s.Cell(0, 0).FG != screen.Indexed(10) || s.Cell(0, 0).BG != screen.Indexed(9) {
		t.Fatal("bright")
	}
}

func TestSGRColonSubparamsDoNotCrash(t *testing.T) {
	run(10, 1, "\x1b[4:3m\x1b[58:2::1:2:3ma")
}

func TestSGRColonTruecolorWithColorspaceSlot(t *testing.T) {
	s := run(10, 1, "\x1b[38:2::1:2:3ma\x1b[48:2::4:5:6mb")
	if s.Cell(0, 0).FG != screen.RGB(1, 2, 3) || s.Cell(1, 0).BG != screen.RGB(4, 5, 6) {
		t.Fatalf("%+v %+v", s.Cell(0, 0), s.Cell(1, 0))
	}
}

func TestSGRBareAndEmptyResets(t *testing.T) {
	s := run(10, 1, "\x1b[1;3;4;7;9;2m\x1b[ma\x1b[1m\x1b[;mb")
	if s.Cell(0, 0).Attrs != 0 || s.Cell(1, 0).Attrs != 0 {
		t.Fatalf("%+v %+v", s.Cell(0, 0), s.Cell(1, 0))
	}
}

func TestSGRAttrOnOff(t *testing.T) {
	s := run(10, 1, "\x1b[1;2;3;4;7;9ma\x1b[22;23;24;27;29mb")
	want := screen.AttrBold | screen.AttrDim | screen.AttrItalic | screen.AttrUnderline | screen.AttrInverse | screen.AttrStrike
	if s.Cell(0, 0).Attrs != want || s.Cell(1, 0).Attrs != 0 {
		t.Fatalf("%v %v", s.Cell(0, 0).Attrs, s.Cell(1, 0).Attrs)
	}
}

func TestSGRDefaultsAndOutOfRange(t *testing.T) {
	s := run(10, 1, "\x1b[31;42m\x1b[39;49ma\x1b[38;5;300mb\x1b[38;2;1;2;999mc")
	a := s.Cell(0, 0)
	if a.FG != screen.DefaultColor || a.BG != screen.DefaultColor {
		t.Fatalf("%+v", a)
	}
	if s.Cell(1, 0).FG != screen.DefaultColor || s.Cell(2, 0).FG != screen.DefaultColor {
		t.Fatal("out of range applied")
	}
}

func TestUnderlineStyleColonDoesNotItalicize(t *testing.T) {
	c := run(10, 1, "\x1b[4:3ma").Cell(0, 0)
	if c.Attrs&screen.AttrUnderline == 0 || c.Attrs&screen.AttrItalic != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestUnderlineColonZeroTurnsOffOnlyUnderline(t *testing.T) {
	c := run(10, 1, "\x1b[1;4m\x1b[4:0ma").Cell(0, 0)
	if c.Attrs&screen.AttrBold == 0 || c.Attrs&screen.AttrUnderline != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestMixedGroups(t *testing.T) {
	c := run(10, 1, "\x1b[1;4:3;31ma").Cell(0, 0)
	if c.Attrs&screen.AttrBold == 0 || c.Attrs&screen.AttrUnderline == 0 || c.FG != screen.Indexed(1) {
		t.Fatalf("%+v", c)
	}
}

func TestColonTruecolorWithAndWithoutColorspace(t *testing.T) {
	if run(10, 1, "\x1b[38:2:0:1:2:3ma").Cell(0, 0).FG != screen.RGB(1, 2, 3) {
		t.Fatal("with cs")
	}
	if run(10, 1, "\x1b[38:2::1:2:3ma").Cell(0, 0).FG != screen.RGB(1, 2, 3) {
		t.Fatal("empty cs")
	}
	if run(10, 1, "\x1b[38:2:1:2:3ma").Cell(0, 0).FG != screen.RGB(1, 2, 3) {
		t.Fatal("no cs")
	}
	if run(10, 1, "\x1b[48:5:208ma").Cell(0, 0).BG != screen.Indexed(208) {
		t.Fatal("256 colon")
	}
}

func TestUnderlineColorGroupSkippedWhole(t *testing.T) {
	c := run(10, 1, "\x1b[58:2::1:2:3;1ma").Cell(0, 0)
	if c.Attrs != screen.AttrBold || c.FG != screen.DefaultColor {
		t.Fatalf("%+v", c)
	}
}

func TestSemicolonFormsUnchanged(t *testing.T) {
	if run(10, 1, "\x1b[38;2;1;2;3ma").Cell(0, 0).FG != screen.RGB(1, 2, 3) {
		t.Fatal("rgb")
	}
	if run(10, 1, "\x1b[38;5;208ma").Cell(0, 0).FG != screen.Indexed(208) {
		t.Fatal("256")
	}
}
