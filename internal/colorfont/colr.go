package colorfont

// TextColor is the palette index meaning "the text colour".
const TextColor = 0xFFFF

// Layer is one COLR v0 layer: a glyph outline drawn in a palette colour.
type Layer struct{ Glyph, Palette uint16 }

type baseRecord struct{ glyph, first, num uint16 }

// COLR holds the v0 base glyph and layer records of a COLR table.
type COLR struct {
	bases  []baseRecord // ascending by glyph
	layers []Layer
}

// ParseCOLR reads the version 0 part of a COLR table (also present in v1 tables).
// Every base's layers must lie inside the layer records, every layer glyph must be
// below numGlyphs and base glyphs must ascend; otherwise it returns ErrMalformed.
func ParseCOLR(b []byte, numGlyphs int) (*COLR, error) {
	nb, ok1 := u16(b, 2)
	baseOff, ok2 := u32(b, 4)
	layerOff, ok3 := u32(b, 8)
	nl, ok4 := u16(b, 12)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, ErrMalformed
	}
	if int64(baseOff)+6*int64(nb) > int64(len(b)) || int64(layerOff)+4*int64(nl) > int64(len(b)) {
		return nil, ErrMalformed
	}
	c := &COLR{bases: make([]baseRecord, nb), layers: make([]Layer, nl)}
	for i := range c.layers {
		o := int(layerOff) + 4*i
		g, _ := u16(b, o)
		p, _ := u16(b, o+2)
		if int(g) >= numGlyphs {
			return nil, ErrMalformed
		}
		c.layers[i] = Layer{g, p}
	}
	for i := range c.bases {
		o := int(baseOff) + 6*i
		g, _ := u16(b, o)
		first, _ := u16(b, o+2)
		num, _ := u16(b, o+4)
		if int(first)+int(num) > int(nl) || (i > 0 && g <= c.bases[i-1].glyph) {
			return nil, ErrMalformed
		}
		c.bases[i] = baseRecord{g, first, num}
	}
	return c, nil
}

// NumBase is the number of base glyph records.
func (c *COLR) NumBase() int { return len(c.bases) }

// Layers returns gid's layers, bottom first, as a sub-slice (no allocation); nil when
// gid has no colour record.
func (c *COLR) Layers(gid uint16) []Layer {
	lo, hi := 0, len(c.bases)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if c.bases[m].glyph < gid {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo == len(c.bases) || c.bases[lo].glyph != gid || c.bases[lo].num == 0 {
		return nil
	}
	r := c.bases[lo]
	return c.layers[r.first : int(r.first)+int(r.num) : int(r.first)+int(r.num)]
}
