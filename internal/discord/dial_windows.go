package discord

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Dial opens a local pipe in overlapped mode. Go's poller can then cancel pending
// reads and writes on Close and supports deadlines for a frozen Discord client.
func Dial() (io.ReadWriteCloser, error) {
	var err error
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		f, e := openPipe(name)
		if e != nil {
			err = e
			continue
		}
		return f, nil
	}
	return nil, err
}

func openPipe(name string) (*os.File, error) {
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(handle), name)
	// Refuse a handle the runtime cannot poll before any I/O is submitted.
	if err = f.SetDeadline(time.Time{}); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
