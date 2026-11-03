package colorfont

// Cmap maps runes to glyph ids through one format 4 or format 12 subtable.
// It is for fonts that golang.org/x/image/font/sfnt cannot parse (an sbix-only
// font such as Apple Color Emoji has no glyf or CFF outlines).
type Cmap struct {
	format int
	sub    []byte // the chosen subtable
	groups int    // format 12 group count
	segs   int    // format 4 segment count
}

// maxGroups bounds a format 12 subtable; Apple Color Emoji has a few thousand.
const maxGroups = 1 << 16

// ParseCmap picks the best Unicode subtable: platform 3 encoding 10, then 0/4,
// then 3/1, then 0/3. Only formats 4 and 12 are read.
func ParseCmap(b []byte) (Cmap, error) {
	n, ok := u16(b, 2)
	if !ok {
		return Cmap{}, ErrMalformed
	}
	prefs := [][2]uint16{{3, 10}, {0, 4}, {3, 1}, {0, 3}}
	best, bestOff := len(prefs), uint32(0)
	for i := 0; i < int(n); i++ {
		p, ok1 := u16(b, 4+8*i)
		e, ok2 := u16(b, 6+8*i)
		off, ok3 := u32(b, 8+8*i)
		if !ok1 || !ok2 || !ok3 {
			return Cmap{}, ErrMalformed
		}
		for rank, pe := range prefs[:best] {
			if pe == [2]uint16{p, e} {
				best, bestOff = rank, off
				break
			}
		}
	}
	if best == len(prefs) || int64(bestOff) >= int64(len(b)) {
		return Cmap{}, ErrMalformed
	}
	sub := b[bestOff:]
	format, _ := u16(sub, 0)
	switch format {
	case 12:
		groups, ok := u32(sub, 12)
		if !ok || groups > maxGroups || 16+12*int64(groups) > int64(len(sub)) {
			return Cmap{}, ErrMalformed
		}
		return Cmap{format: 12, sub: sub[:16+12*groups], groups: int(groups)}, nil
	case 4:
		x2, ok := u16(sub, 6)
		segs := int(x2 / 2)
		if !ok || segs == 0 || 16+8*segs > len(sub) {
			return Cmap{}, ErrMalformed
		}
		return Cmap{format: 4, sub: sub, segs: segs}, nil
	}
	return Cmap{}, ErrMalformed
}

// Glyph returns r's glyph id, 0 when unmapped.
func (c Cmap) Glyph(r rune) uint16 {
	switch c.format {
	case 12:
		lo, hi := 0, c.groups
		for lo < hi {
			m := int(uint(lo+hi) >> 1)
			end, _ := u32(c.sub, 16+12*m+4)
			if end < uint32(r) {
				lo = m + 1
			} else {
				hi = m
			}
		}
		if lo == c.groups || r < 0 {
			return 0
		}
		o := 16 + 12*lo
		start, _ := u32(c.sub, o)
		g, _ := u32(c.sub, o+8)
		if uint32(r) < start {
			return 0
		}
		g += uint32(r) - start
		if g > 0xFFFF {
			return 0
		}
		return uint16(g)
	case 4:
		if r < 0 || r > 0xFFFF {
			return 0
		}
		code := uint16(r)
		n := c.segs
		for i := 0; i < n; i++ {
			end, _ := u16(c.sub, 14+2*i)
			if end < code {
				continue
			}
			start, _ := u16(c.sub, 16+2*n+2*i)
			if start > code {
				return 0
			}
			delta, _ := u16(c.sub, 16+4*n+2*i)
			rangePos := 16 + 6*n + 2*i
			ro, _ := u16(c.sub, rangePos)
			if ro == 0 {
				return code + delta
			}
			g, ok := u16(c.sub, rangePos+int(ro)+2*int(code-start))
			if !ok || g == 0 {
				return 0
			}
			return g + delta
		}
	}
	return 0
}
