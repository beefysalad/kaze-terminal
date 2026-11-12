package discord

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var b bytes.Buffer
	if err := writeFrame(&b, opFrame, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes()[:8], []byte{1, 0, 0, 0, 7, 0, 0, 0}) {
		t.Fatalf("header: %v", b.Bytes()[:8])
	}
	op, payload, err := readFrame(&b)
	if err != nil || op != opFrame || string(payload) != `{"x":1}` {
		t.Fatalf("got %d %s %v", op, payload, err)
	}
}

type headerOnlyReader struct {
	t      *testing.T
	header []byte
}

func (r *headerOnlyReader) Read(p []byte) (int, error) {
	if len(r.header) == 0 {
		r.t.Fatal("oversized frame read its body")
	}
	n := copy(p, r.header)
	r.header = r.header[n:]
	return n, nil
}
func TestFrameRejectsTooBig(t *testing.T) {
	h := make([]byte, 8)
	binary.LittleEndian.PutUint32(h[4:], maxFrame+1)
	_, _, err := readFrame(&headerOnlyReader{t: t, header: h})
	if !errors.Is(err, errFrameTooBig) {
		t.Fatalf("got %v", err)
	}
	var b bytes.Buffer
	if err := writeFrame(&b, opFrame, make([]byte, maxFrame+1)); !errors.Is(err, errFrameTooBig) || b.Len() != 0 {
		t.Fatalf("write: %v, %d bytes", err, b.Len())
	}
}
func TestFrameShortRead(t *testing.T) {
	h := make([]byte, 8)
	binary.LittleEndian.PutUint32(h[4:], 10)
	_, _, err := readFrame(bytes.NewReader(append(h, []byte("abc")...)))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
}

type shortFrameWriter struct{ bytes.Buffer }

func (w *shortFrameWriter) Write(p []byte) (int, error) {
	if len(p) > 2 {
		p = p[:2]
	}
	return w.Buffer.Write(p)
}

type zeroFrameWriter struct{}

func (zeroFrameWriter) Write([]byte) (int, error) { return 0, nil }
func TestFrameShortWrites(t *testing.T) {
	var w shortFrameWriter
	if err := writeFrame(&w, opPing, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	op, p, err := readFrame(&w.Buffer)
	if err != nil || op != opPing || string(p) != "abc" {
		t.Fatalf("got %d %s %v", op, p, err)
	}
	if err := writeFrame(zeroFrameWriter{}, opFrame, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v", err)
	}
}
