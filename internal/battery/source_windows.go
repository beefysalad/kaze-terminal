package battery

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")

	powrprof         = windows.NewLazySystemDLL("powrprof.dll")
	procRegisterPS   = powrprof.NewProc("PowerSettingRegisterNotification")
	procUnregisterPS = powrprof.NewProc("PowerSettingUnregisterNotification")
)

const deviceNotifyCallback = 2 // DEVICE_NOTIFY_CALLBACK

var (
	// GUID_BATTERY_PERCENTAGE_REMAINING
	guidBatteryPercent = windows.GUID{Data1: 0xA7AD8041, Data2: 0xB45A, Data3: 0x4CAE, Data4: [8]byte{0x87, 0xA3, 0xEE, 0xCB, 0xB4, 0x68, 0xA9, 0xE1}}
	// GUID_ACDC_POWER_SOURCE
	guidPowerSource = windows.GUID{Data1: 0x5D3E9A59, Data2: 0xE9D5, Data3: 0x4B00, Data4: [8]byte{0xA6, 0xBD, 0xFF, 0x34, 0xFF, 0x51, 0x65, 0x48}}
)

// systemPowerStatus is SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus, BatteryFlag, BatteryLifePercent, SystemStatusFlag byte
	BatteryLifeTime, BatteryFullLifeTime                            uint32
}

// subscribeParams is DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS.
type subscribeParams struct {
	callback, context uintptr
}

type winSource struct{}

// Native is the Windows battery source.
func Native() Source { return winSource{} }

// Read calls GetSystemPowerStatus once, without allocating.
func (winSource) Read() (Status, error) { return readPower() }

// readPower, register and unregister are variables so tests can fake them.
var (
	readPower  = readSystemPower
	register   = registerSetting
	unregister = unregisterSetting
)

func readSystemPower() (Status, error) {
	if err := procGetSystemPowerStatus.Find(); err != nil {
		return Status{}, err
	}
	var s systemPowerStatus
	r, _, err := syscall.SyscallN(procGetSystemPowerStatus.Addr(), uintptr(unsafe.Pointer(&s)))
	if r == 0 {
		return Status{}, err
	}
	return fromPowerStatus(s.ACLineStatus, s.BatteryFlag, s.BatteryLifePercent), nil
}

// The OS callback is made once per process (Go caps how many callbacks a
// process can create) and only hands the notification to the current watcher.
var (
	callbackOnce sync.Once
	params       subscribeParams // package-level: it outlives every registration
	current      atomic.Pointer[func()]
)

// dispatch is the DEVICE_NOTIFY_CALLBACK_ROUTINE. It runs on an OS thread and
// does nothing but call the watcher's changed: no read, no lock, no allocation.
func dispatch(context, typ, setting uintptr) uintptr {
	if f := current.Load(); f != nil {
		(*f)()
	}
	return 0
}

func registerSetting(g *windows.GUID) (uintptr, error) {
	if err := procRegisterPS.Find(); err != nil {
		return 0, err
	}
	callbackOnce.Do(func() { params.callback = windows.NewCallback(dispatch) })
	var h uintptr
	r, _, _ := syscall.SyscallN(procRegisterPS.Addr(), uintptr(unsafe.Pointer(g)), deviceNotifyCallback,
		uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return h, nil
}

func unregisterSetting(h uintptr) error {
	if err := procUnregisterPS.Find(); err != nil {
		return err
	}
	if r, _, _ := syscall.SyscallN(procUnregisterPS.Addr(), h); r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// Watch registers for battery-percentage and AC/DC changes. With no battery it
// registers nothing; if either registration fails, it undoes the other and
// reports ok false so the caller falls back to a lazy poll. One watcher per
// process: a later Watch replaces the earlier one's changed.
func (winSource) Watch(changed func()) (func(), bool) {
	if st, err := readPower(); err != nil || !st.Present {
		return nil, false
	}
	fn := &changed
	current.Store(fn)
	var hs [2]uintptr
	for i, g := range []*windows.GUID{&guidBatteryPercent, &guidPowerSource} {
		h, err := register(g)
		if err != nil {
			for _, done := range hs[:i] {
				unregister(done)
			}
			current.CompareAndSwap(fn, nil)
			return nil, false
		}
		hs[i] = h
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			current.CompareAndSwap(fn, nil)
			for _, h := range hs {
				unregister(h)
			}
		})
	}, true
}
