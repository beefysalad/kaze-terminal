package sound

import (
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/assets/sounds"
)

func TestBundledChimeIsValidWav(t *testing.T) {
	b := sounds.Chime
	if len(b) < 44 || len(b) >= 64*1024 {
		t.Fatalf("size = %d", len(b))
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[12:16]) != "fmt " {
		t.Fatal("bad RIFF/WAVE/fmt header")
	}
	if f := binary.LittleEndian.Uint16(b[20:]); f != 1 {
		t.Fatalf("format = %d, want PCM", f)
	}
	if ch := binary.LittleEndian.Uint16(b[22:]); ch != 1 {
		t.Fatalf("channels = %d", ch)
	}
	rate := binary.LittleEndian.Uint32(b[24:])
	if rate != 44100 {
		t.Fatalf("rate = %d", rate)
	}
	if bits := binary.LittleEndian.Uint16(b[34:]); bits != 16 {
		t.Fatalf("bits = %d", bits)
	}
	if string(b[36:40]) != "data" {
		t.Fatal("no data chunk")
	}
	n := binary.LittleEndian.Uint32(b[40:])
	if int(n) != len(b)-44 {
		t.Fatalf("data size %d, file has %d", n, len(b)-44)
	}
	dur := float64(n) / 2 / float64(rate)
	if dur < 0.3 || dur > 0.5 {
		t.Fatalf("duration = %.3fs", dur)
	}
}

func TestPlayCoalescesWithinOneSecond(t *testing.T) {
	var plays atomic.Int32
	p := newWith("", func([]byte) { plays.Add(1) })
	t0 := time.Now()
	p.Play(t0)
	p.Play(t0.Add(100 * time.Millisecond))
	p.Play(t0.Add(200 * time.Millisecond))
	waitFor(t, &plays, 1)
	time.Sleep(50 * time.Millisecond)
	if n := plays.Load(); n != 1 {
		t.Fatalf("plays = %d, want 1", n)
	}
	p.Play(t0.Add(1300 * time.Millisecond))
	waitFor(t, &plays, 2)
}

func TestPlayNeverBlocks(t *testing.T) {
	p := newWith("", func([]byte) { time.Sleep(5 * time.Second) })
	start := time.Now()
	p.Play(time.Now())
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("Play took %v", d)
	}
}

func TestMissingCustomFileFallsBackToBundled(t *testing.T) {
	var got []byte
	done := make(chan struct{})
	p := newWith("Z:/definitely/not/here.wav", func(b []byte) { got = b; close(done) })
	p.Play(time.Now())
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("no play")
	}
	if len(got) != len(sounds.Chime) {
		t.Fatalf("played %d bytes, want bundled %d", len(got), len(sounds.Chime))
	}
}

func waitFor(t *testing.T, c *atomic.Int32, n int32) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if c.Load() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("plays = %d, want %d", c.Load(), n)
}
