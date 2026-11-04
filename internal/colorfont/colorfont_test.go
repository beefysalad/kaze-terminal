package colorfont

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	cft "github.com/beefysalad/kaze-terminal/internal/colorfont/colorfonttest"
)

func sampleTables() map[string][]byte {
	return map[string][]byte{
		"maxp": cft.Maxp(20),
		"COLR": sampleCOLR(),
		"CPAL": sampleCPAL(),
	}
}

func sampleCOLR() []byte {
	return cft.COLR(
		[]cft.Base{{Glyph: 5, First: 0, Num: 2}, {Glyph: 9, First: 2, Num: 1}},
		[]cft.Layer{{Glyph: 10, Palette: 0}, {Glyph: 11, Palette: 1}, {Glyph: 12, Palette: 0xFFFF}},
	)
}

func sampleCPAL() []byte { return cft.CPAL([4]byte{0, 0, 0xFF, 0xFF}, [4]byte{0, 0xFF, 0, 0x80}) }

func checkDirectory(t *testing.T, font []byte, index int) {
	t.Helper()
	r := bytes.NewReader(font)
	d, err := ReadDirectory(r, int64(len(font)), index)
	if err != nil {
		t.Fatal(err)
	}
	for tag, want := range sampleTables() {
		got, err := d.Table(r, tag, 1<<10)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: got %x want %x", tag, got, want)
		}
	}
	if _, err := d.Table(r, "sbix", 1<<10); !errors.Is(err, ErrMissing) {
		t.Fatalf("absent table: err %v, want ErrMissing", err)
	}
	if _, err := d.Table(r, "COLR", 4); !errors.Is(err, ErrTooBig) {
		t.Fatalf("table over max: err %v, want ErrTooBig", err)
	}
	maxp, _ := d.Table(r, "maxp", 64)
	if n, err := NumGlyphs(maxp); err != nil || n != 20 {
		t.Fatalf("NumGlyphs = %d, %v; want 20", n, err)
	}
}

func TestReadDirectory(t *testing.T) {
	checkDirectory(t, cft.Sfnt(sampleTables()), 0)
	other := map[string][]byte{"maxp": cft.Maxp(3)}
	checkDirectory(t, cft.TTC(other, sampleTables()), 1)
}

func TestReadDirectoryRejects(t *testing.T) {
	good := cft.Sfnt(sampleTables())
	cases := map[string][]byte{
		"empty":     nil,
		"short":     good[:10],
		"truncated": good[:len(good)-8], // last table runs past the file
	}
	many := append([]byte(nil), good...)
	binary.BigEndian.PutUint16(many[4:], 0xFFFF)
	cases["too many tables"] = many
	for name, b := range cases {
		if _, err := ReadDirectory(bytes.NewReader(b), int64(len(b)), 0); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	ttc := cft.TTC(sampleTables())
	if _, err := ReadDirectory(bytes.NewReader(ttc), int64(len(ttc)), 1); err == nil {
		t.Error("font index past the collection: no error")
	}
}

func TestParseCOLR(t *testing.T) {
	c, err := ParseCOLR(sampleCOLR(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Layers(5); !reflect.DeepEqual(got, []Layer{{10, 0}, {11, 1}}) {
		t.Fatalf("Layers(5) = %v", got)
	}
	if got := c.Layers(9); !reflect.DeepEqual(got, []Layer{{12, TextColor}}) {
		t.Fatalf("Layers(9) = %v", got)
	}
	if got := c.Layers(6); got != nil {
		t.Fatalf("Layers(6) = %v, want nil", got)
	}
	if n := c.NumBase(); n != 2 {
		t.Fatalf("NumBase = %d, want 2", n)
	}
	if a := testing.AllocsPerRun(100, func() { c.Layers(5) }); a != 0 {
		t.Fatalf("Layers allocates %v times", a)
	}
}

func TestParseCPAL(t *testing.T) {
	p, err := ParseCPAL(sampleCPAL())
	if err != nil {
		t.Fatal(err)
	}
	want := Palette{{R: 255, A: 255}, {G: 255, A: 128}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("palette %v, want %v", p, want)
	}
	_ = color.NRGBA{}
}

func TestParseCOLRRejects(t *testing.T) {
	good := sampleCOLR()
	mut := func(f func(b []byte)) []byte {
		b := append([]byte(nil), good...)
		f(b)
		return b
	}
	cases := map[string][]byte{
		"shorter than header": good[:10],
		"base offset past end": mut(func(b []byte) {
			binary.BigEndian.PutUint32(b[2:], uint32(len(b)))
		}),
		"layer range past layer records": mut(func(b []byte) {
			binary.BigEndian.PutUint16(b[14+6+4:], 5) // base 9: numLayers 5
		}),
		"layer glyph past numGlyphs": mut(func(b []byte) {
			binary.BigEndian.PutUint16(b[14+12:], 20) // first layer glyph = numGlyphs
		}),
		"base gids not ascending": mut(func(b []byte) {
			binary.BigEndian.PutUint16(b[14:], 9) // both base records are gid 9
		}),
		"huge base count": mut(func(b []byte) {
			binary.BigEndian.PutUint16(b[2:], 0xFFFF)
		})[:14],
	}
	for name, b := range cases {
		c, err := ParseCOLR(b, 20)
		if !errors.Is(err, ErrMalformed) || c != nil {
			t.Errorf("%s: got %v, %v; want nil, ErrMalformed", name, c, err)
		}
	}
}

func TestParseCPALRejects(t *testing.T) {
	good := sampleCPAL()
	off := append([]byte(nil), good...)
	binary.BigEndian.PutUint32(off[8:], uint32(len(off)))
	entries := append([]byte(nil), good...)
	binary.BigEndian.PutUint16(entries[2:], 3) // 3 entries, 2 records
	for name, b := range map[string][]byte{"records past end": off, "entries past records": entries, "short": good[:6]} {
		if p, err := ParseCPAL(b); !errors.Is(err, ErrMalformed) || p != nil {
			t.Errorf("%s: got %v, %v; want nil, ErrMalformed", name, p, err)
		}
	}
}

func FuzzParseCOLR(f *testing.F) {
	f.Add(sampleCOLR())
	f.Add(sampleCOLR()[:14])
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := ParseCOLR(b, 64)
		if err != nil {
			return
		}
		for g := 0; g < 64; g++ {
			for _, l := range c.Layers(uint16(g)) {
				if l.Glyph >= 64 {
					t.Fatalf("layer glyph %d past numGlyphs", l.Glyph)
				}
			}
		}
	})
}

func FuzzParseCPAL(f *testing.F) {
	f.Add(sampleCPAL())
	f.Fuzz(func(t *testing.T, b []byte) { ParseCPAL(b) })
}

func TestSegoeEmojiCOLR(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Segoe UI Emoji is a Windows font")
	}
	p := filepath.Join(os.Getenv("WINDIR"), "Fonts", "seguiemj.ttf")
	f, err := os.Open(p)
	if err != nil {
		t.Skip("Segoe UI Emoji is not installed")
	}
	defer f.Close()
	st, _ := f.Stat()
	d, err := ReadDirectory(f, st.Size(), 0)
	if err != nil {
		t.Fatal(err)
	}
	maxp, err := d.Table(f, "maxp", 64)
	if err != nil {
		t.Fatal(err)
	}
	n, err := NumGlyphs(maxp)
	if err != nil {
		t.Fatal(err)
	}
	colr, err := d.Table(f, "COLR", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCOLR(colr, n)
	if err != nil {
		t.Fatal(err)
	}
	if c.NumBase() < 3000 {
		t.Fatalf("%d base records, want at least 3000", c.NumBase())
	}
	cpal, err := d.Table(f, "CPAL", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if pal, err := ParseCPAL(cpal); err != nil || len(pal) < 1 {
		t.Fatalf("palette %d colors, %v", len(pal), err)
	}
}
