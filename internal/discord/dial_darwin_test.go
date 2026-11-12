package discord

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDialFindsSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "kd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", dir)
	l, err := net.Listen("unix", filepath.Join(dir, "discord-ipc-1"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	type result struct {
		c   io.ReadWriteCloser
		err error
	}
	done := make(chan result, 1)
	go func() { c, err := Dial(); done <- result{c, err} }()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		r.c.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("Dial did not return")
	}
	l.Close()
	start := time.Now()
	go func() { c, err := Dial(); done <- result{c, err} }()
	select {
	case r := <-done:
		if r.c != nil {
			r.c.Close()
		}
		if r.err == nil {
			t.Fatal("Dial without socket succeeded")
		}
		if time.Since(start) > time.Second {
			t.Fatal("missing socket took over 1 s")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Dial without socket did not return")
	}
}
