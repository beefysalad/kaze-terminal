package discord

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsPipeDeadlineAndClose(t *testing.T) {
	// A private pipe avoids connecting to the user's running Discord instance.
	name := fmt.Sprintf(`\\.\pipe\kaze-discord-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	server, err := windows.CreateNamedPipe(path,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		1, 1024, 1024, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(server)
	f, err := openPipe(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// CreateFile connects to the waiting instance before ConnectNamedPipe runs.
	if err := windows.ConnectNamedPipe(server, &windows.Overlapped{}); !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		t.Fatalf("connect: %v", err)
	}
	if err := f.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	read := make(chan error, 1)
	go func() { var b [1]byte; _, err := f.Read(b[:]); read <- err }()
	select {
	case err := <-read:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("read deadline: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read ignored deadline")
	}
	if err := f.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	go func() { var b [1]byte; _, err := f.Read(b[:]); read <- err }()
	// Leave a read pending on the silent peer before closing the client handle.
	select {
	case err := <-read:
		t.Fatalf("silent peer read returned early: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	closed := make(chan error, 1)
	go func() { closed <- f.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close waited for silent peer")
	}
	select {
	case err := <-read:
		if err == nil {
			t.Fatal("read succeeded after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not cancel pending read")
	}
}
