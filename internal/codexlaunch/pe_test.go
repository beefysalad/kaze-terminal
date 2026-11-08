package codexlaunch

import (
	"encoding/binary"
	"slices"
	"testing"
)

func fakePE(magic uint16) []byte {
	b := make([]byte, 512)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[0x3c:], 64)
	copy(b[64:], "PE\x00\x00")
	machine := uint16(0x8664)
	size := uint16(240)
	if magic == 0x10b {
		machine = 0x14c
		size = 224
	}
	binary.LittleEndian.PutUint16(b[68:], machine)
	binary.LittleEndian.PutUint16(b[84:], size)
	binary.LittleEndian.PutUint16(b[88:], magic)
	binary.LittleEndian.PutUint16(b[88+68:], 2)
	count := 88 + 108
	if magic == 0x10b {
		count = 88 + 92
	}
	binary.LittleEndian.PutUint32(b[count:], 16)
	return b
}

func TestConsoleImageChangesOnlySubsystemCopy(t *testing.T) {
	for _, magic := range []uint16{0x10b, 0x20b} {
		b := fakePE(magic)
		before := slices.Clone(b)
		got, err := ConsoleImage(b)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(before, b) {
			t.Fatal("original image mutated")
		}
		want := slices.Clone(b)
		binary.LittleEndian.PutUint16(want[156:], 3)
		if !slices.Equal(got, want) {
			t.Fatal("changed more than subsystem")
		}
		again, err := ConsoleImage(got)
		if err != nil || !slices.Equal(again, got) {
			t.Fatalf("console not idempotent %v", err)
		}
	}
}

func TestConsoleImageRejectsInvalidOrSigned(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"empty", func(b []byte) []byte { return nil }}, {"truncated", func(b []byte) []byte { return b[:100] }},
		{"bad offset", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[0x3c:], 0xfffffff0); return b }},
		{"bad machine", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[68:], 1); return b }},
		{"bad magic", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[88:], 1); return b }},
		{"bad subsystem", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[156:], 10); return b }},
		{"signed", func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[88+112+4*8:], 400)
			binary.LittleEndian.PutUint32(b[88+112+4*8+4:], 20)
			return b
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ConsoleImage(tc.mutate(fakePE(0x20b))); err == nil {
				t.Fatal("accepted invalid/signed image")
			}
		})
	}
}
