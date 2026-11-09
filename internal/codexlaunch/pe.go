package codexlaunch

import (
	"encoding/binary"
	"errors"
	"slices"
)

// ConsoleImage makes a private Windows launcher's subsystem console so its
// calling shell waits and inherited stdio behaves like a CLI. The installed
// Kaze image is never edited. Signed images are rejected instead of invalidated.
func ConsoleImage(src []byte) ([]byte, error) {
	bad := errors.New("kaze: cannot create Codex launcher from invalid or signed Windows image")
	if len(src) < 64 || string(src[:2]) != "MZ" {
		return nil, bad
	}
	off := uint64(binary.LittleEndian.Uint32(src[0x3c:]))
	if off > uint64(len(src)) || uint64(len(src))-off < 24 {
		return nil, bad
	}
	p := int(off)
	if string(src[p:p+4]) != "PE\x00\x00" {
		return nil, bad
	}
	machine := binary.LittleEndian.Uint16(src[p+4:])
	size := int(binary.LittleEndian.Uint16(src[p+20:]))
	o := p + 24
	if size < 70 || o > len(src)-size {
		return nil, bad
	}
	magic := binary.LittleEndian.Uint16(src[o:])
	dirs := 0
	count := 0
	switch magic {
	case 0x10b:
		if machine != 0x14c || size < 96 {
			return nil, bad
		}
		dirs, count = 96, 92
	case 0x20b:
		if (machine != 0x8664 && machine != 0xaa64) || size < 112 {
			return nil, bad
		}
		dirs, count = 112, 108
	default:
		return nil, bad
	}
	n := uint64(binary.LittleEndian.Uint32(src[o+count:]))
	if n > 16 || uint64(dirs)+n*8 > uint64(size) {
		return nil, bad
	}
	if n > 4 {
		cert := src[o+dirs+4*8 : o+dirs+5*8]
		if binary.LittleEndian.Uint32(cert[:4]) != 0 || binary.LittleEndian.Uint32(cert[4:]) != 0 {
			return nil, bad
		}
	}
	sub := binary.LittleEndian.Uint16(src[o+68:])
	if sub != 2 && sub != 3 {
		return nil, bad
	}
	out := slices.Clone(src)
	binary.LittleEndian.PutUint16(out[o+68:], 3)
	return out, nil
}
