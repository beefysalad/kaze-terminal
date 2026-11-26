//go:build unix

package orchestrate

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTemplateFIFOReplacementDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.md")
	writeTemplate(t, dir, "review", "---\nagent: codex\n---\nReview")
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	checked, err := root.Lstat("review.md")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	// This is the exact post-Lstat replacement: the open sees a FIFO instead
	// of the previously checked regular file. Always unblock and join on RED.
	done := make(chan error, 1)
	go func() {
		f, err := openCheckedTemplate(root, "review.md", checked)
		if f != nil {
			f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO replacement accepted")
		}
	case <-time.After(200 * time.Millisecond):
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0600)
		if err != nil {
			t.Fatalf("unblock FIFO: %v", err)
		}
		defer syscall.Close(fd)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("FIFO open did not join within 2s deadline")
		}
		t.Fatal("template open blocked on FIFO replacement")
	}
}

func TestTemplateFIFOLoadAndList(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "review.md"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() {
		_, _, err := LoadTemplate(dir, "review")
		catalog := ListTemplates(dir)
		done <- err != nil && len(catalog.Templates) == 0 && len(catalog.Warnings) > 0
	}()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("FIFO accepted by load or list")
		}
	case <-time.After(time.Second):
		t.Fatal("load/list FIFO exceeded 2s deadline")
	}
}
