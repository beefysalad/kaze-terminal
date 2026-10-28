//go:build windows

package ptyx

import "os/exec"

// DefaultShell prefers PowerShell 7 and falls back to the inbox Windows PowerShell.
func DefaultShell() []string {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return []string{"pwsh", "-NoLogo"}
	}
	return []string{"powershell.exe", "-NoLogo"}
}
