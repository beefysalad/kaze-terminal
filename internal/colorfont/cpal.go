package colorfont

import "image/color"

// Palette is one CPAL palette.
type Palette []color.NRGBA

// ParseCPAL returns palette 0 of a CPAL table (stored as BGRA records).
func ParseCPAL(b []byte) (Palette, error) {
	entries, ok1 := u16(b, 2)
	palettes, ok2 := u16(b, 4)
	records, ok3 := u16(b, 6)
	off, ok4 := u32(b, 8)
	first, ok5 := u16(b, 12)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || palettes == 0 {
		return nil, ErrMalformed
	}
	if int(first)+int(entries) > int(records) || int64(off)+4*int64(records) > int64(len(b)) {
		return nil, ErrMalformed
	}
	p := make(Palette, entries)
	for i := range p {
		c := b[int(off)+4*(int(first)+i):]
		p[i] = color.NRGBA{R: c[2], G: c[1], B: c[0], A: c[3]}
	}
	return p, nil
}
