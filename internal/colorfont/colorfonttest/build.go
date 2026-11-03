// Package colorfonttest builds small synthetic font tables for tests of
// colorfont and its users. It is imported by tests only.
package colorfonttest

import (
	"encoding/binary"
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
