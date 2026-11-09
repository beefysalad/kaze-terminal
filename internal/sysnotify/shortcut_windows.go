//go:build windows

package sysnotify

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type winPropertyKey struct {
	format windows.GUID
	id     uint32
}
type winPropVariant struct {
	kind     uint16
	reserved [3]uint16
	value    [2]uintptr
}

var winAppIDKey = winPropertyKey{winGUID("9f4c2855-9f79-4b39-a8d0-e1d42de1d5f3"), 5}

func newShellLink() (*winObject, error) {
	var link *winObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, 1, uintptr(unsafe.Pointer(&iidShellLink)), uintptr(unsafe.Pointer(&link)))
	return link, winHRESULT("Create Kaze shortcut", hr)
}
func withWinWide(s string, f func(uintptr) error) error {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return err
	}
	err = f(uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	return err
}

// Generate the complete shortcut in a private temporary file first. The final
// destination is held without write/delete sharing during ownership inspection
// and replacement, so a racing unrelated shortcut is never overwritten.
func ensureWindowsShortcut(exe string) error {
	programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(programs, 0700); err != nil {
		return err
	}
	link, err := newShellLink()
	if err != nil {
		return err
	}
	defer link.release()
	if err = withWinWide(exe, func(p uintptr) error { return link.call(20, p) }); err != nil {
		return err
	}
	if err = withWinWide("", func(p uintptr) error { return link.call(11, p) }); err != nil {
		return err
	}
	if err = withWinWide(filepath.Dir(exe), func(p uintptr) error { return link.call(9, p) }); err != nil {
		return err
	}
	if err = withWinWide("Kaze terminal", func(p uintptr) error { return link.call(7, p) }); err != nil {
		return err
	}
	if err = withWinWide(exe, func(p uintptr) error { return link.call(17, p, 0) }); err != nil {
		return err
	}
	store, err := link.query(&iidPropertyStore)
	if err != nil {
		return err
	}
	defer store.release()
	if err = withWinWide(windowsAUMID, func(p uintptr) error {
		v := winPropVariant{kind: 31, value: [2]uintptr{p, 0}} // VT_LPWSTR; SetValue copies the string
		return store.call(6, uintptr(unsafe.Pointer(&winAppIDKey)), uintptr(unsafe.Pointer(&v)))
	}); err != nil {
		return err
	}
	if err = store.call(7); err != nil {
		return err
	}
	persist, err := link.query(&iidPersistFile)
	if err != nil {
		return err
	}
	defer persist.release()
	temp, err := os.CreateTemp(programs, ".kaze-shortcut-*.lnk")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	temp.Close()
	defer os.Remove(tempName)
	if err = withWinWide(tempName, func(p uintptr) error { return persist.call(6, p, 1) }); err != nil {
		return err
	}
	saved, err := os.Open(tempName)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(saved, (1<<20)+1))
	saved.Close()
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("Kaze shortcut exceeds size limit")
	}
	return writeWindowsShortcut(filepath.Join(programs, "Kaze.lnk"), exe, data)
}

func writeWindowsShortcut(name, exe string, data []byte) error {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var handle windows.Handle
	created := false
	// Bounded retry if another Kaze creates its shortcut between open/create.
	for attempt := 0; attempt < 2; attempt++ {
		handle, err = windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return err
		}
		handle, err = windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			created = true
			break
		}
		if !errors.Is(err, windows.ERROR_FILE_EXISTS) && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return err
		}
	}
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(handle), name)
	defer file.Close()
	if !created {
		target, err := readWindowsShortcutTarget(name)
		if err != nil {
			return fmt.Errorf("Refuse unreadable existing Kaze shortcut: %w", err)
		}
		if target == "" {
			return errors.New("Refuse Kaze shortcut without an executable target")
		}
		if err = windowsShortcutTarget(exe, target); err != nil {
			return err
		}
		// Avoid rewriting an unchanged owned shortcut on every startup.
		old, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
		if err != nil {
			return err
		}
		if string(old) == string(data) {
			return nil
		}
		if _, err = file.Seek(0, 0); err != nil {
			return err
		}
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Truncate(int64(len(data))); err != nil {
		return err
	}
	return file.Sync()
}
func readWindowsShortcutTarget(name string) (string, error) {
	link, err := newShellLink()
	if err != nil {
		return "", err
	}
	defer link.release()
	persist, err := link.query(&iidPersistFile)
	if err != nil {
		return "", err
	}
	defer persist.release()
	// STGM_READ | STGM_SHARE_DENY_NONE allows our existing ownership lock.
	if err = withWinWide(name, func(p uintptr) error { return persist.call(5, p, 0x40) }); err != nil {
		return "", err
	}
	buf := make([]uint16, 32768)
	// SLGP_RAWPATH: inspect the stored target without resolving or executing it.
	if err = link.call(3, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0, 4); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}
