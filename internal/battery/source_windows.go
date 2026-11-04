package battery

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
)

// systemPowerStatus is SYSTEM_POWER_STATUS.
type systemPowerStatus struct {
	ACLineStatus, BatteryFlag, BatteryLifePercent, SystemStatusFlag byte
	BatteryLifeTime, BatteryFullLifeTime                            uint32
}

type winSource struct{}

// Native is the Windows battery source.
func Native() Source { return winSource{} }

// Read calls GetSystemPowerStatus once, without allocating.
func (winSource) Read() (Status, error) {
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

func (winSource) Watch(changed func()) (func(), bool) { return nil, false }
