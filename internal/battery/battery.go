// Package battery reads the laptop battery level for the status bar. It is
// the only package that asks the OS about power. Windows reads
// GetSystemPowerStatus and hears about changes from power-setting
// notifications; macOS reads IOKit power sources and hears about changes from
// the system notify alerts; every other OS reports no battery.
package battery

// Status is one battery reading. It is comparable and holds no pointers, so
// the status model copies it without allocating.
type Status struct {
	Present, Charging bool
	Percent           int // 0-100; 0 when not Present
}

// Source reads the battery and, where the OS can, reports changes.
type Source interface {
	Read() (Status, error)
	// Watch calls changed (on an OS thread) when the battery level or the
	// power source changes. ok is false when the platform has no
	// notifications or registering failed; stop is then nil.
	Watch(changed func()) (stop func(), ok bool)
}

// Windows SYSTEM_POWER_STATUS values.
const (
	flagCharging  = 8
	flagNoBattery = 128
	unknown       = 255
	acOnline      = 1
)

// fromPowerStatus decodes SYSTEM_POWER_STATUS. Charging is Windows' charging
// flag, or on AC, below 100 %: a full battery shows no bolt even when Windows
// still sets the flag (it does at 100 % on AC on some laptops).
func fromPowerStatus(acLine, flag, percent byte) Status {
	if flag == flagNoBattery || flag == unknown || percent == unknown {
		return Status{}
	}
	p := min(int(percent), 100)
	return Status{Present: true, Percent: p, Charging: p < 100 && (flag&flagCharging != 0 || acLine == acOnline)}
}

// fromIOPS decodes macOS IOKit power-source values: whether an internal
// battery was found, IOKit's charging flag, whether the Mac is on AC, and the
// current and maximum capacity. Charging follows the Windows rule: the flag or
// AC, below 100 %, so a full battery shows no bolt.
func fromIOPS(found, charging, onAC bool, cur, full int) Status {
	if !found || full <= 0 {
		return Status{}
	}
	p := min(max(0, (cur*100+full/2)/full), 100)
	return Status{Present: true, Percent: p, Charging: p < 100 && (charging || onAC)}
}
