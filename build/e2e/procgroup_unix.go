//go:build unix

package main

import (
	"errors"
	"os/exec"
	"syscall"
)

// setNewProcGroup starts cmd in a process group of its own, which the devnet
// processes it starts inherit.
func setNewProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup kills every process of the group led by pid.
func killProcGroup(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}

	return nil
}
