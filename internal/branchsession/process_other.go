//go:build !windows

package branchsession

import "os/exec"

func hideWindow(*exec.Cmd) {}
