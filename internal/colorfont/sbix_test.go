package colorfont

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	"testing"

	cft "github.com/beefysalad/kaze-terminal/internal/colorfont/colorfonttest"
)

func TestParseCmapFormat12(t *testing.T) {
	c, err := ParseCmap(cft.Cmap12(
		cft.Group{Start: 0x2705, End: 0x2705, Glyph: 12},
		cft.Group{Start: 0x1F500, End: 0x1F510, Glyph: 73},
	))
	if err != nil {
		t.Fatal(err)
	}
	for r, want := range map[rune]uint16{0x1F504: 77, 0x2705: 12, 'A': 0, 0x1F511: 0, 0x10FFFF: 0} {
		if got := c.Glyph(r); got != want {
			t.Errorf("Glyph(%U) = %d, want %d", r, got, want)
		}
	}
}

func TestParseCmapFormat4(t *testing.T) {
	c, err := ParseCmap(cft.Cmap4(
		cft.Seg{Start: 'A', End: 'C', Delta: 1 - 'A'},
		cft.Seg{Start: 0x2705, End: 0x2706, Glyphs: []uint16{12, 0}},
	))
	if err != nil {
		t.Fatal(err)
	}
	for r, want := range map[rune]uint16{'A': 1, 'C': 3, 'D': 0, 0x2705: 12, 0x2706: 0, 0x1F504: 0} {
		if got := c.Glyph(r); got != want {
			t.Errorf("Glyph(%U) = %d, want %d", r, got, want)
		}
	}
}

func TestParseCmapRejects(t *testing.T) {
	short := cft.Cmap12(cft.Group{Start: 1, End: 2, Glyph: 3})
	binary.BigEndian.PutUint32(short[12+12:], 1000) // numGroups past the data
	for name, b := range map[string][]byte{"empty": nil, "groups past end": short, "no subtable": {0, 0, 0, 0}} {
		if _, err := ParseCmap(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// sbixFont is a synthetic font: maxp, cmap and an sbix with the given strikes.
func sbixFont(t *testing.T, numGlyphs int, strikes ...cft.Strike) (*bytes.Reader, Record) {
	t.Helper()
	b := cft.Sfnt(map[string][]byte{
		"maxp": cft.Maxp(numGlyphs),
		"sbix": cft.Sbix(numGlyphs, strikes...),
	})
	r := bytes.NewReader(b)
	d, err := ReadDirectory(r, int64(len(b)), 0)
	if err != nil {
		t.Fatal(err)
	}
	return r, d["sbix"]
}

func TestSbixNearestStrike(t *testing.T) {
	var strikes []cft.Strike
	for _, p := range []uint16{20, 32, 40, 64, 96, 160} {
		strikes = append(strikes, cft.Strike{PPEM: p})
	}
	r, rec := sbixFont(t, 4, strikes...)
	s, err := ParseSbix(r, rec, 4)
	if err != nil {
		t.Fatal(err)
	}
	for ask, want := range map[int]int{19: 20, 36: 40, 35: 32, 37: 40, 500: 160, 1: 20} {
		i, got := s.Nearest(ask)
		if got != want || strikes[i].PPEM != uint16(want) {
			t.Errorf("Nearest(%d) = strike %d ppem %d, want ppem %d", ask, i, got, want)
		}
	}
}

func TestSbixPNG(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	r, rec := sbixFont(t, 8, cft.Strike{PPEM: 32, Glyphs: map[uint16]cft.Glyph{
		3: {Type: "png ", Data: cft.PNG(4, 4, red)},
		4: {Type: "dupe", Data: cft.Dupe(3)},
	}})
	s, err := ParseSbix(r, rec, 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, gid := range []uint16{3, 4} {
		if !s.Has(r, 0, gid) {
			t.Fatalf("Has(%d) = false", gid)
		}
		data, err := s.PNG(r, 0, gid)
		if err != nil {
			t.Fatalf("PNG(%d): %v", gid, err)
		}
		img, err := DecodePNG(data, 64)
		if err != nil {
			t.Fatalf("DecodePNG(%d): %v", gid, err)
		}
		if b := img.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
			t.Fatalf("gid %d decodes to %v, want 4x4", gid, b)
		}
	}
	for _, gid := range []uint16{5, 8, 0xFFFF} {
		if s.Has(r, 0, gid) {
			t.Errorf("Has(%d) = true for an empty or absent glyph", gid)
		}
		if _, err := s.PNG(r, 0, gid); err == nil {
			t.Errorf("PNG(%d): no error", gid)
		}
	}
	if s.Has(r, 1, 3) {
		t.Error("Has on a strike that does not exist")
	}
}

func TestSbixPNGTooLarge(t *testing.T) {
	if _, err := DecodePNG(cft.PNGHeader(60000, 60000), 256); !errors.Is(err, ErrTooBig) {
		t.Fatalf("60000x60000 header: %v, want ErrTooBig", err)
	}
	if a := testing.AllocsPerRun(5, func() { DecodePNG(cft.PNGHeader(60000, 60000), 256) }); a > 50 {
		t.Fatalf("refusing a huge PNG made %v allocations", a)
	}
	// A glyph whose offsets claim 2 GiB of data.
	tbl := cft.Sbix(4, cft.Strike{PPEM: 32, Glyphs: map[uint16]cft.Glyph{1: {Type: "png ", Data: cft.PNG(2, 2, color.NRGBA{A: 255})}}})
	so := binary.BigEndian.Uint32(tbl[8:])
	binary.BigEndian.PutUint32(tbl[so+4+4*2:], 1<<31) // offset of glyph 2 = end of glyph 1
	r := bytes.NewReader(tbl)
	s, err := ParseSbix(r, Record{0, uint32(len(tbl))}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PNG(r, 0, 1); !errors.Is(err, ErrTooBig) {
		t.Fatalf("2 GiB glyph: %v, want ErrTooBig", err)
	}
}

func TestSbixRejects(t *testing.T) {
	good := cft.Sbix(4, cft.Strike{PPEM: 32, Glyphs: map[uint16]cft.Glyph{
		1: {Type: "png ", Data: cft.PNG(2, 2, color.NRGBA{A: 255})},
		2: {Type: "dupe", Data: cft.Dupe(3)},
		3: {Type: "dupe", Data: cft.Dupe(2)},
	}})
	parse := func(b []byte) (*Sbix, *bytes.Reader, error) {
		r := bytes.NewReader(b)
		s, err := ParseSbix(r, Record{0, uint32(len(b))}, 4)
		return s, r, err
	}

	past := append([]byte(nil), good...)
	binary.BigEndian.PutUint32(past[8:], uint32(len(past)))
	if _, _, err := parse(past); err == nil {
		t.Error("strike offset past the table: no error")
	}
	many := append([]byte(nil), good...)
	binary.BigEndian.PutUint32(many[4:], 1<<20)
	if _, _, err := parse(many); err == nil {
		t.Error("a million strikes: no error")
	}

	desc := append([]byte(nil), good...)
	so := binary.BigEndian.Uint32(desc[8:])
	binary.BigEndian.PutUint32(desc[so+4+4*1:], binary.BigEndian.Uint32(desc[so+4+4*2:])+1) // glyph 1 starts after it ends
	s, r, err := parse(desc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PNG(r, 0, 1); !errors.Is(err, ErrMalformed) || s.Has(r, 0, 1) {
		t.Errorf("descending offsets: %v, want ErrMalformed and Has false", err)
	}

	s, r, err = parse(good)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PNG(r, 0, 2); !errors.Is(err, ErrMalformed) || s.Has(r, 0, 2) {
		t.Errorf("dupe cycle: %v, want ErrMalformed and Has false", err)
	}
	if _, err := s.PNG(r, 0, 1); err != nil {
		t.Errorf("good glyph next to the cycle: %v", err)
	}
}

func FuzzParseSbix(f *testing.F) {
	f.Add(cft.Sbix(4, cft.Strike{PPEM: 32, Glyphs: map[uint16]cft.Glyph{
		1: {Type: "png ", Data: cft.PNG(2, 2, color.NRGBA{A: 255})},
		2: {Type: "dupe", Data: cft.Dupe(1)},
	}}))
	f.Fuzz(func(t *testing.T, b []byte) {
		r := bytes.NewReader(b)
		s, err := ParseSbix(r, Record{0, uint32(len(b))}, 4)
		if err != nil {
			return
		}
		for st := 0; st < 3; st++ {
			for g := uint16(0); g < 6; g++ {
				s.Has(r, st, g)
				if data, err := s.PNG(r, st, g); err == nil {
					DecodePNG(data, 64)
				}
			}
		}
	})
}

func FuzzParseCmap(f *testing.F) {
	f.Add(cft.Cmap12(cft.Group{Start: 0x2705, End: 0x2705, Glyph: 12}))
	f.Add(cft.Cmap4(cft.Seg{Start: 0x2705, End: 0x2706, Glyphs: []uint16{12, 0}}))
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := ParseCmap(b)
		if err != nil {
			return
		}
		for _, r := range []rune{0, 'A', 0x2705, 0x2706, 0xFFFF, 0x1F504, 0x10FFFF} {
			c.Glyph(r)
		}
	})
}
