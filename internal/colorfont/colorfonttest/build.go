// Package colorfonttest builds small synthetic font tables for tests of
// colorfont and its users. It is imported by tests only.
package colorfonttest

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"sort"
)

func put16(b []byte, v uint16) []byte { return binary.BigEndian.AppendUint16(b, v) }
func put32(b []byte, v uint32) []byte { return binary.BigEndian.AppendUint32(b, v) }

// Sfnt builds a plain sfnt holding tables (tags sorted, no checksums).
func Sfnt(tables map[string][]byte) []byte { return sfntAt(tables, 0) }

// sfntAt builds an sfnt whose table offsets are relative to a file in which it starts at base.
func sfntAt(tables map[string][]byte, base int) []byte {
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	b := put32(nil, 0x00010000)
	b = put16(b, uint16(len(tags)))
	b = put16(b, 0)
	b = put16(b, 0)
	b = put16(b, 0)
	off := base + 12 + 16*len(tags)
	var data []byte
	for _, t := range tags {
		b = append(b, t...)
		b = put32(b, 0)
		b = put32(b, uint32(off+len(data)))
		b = put32(b, uint32(len(tables[t])))
		data = append(data, tables[t]...)
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
	}
	return append(b, data...)
}

// TTC wraps fonts (each given as its table map) in a ttcf collection.
func TTC(fonts ...map[string][]byte) []byte {
	head := 12 + 4*len(fonts)
	b := append([]byte("ttcf"), 0, 1, 0, 0)
	b = put32(b, uint32(len(fonts)))
	var body []byte
	offs := make([]int, len(fonts))
	for i, f := range fonts {
		offs[i] = head + len(body)
		body = append(body, sfntAt(f, offs[i])...)
	}
	for _, o := range offs {
		b = put32(b, uint32(o))
	}
	return append(b, body...)
}

// Maxp builds a version 0.5 maxp table.
func Maxp(numGlyphs int) []byte { return put16(put32(nil, 0x00005000), uint16(numGlyphs)) }

// Base is one COLR base glyph record.
type Base struct{ Glyph, First, Num uint16 }

// Layer is one COLR layer record.
type Layer struct{ Glyph, Palette uint16 }

// COLR builds a version 0 COLR table: header, base records, then layer records.
func COLR(bases []Base, layers []Layer) []byte {
	b := put16(nil, 0)
	b = put16(b, uint16(len(bases)))
	b = put32(b, 14)
	b = put32(b, uint32(14+6*len(bases)))
	b = put16(b, uint16(len(layers)))
	for _, r := range bases {
		b = put16(put16(put16(b, r.Glyph), r.First), r.Num)
	}
	for _, l := range layers {
		b = put16(put16(b, l.Glyph), l.Palette)
	}
	return b
}

// CPAL builds a version 0 CPAL table with one palette of BGRA colors.
func CPAL(bgra ...[4]byte) []byte {
	b := put16(nil, 0)
	b = put16(b, uint16(len(bgra))) // numPaletteEntries
	b = put16(b, 1)                 // numPalettes
	b = put16(b, uint16(len(bgra))) // numColorRecords
	b = put32(b, 14)                // colorRecordsArrayOffset
	b = put16(b, 0)                 // colorRecordIndices[0]
	for _, c := range bgra {
		b = append(b, c[:]...)
	}
	return b
}

// Group maps runes Start..End to glyphs from Glyph on (cmap format 12).
type Group struct {
	Start, End rune
	Glyph      uint32
}

// Cmap12 builds a cmap table with one platform 3 encoding 10 format 12 subtable.
func Cmap12(groups ...Group) []byte {
	b := put16(nil, 0)
	b = put16(b, 1)
	b = put16(put16(b, 3), 10)
	b = put32(b, 12)
	b = put16(put16(b, 12), 0)
	b = put32(b, uint32(16+12*len(groups)))
	b = put32(b, 0)
	b = put32(b, uint32(len(groups)))
	for _, g := range groups {
		b = put32(put32(put32(b, uint32(g.Start)), uint32(g.End)), g.Glyph)
	}
	return b
}

// Seg is one format 4 segment. With Glyphs nil it maps by Delta; otherwise Glyphs
// lists the glyph of each code in Start..End (through idRangeOffset).
type Seg struct {
	Start, End uint16
	Delta      int16
	Glyphs     []uint16
}

// Cmap4 builds a cmap table with one platform 3 encoding 1 format 4 subtable; the
// 0xFFFF terminator segment is added.
func Cmap4(segs ...Seg) []byte {
	segs = append(segs, Seg{Start: 0xFFFF, End: 0xFFFF, Delta: 1})
	n := len(segs)
	var glyphs []uint16
	ranges := make([]uint16, n)
	for i, s := range segs {
		if s.Glyphs != nil {
			// offset from ranges[i] to glyphs[len(glyphs)]
			ranges[i] = uint16(2*(n-i) + 2*len(glyphs))
			glyphs = append(glyphs, s.Glyphs...)
		}
	}
	sub := put16(nil, 4)
	sub = put16(sub, uint16(16+8*n+2*len(glyphs)))
	sub = put16(sub, 0)
	sub = put16(sub, uint16(2*n))
	sub = put16(put16(put16(sub, 0), 0), 0)
	for _, s := range segs {
		sub = put16(sub, s.End)
	}
	sub = put16(sub, 0)
	for _, s := range segs {
		sub = put16(sub, s.Start)
	}
	for _, s := range segs {
		sub = put16(sub, uint16(s.Delta))
	}
	for _, r := range ranges {
		sub = put16(sub, r)
	}
	for _, g := range glyphs {
		sub = put16(sub, g)
	}
	b := put16(nil, 0)
	b = put16(b, 1)
	b = put16(put16(b, 3), 1)
	b = put32(b, 12)
	return append(b, sub...)
}

// Glyph is one sbix glyph record: Type "png " with PNG data, or "dupe" with a 2-byte gid.
type Glyph struct {
	Type string
	Data []byte
}

// Strike is one sbix strike.
type Strike struct {
	PPEM   uint16
	Glyphs map[uint16]Glyph
}

// Sbix builds an sbix table for numGlyphs glyphs.
func Sbix(numGlyphs int, strikes ...Strike) []byte {
	b := put16(nil, 1)
	b = put16(b, 1)
	b = put32(b, uint32(len(strikes)))
	var body []byte
	head := 8 + 4*len(strikes)
	for _, s := range strikes {
		b = put32(b, uint32(head+len(body)))
		st := put16(nil, s.PPEM)
		st = put16(st, 72)
		var data []byte
		base := 4 + 4*(numGlyphs+1)
		for g := 0; g <= numGlyphs; g++ {
			st = put32(st, uint32(base+len(data)))
			if gl, ok := s.Glyphs[uint16(g)]; ok && g < numGlyphs {
				data = put16(put16(data, 0), 0)
				data = append(data, gl.Type...)
				data = append(data, gl.Data...)
			}
		}
		body = append(body, append(st, data...)...)
	}
	return append(b, body...)
}

// Dupe is the data of a dupe record pointing at gid.
func Dupe(gid uint16) []byte { return put16(nil, gid) }

// PNG encodes a w×h image filled with c.
func PNG(w, h int, c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// PNGHeader is a PNG signature and IHDR chunk claiming w×h, with no pixel data.
func PNGHeader(w, h uint32) []byte {
	b := []byte("\x89PNG\r\n\x1a\n")
	b = put32(b, 13)
	chunk := append([]byte("IHDR"), 0, 0, 0, 0, 0, 0, 0, 0, 8, 6, 0, 0, 0)
	binary.BigEndian.PutUint32(chunk[4:], w)
	binary.BigEndian.PutUint32(chunk[8:], h)
	b = append(b, chunk...)
	return put32(b, crc32.ChecksumIEEE(chunk))
}
