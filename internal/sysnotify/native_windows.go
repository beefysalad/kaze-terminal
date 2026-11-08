//go:build windows

package sysnotify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The GUIDs and vtable ordering follow the Windows SDK metadata published at
// github.com/microsoft/windows-rs (Windows/UI/Notifications and Data/Xml/Dom).
var (
	combase             = windows.NewLazySystemDLL("combase.dll")
	roInitialize        = combase.NewProc("RoInitialize")
	roUninitialize      = combase.NewProc("RoUninitialize")
	roFactory           = combase.NewProc("RoGetActivationFactory")
	roActivate          = combase.NewProc("RoActivateInstance")
	createHString       = combase.NewProc("WindowsCreateString")
	deleteHString       = combase.NewProc("WindowsDeleteString")
	ole32               = windows.NewLazySystemDLL("ole32.dll")
	coCreateInstance    = ole32.NewProc("CoCreateInstance")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	setProcessAppID     = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	user32              = windows.NewLazySystemDLL("user32.dll")
	isIconic            = user32.NewProc("IsIconic")
	showWindowAsync     = user32.NewProc("ShowWindowAsync")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")

	iidUnknown      = winGUID("00000000-0000-0000-c000-000000000046")
	iidAgile        = winGUID("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")
	iidManager      = winGUID("50ac103f-d235-4598-bbef-98fe4d1a3ad4")
	iidManager2     = winGUID("7ab93c52-0e48-4750-ba9d-1a4113981847")
	iidToastFactory = winGUID("04124b20-82c6-4229-b109-fd9ed4662b53")
	iidToast2       = winGUID("9dfb9fd1-143a-490e-90bf-b9fba7132de7")
	iidXMLIO        = winGUID("6cd0e74e-ee65-4489-9ebf-ca43e87ba637")
	// Parameterized TypedEventHandler IIDs, generated from the SDK runtime
	// signatures (not the non-parameterized generic interface identifier).
	iidActivated     = winGUID("ab54de2d-97d9-5528-b6ad-105afe156530")
	iidDismissed     = winGUID("61c2402f-0ed0-5a18-ab69-59f4aa99a368")
	iidFailed        = winGUID("95e3e803-c969-5e3a-9753-ea2ad22a9a33")
	clsidShellLink   = winGUID("00021401-0000-0000-c000-000000000046")
	iidShellLink     = winGUID("000214f9-0000-0000-c000-000000000046")
	iidPersistFile   = winGUID("0000010b-0000-0000-c000-000000000046")
	iidPropertyStore = winGUID("886d8eeb-8cf2-4446-8d02-cdba1dbdcf99")
)

func winGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}
func winHRESULT(op string, value uintptr) error {
	if int32(value) < 0 {
		return fmt.Errorf("%s: Windows error 0x%08x", op, uint32(value))
	}
	return nil
}

type winObject struct{ vtable *[32]uintptr }

//go:uintptrescapes
func (o *winObject) call(slot int, args ...uintptr) error {
	if o == nil {
		return errors.New("missing Windows notification interface")
	}
	all := make([]uintptr, 1, len(args)+1)
	all[0] = uintptr(unsafe.Pointer(o))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(o.vtable[slot], all...)
	runtime.KeepAlive(o)
	return winHRESULT("notification API", hr)
}
func (o *winObject) query(iid *windows.GUID) (*winObject, error) {
	var out *winObject
	err := o.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, err
}
func (o *winObject) release() {
	if o != nil {
		_, _, _ = syscall.SyscallN(o.vtable[2], uintptr(unsafe.Pointer(o)))
		runtime.KeepAlive(o)
	}
}

func newWindowsApartment() (*windowsApartment, error) {
	return newWindowsApartmentWith(func() error {
		if err := roInitialize.Find(); err != nil {
			return err
		}
		hr, _, _ := roInitialize.Call(1) // RO_INIT_MULTITHREADED
		return winHRESULT("Initialize Windows notifications", hr)
	}, func() { roUninitialize.Call() })
}
func withWinStrings(values []string, f func([]uintptr) error) error {
	hs := make([]uintptr, len(values))
	defer func() {
		for _, h := range hs {
			if h != 0 {
				deleteHString.Call(h)
			}
		}
	}()
	for i, s := range values {
		u := utf16.Encode([]rune(s))
		length := len(u)
		u = append(u, 0)
		hr, _, _ := createHString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(length), uintptr(unsafe.Pointer(&hs[i])))
		runtime.KeepAlive(u)
		if err := winHRESULT("Create notification string", hr); err != nil {
			return err
		}
	}
	return f(hs)
}
func winActivationFactory(class string, iid *windows.GUID) (out *winObject, err error) {
	err = withWinStrings([]string{class}, func(h []uintptr) error {
		hr, _, _ := roFactory.Call(h[0], uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
		return winHRESULT("Create notification factory", hr)
	})
	return
}

// New does no setup until the explicit startup permission check or first post.
// Tests construct newWindowsBackend with a fake driver instead.
func New(cb Callbacks) Backend { return newWindowsBackend(&nativeWindowsDriver{}, cb) }

type nativeWindowsDriver struct {
	notifier, history, factory *winObject
	apartment                  *windowsApartment
}

func (d *nativeWindowsDriver) Init() error {
	apartment, err := newWindowsApartment()
	if err != nil {
		return err
	}
	d.apartment = apartment
	err = apartment.run(func() (err error) {
		defer func() {
			if err != nil {
				d.release()
			}
		}()
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		exe, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return err
		}
		if err = windowsShortcutTarget(exe, ""); err != nil {
			return err
		}
		if err = ensureWindowsShortcut(exe); err != nil {
			return fmt.Errorf("Register Kaze notifications: %w", err)
		}
		appid, err := windows.UTF16PtrFromString(windowsAUMID)
		if err != nil {
			return err
		}
		hr, _, _ := setProcessAppID.Call(uintptr(unsafe.Pointer(appid)))
		if err = winHRESULT("Set Kaze notification identity", hr); err != nil {
			return err
		}
		mgr, err := winActivationFactory("Windows.UI.Notifications.ToastNotificationManager", &iidManager)
		if err != nil {
			return err
		}
		defer mgr.release()
		err = withWinStrings([]string{windowsAUMID}, func(h []uintptr) error { return mgr.call(7, h[0], uintptr(unsafe.Pointer(&d.notifier))) })
		if err != nil {
			return err
		}
		mgr2, err := winActivationFactory("Windows.UI.Notifications.ToastNotificationManager", &iidManager2)
		if err != nil {
			return err
		}
		defer mgr2.release()
		if err = mgr2.call(6, uintptr(unsafe.Pointer(&d.history))); err != nil {
			return err
		}
		d.factory, err = winActivationFactory("Windows.UI.Notifications.ToastNotification", &iidToastFactory)
		return err
	})
	if err != nil {
		apartment.close()
		d.apartment = nil
	}
	return err
}
func (d *nativeWindowsDriver) Permission() (status string, err error) {
	err = d.apartment.run(func() error {
		var setting uint32
		if err := d.notifier.call(8, uintptr(unsafe.Pointer(&setting))); err != nil {
			return err
		}
		switch setting {
		case 0:
		case 1, 2:
			status = "Blocked in Windows notification settings"
		case 3:
			status = "Blocked by Windows notification policy"
		default:
			status = "Windows notifications are unavailable"
		}
		return nil
	})
	return
}
func (d *nativeWindowsDriver) Post(n Notification, group, tag string, event func(windowsEvent)) (result windowsToast, err error) {
	err = d.apartment.run(func() (err error) {
		var document *winObject
		err = withWinStrings([]string{"Windows.Data.Xml.Dom.XmlDocument"}, func(h []uintptr) error {
			hr, _, _ := roActivate.Call(h[0], uintptr(unsafe.Pointer(&document)))
			return winHRESULT("Create notification document", hr)
		})
		if err != nil {
			return err
		}
		defer document.release()
		xmlio, err := document.query(&iidXMLIO)
		if err != nil {
			return err
		}
		defer xmlio.release()
		if err = withWinStrings([]string{windowsToastXML(n)}, func(h []uintptr) error { return xmlio.call(6, h[0]) }); err != nil {
			return err
		}
		xmlDocumentIID := winGUID("f7f3a506-1e87-42d6-bcfb-b8c809fa5494")
		xmlDocument, err := document.query(&xmlDocumentIID)
		if err != nil {
			return err
		}
		defer xmlDocument.release()
		var toast *winObject
		if err = d.factory.call(6, uintptr(unsafe.Pointer(xmlDocument)), uintptr(unsafe.Pointer(&toast))); err != nil {
			return err
		}
		live := &nativeWindowsToast{object: toast, notifier: d.notifier, apartment: d.apartment}
		defer func() {
			if err != nil {
				live.release()
			}
		}()
		identity, err := toast.query(&iidToast2)
		if err != nil {
			return err
		}
		defer identity.release()
		if err = withWinStrings([]string{tag, group}, func(h []uintptr) error {
			if err := identity.call(6, h[0]); err != nil {
				return err
			}
			return identity.call(8, h[1])
		}); err != nil {
			return err
		}
		if err = live.subscribe(11, 12, iidActivated, func(*winObject) { event(windowsEvent{kind: "activate"}) }); err != nil {
			return err
		}
		if err = live.subscribe(9, 10, iidDismissed, func(args *winObject) {
			var reason uint32
			err := args.call(6, uintptr(unsafe.Pointer(&reason)))
			if err == nil && reason == 2 {
				event(windowsEvent{kind: "timeout"})
				return
			}
			event(windowsEvent{kind: "dismiss", err: err})
		}); err != nil {
			return err
		}
		if err = live.subscribe(13, 14, iidFailed, func(args *winObject) {
			var code uint32
			err := args.call(6, uintptr(unsafe.Pointer(&code)))
			if err == nil {
				err = fmt.Errorf("Windows notification failed: 0x%08x", code)
			}
			event(windowsEvent{kind: "failed", err: err})
		}); err != nil {
			return err
		}
		if err = d.notifier.call(6, uintptr(unsafe.Pointer(toast))); err != nil {
			return err
		}
		result = live
		return nil
	})
	return
}
func (d *nativeWindowsDriver) Remove(group, tag string) error {
	return d.apartment.run(func() error {
		return withWinStrings([]string{tag, group, windowsAUMID}, func(h []uintptr) error { return d.history.call(8, h[0], h[1], h[2]) })
	})
}
func (d *nativeWindowsDriver) Raise(hwnd uintptr) {
	iconic, _, _ := isIconic.Call(hwnd)
	if iconic != 0 {
		showWindowAsync.Call(hwnd, 9)
	} // SW_RESTORE, on the window's owning thread
	setForegroundWindow.Call(hwnd)
}
func (d *nativeWindowsDriver) release() {
	d.notifier.release()
	d.history.release()
	d.factory.release()
	d.notifier, d.history, d.factory = nil, nil, nil
}
func (d *nativeWindowsDriver) Close() {
	if d.apartment == nil {
		return
	}
	_ = d.apartment.run(func() error { d.release(); return nil })
	d.apartment.close()
	d.apartment = nil
}

type nativeSubscription struct {
	remove  int
	token   int64
	handler *nativeToastHandler
}
type nativeWindowsToast struct {
	object        *winObject
	notifier      *winObject
	apartment     *windowsApartment
	subscriptions []nativeSubscription
	once          sync.Once
}

func (t *nativeWindowsToast) subscribe(add, remove int, iid windows.GUID, invoke func(*winObject)) error {
	h := newNativeToastHandler(iid, invoke)
	var token int64
	if err := t.object.call(add, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token))); err != nil {
		h.release()
		return err
	}
	t.subscriptions = append(t.subscriptions, nativeSubscription{remove, token, h})
	return nil
}
func (t *nativeWindowsToast) release() {
	// Remove a still-visible banner as well as the history entry removed by
	// the backend. Revoke handlers before releasing their Go-owned reference.
	_ = t.notifier.call(7, uintptr(unsafe.Pointer(t.object)))
	for _, s := range t.subscriptions {
		_ = t.object.call(s.remove, uintptr(s.token))
		s.handler.release()
	}
	t.subscriptions = nil
	t.object.release()
	t.object = nil
}
func (t *nativeWindowsToast) Close() {
	t.once.Do(func() { _ = t.apartment.run(func() error { t.release(); return nil }) })
}

// Windows owns COM references to these Go delegates. Keep each delegate pinned
// and strongly reachable until its final native Release, including in-flight
// callbacks after an event registration is revoked.
var nativeHandlers = struct {
	sync.Mutex
	all map[*nativeToastHandler]struct{}
}{all: make(map[*nativeToastHandler]struct{})}

type nativeToastHandler struct {
	vtable *[4]uintptr
	iid    windows.GUID
	refs   atomic.Int32
	invoke func(*winObject)
	pin    runtime.Pinner
}

var nativeHandlerVTable = [4]uintptr{
	syscall.NewCallback(nativeHandlerQuery),
	syscall.NewCallback(nativeHandlerAddRef),
	syscall.NewCallback(nativeHandlerRelease),
	syscall.NewCallback(nativeHandlerInvoke),
}

func nativeHandlerQuery(h *nativeToastHandler, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if out == nil || iid == nil {
		return 0x80004003
	}
	*out = nil
	if *iid != iidUnknown && *iid != iidAgile && *iid != h.iid {
		return 0x80004002
	}
	h.refs.Add(1)
	*out = unsafe.Pointer(h)
	return 0
}
func nativeHandlerAddRef(h *nativeToastHandler) uintptr  { return uintptr(h.refs.Add(1)) }
func nativeHandlerRelease(h *nativeToastHandler) uintptr { return uintptr(h.release()) }
func nativeHandlerInvoke(h *nativeToastHandler, sender, args *winObject) (result uintptr) {
	defer func() {
		if recover() != nil {
			result = 0x80004005
		}
	}()
	h.invoke(args)
	runtime.KeepAlive(h)
	return 0
}

func newNativeToastHandler(iid windows.GUID, invoke func(*winObject)) *nativeToastHandler {
	h := &nativeToastHandler{vtable: &nativeHandlerVTable, iid: iid, invoke: invoke}
	h.refs.Store(1)
	h.pin.Pin(h)
	nativeHandlers.Lock()
	nativeHandlers.all[h] = struct{}{}
	nativeHandlers.Unlock()
	return h
}
func (h *nativeToastHandler) release() int32 {
	n := h.refs.Add(-1)
	if n == 0 {
		nativeHandlers.Lock()
		delete(nativeHandlers.all, h)
		nativeHandlers.Unlock()
		h.pin.Unpin()
	}
	return n
}
