package discord

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	opHandshake uint32 = 0
	opFrame     uint32 = 1
	opClose     uint32 = 2
	opPing      uint32 = 3
	opPong      uint32 = 4
)

const maxFrame = 64 << 10

var errFrameTooBig = errors.New("discord: frame too big")

func writeFrame(w io.Writer, op uint32, payload []byte) error {
	if len(payload) > maxFrame {
		return errFrameTooBig
	}
	var header [8]byte
	binary.LittleEndian.PutUint32(header[:4], op)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(payload)))
	for _, part := range [][]byte{header[:], payload} {
		for len(part) > 0 {
			n, err := w.Write(part)
			if err != nil {
				return fmt.Errorf("discord: write frame: %w", err)
			}
			if n <= 0 || n > len(part) {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}

func readFrame(r io.Reader) (op uint32, payload []byte, err error) {
	var header [8]byte
	if _, err = io.ReadFull(r, header[:]); err != nil {
		return 0, nil, fmt.Errorf("discord: read header: %w", err)
	}
	op = binary.LittleEndian.Uint32(header[:4])
	size := binary.LittleEndian.Uint32(header[4:])
	if size > maxFrame {
		return op, nil, errFrameTooBig
	}
	payload = make([]byte, int(size))
	if _, err = io.ReadFull(r, payload); err != nil {
		return op, nil, fmt.Errorf("discord: read payload: %w", err)
	}
	return op, payload, nil
}
