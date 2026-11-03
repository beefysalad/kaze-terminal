package colorfont

import (
	"bytes"
	"image"
	"image/png"
	"io"
)

// maxStrikes bounds the strike list; Apple Color Emoji has about ten.
const maxStrikes = 32

// maxGlyphData bounds one glyph's PNG; Apple Color Emoji's largest are under 100 KiB.
const maxGlyphData = 512 << 10

type strike struct {
	off  uint32 // from the start of the table
	ppem uint16
}

// Sbix locates the strikes of an sbix table. Glyph records are read on demand.
type Sbix struct {
	rec       Record
	numGlyphs int
	strikes   []strike
}

// ParseSbix reads the sbix header, its strike offsets and each strike's ppem.
// Every strike's glyph offset array must lie inside the table.
func ParseSbix(r io.ReaderAt, rec Record, numGlyphs int) (*Sbix, error) {
	size := int64(rec.Offset) + int64(rec.Length)
	head, err := readAt(r, size, int64(rec.Offset), 8)
	if err != nil {
		return nil, err
	}
	n, _ := u32(head, 4)
	if n > maxStrikes || numGlyphs <= 0 || numGlyphs > 0xFFFF {
		return nil, ErrMalformed
	}
	offs, err := readAt(r, size, int64(rec.Offset)+8, 4*int(n))
	if err != nil {
		return nil, err
	}
	s := &Sbix{rec: rec, numGlyphs: numGlyphs, strikes: make([]strike, n)}
	for i := range s.strikes {
		o, _ := u32(offs, 4*i)
		if int64(o)+4+4*int64(numGlyphs+1) > int64(rec.Length) {
			return nil, ErrMalformed
		}
		pp, err := readAt(r, size, int64(rec.Offset)+int64(o), 2)
		if err != nil {
			return nil, err
		}
		ppem, _ := u16(pp, 0)
		s.strikes[i] = strike{o, ppem}
	}
	return s, nil
}

// Nearest returns the strike whose ppem is closest to ppem (ties go to the larger)
// and its ppem; (-1, 0) when there are no strikes.
func (s *Sbix) Nearest(ppem int) (int, int) {
	best, bestPPEM := -1, 0
	for i, st := range s.strikes {
		p := int(st.ppem)
		if best < 0 {
			best, bestPPEM = i, p
			continue
		}
		d, bd := abs(p-ppem), abs(bestPPEM-ppem)
		if d < bd || (d == bd && p > bestPPEM) {
			best, bestPPEM = i, p
		}
	}
	return best, bestPPEM
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// record returns the absolute offset and length of gid's record in strike st.
func (s *Sbix) record(r io.ReaderAt, st int, gid uint16) (int64, int64, error) {
	if st < 0 || st >= len(s.strikes) || int(gid) >= s.numGlyphs {
		return 0, 0, ErrMissing
	}
	so := int64(s.strikes[st].off)
	size := int64(s.rec.Offset) + int64(s.rec.Length)
	b, err := readAt(r, size, int64(s.rec.Offset)+so+4+4*int64(gid), 8)
	if err != nil {
		return 0, 0, err
	}
	start, _ := u32(b, 0)
	end, _ := u32(b, 4)
	if end < start {
		return 0, 0, ErrMalformed
	}
	n := int64(end) - int64(start)
	if n == 0 {
		return 0, 0, ErrMissing
	}
	if n > maxGlyphData+8 {
		return 0, 0, ErrTooBig
	}
	if n < 8 || so+int64(end) > int64(s.rec.Length) {
		return 0, 0, ErrMalformed
	}
	return int64(s.rec.Offset) + so + int64(start), n, nil
}

// locate returns the absolute offset and length of gid's PNG data in strike st,
// following at most one dupe record.
func (s *Sbix) locate(r io.ReaderAt, st int, gid uint16) (int64, int64, error) {
	size := int64(s.rec.Offset) + int64(s.rec.Length)
	for hop := 0; hop < 2; hop++ {
		off, n, err := s.record(r, st, gid)
		if err != nil {
			return 0, 0, err
		}
		head, err := readAt(r, size, off, 10)
		if err != nil {
			return 0, 0, err
		}
		switch string(head[4:8]) {
		case "png ":
			return off + 8, n - 8, nil
		case "dupe":
			if n < 10 {
				return 0, 0, ErrMalformed
			}
			gid, _ = u16(head, 8)
		default:
			return 0, 0, ErrMalformed
		}
	}
	return 0, 0, ErrMalformed // a dupe of a dupe, or a cycle
}

// Has reports whether gid has a PNG record (directly or through one dupe) in strike st.
func (s *Sbix) Has(r io.ReaderAt, st int, gid uint16) bool {
	_, _, err := s.locate(r, st, gid)
	return err == nil
}

// PNG returns gid's PNG data in strike st.
func (s *Sbix) PNG(r io.ReaderAt, st int, gid uint16) ([]byte, error) {
	off, n, err := s.locate(r, st, gid)
	if err != nil {
		return nil, err
	}
	return readAt(r, int64(s.rec.Offset)+int64(s.rec.Length), off, int(n))
}

// DecodePNG decodes data after checking, from its header alone, that neither side
// exceeds maxSide, so a hostile header cannot make it allocate a huge image.
func DecodePNG(data []byte, maxSide int) (image.Image, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrMalformed
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSide || cfg.Height > maxSide {
		return nil, ErrTooBig
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrMalformed
	}
	return img, nil
}
