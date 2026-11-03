// Package colorfont reads the colour tables of an OpenType font: COLR v0 and CPAL
// (Segoe UI Emoji), sbix (Apple Color Emoji) and the cmap it needs for them.
//
// Every read is bounds-checked. A malformed table returns an error, never a panic,
// so a broken font costs colour, not the terminal.
package colorfont

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrMissing   = errors.New("colorfont: table not present")
	ErrTooBig    = errors.New("colorfont: table or image too big")
	ErrMalformed = errors.New("colorfont: malformed table")
)

// maxTables bounds the table directory; real fonts have about 20 tables.
const maxTables = 64

// maxFonts bounds a collection's font count.
const maxFonts = 256

// Record locates one table in the font file.
type Record struct{ Offset, Length uint32 }

// Directory maps table tags to their records.
type Directory map[string]Record

// u16 and u32 read big-endian values, reporting false instead of indexing out of range.
func u16(b []byte, off int) (uint16, bool) {
	if off < 0 || off+2 > len(b) {
		return 0, false
	}
	return binary.BigEndian.Uint16(b[off:]), true
}

func u32(b []byte, off int) (uint32, bool) {
	if off < 0 || off+4 > len(b) {
		return 0, false
	}
	return binary.BigEndian.Uint32(b[off:]), true
}

// readAt reads n bytes at off, failing unless they all lie inside size.
func readAt(r io.ReaderAt, size, off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || off > size || int64(n) > size-off {
		return nil, ErrMalformed
	}
	b := make([]byte, n)
	if _, err := r.ReadAt(b, off); err != nil {
		return nil, ErrMalformed
	}
	return b, nil
}

// ReadDirectory reads the table directory of a plain sfnt, or of font index font in a
// ttcf collection. Every table must lie inside size.
func ReadDirectory(r io.ReaderAt, size int64, font int) (Directory, error) {
	head, err := readAt(r, size, 0, 12)
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if string(head[:4]) == "ttcf" {
		n, _ := u32(head, 8)
		if font < 0 || n > maxFonts || uint32(font) >= n {
			return nil, ErrMalformed
		}
		o, err := readAt(r, size, 12+4*int64(font), 4)
		if err != nil {
			return nil, err
		}
		v, _ := u32(o, 0)
		start = int64(v)
		if head, err = readAt(r, size, start, 12); err != nil {
			return nil, err
		}
	} else if font != 0 {
		return nil, ErrMalformed
	}
	n, _ := u16(head, 4)
	if n > maxTables {
		return nil, ErrMalformed
	}
	recs, err := readAt(r, size, start+12, 16*int(n))
	if err != nil {
		return nil, err
	}
	d := make(Directory, n)
	for i := 0; i < int(n); i++ {
		b := recs[16*i:]
		off, _ := u32(b, 8)
		length, _ := u32(b, 12)
		if int64(off)+int64(length) > size {
			return nil, ErrMalformed
		}
		tag := string(b[:4])
		if _, dup := d[tag]; !dup {
			d[tag] = Record{off, length}
		}
	}
	return d, nil
}

// Table reads one table, ErrMissing when absent and ErrTooBig when longer than max.
func (d Directory) Table(r io.ReaderAt, tag string, max int) ([]byte, error) {
	rec, ok := d[tag]
	if !ok {
		return nil, ErrMissing
	}
	if int64(rec.Length) > int64(max) {
		return nil, ErrTooBig
	}
	b := make([]byte, rec.Length)
	if _, err := r.ReadAt(b, int64(rec.Offset)); err != nil {
		return nil, ErrMalformed
	}
	return b, nil
}

// NumGlyphs reads the glyph count from a maxp table.
func NumGlyphs(maxp []byte) (int, error) {
	n, ok := u16(maxp, 4)
	if !ok {
		return 0, ErrMalformed
	}
	return int(n), nil
}
