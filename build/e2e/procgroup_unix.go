//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// shutdownSignals are the signals on which we tear down a running devnet.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// setNewProcGroup makes cmd (the `go test` invocation) the leader of a fresh
// process group, so its whole subtree (the test binary and the
// beacon/validator/gqrl/bootnode processes it launches) can be killed together.
func setNewProcGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Setpgid = true
}

// killProcGroup SIGKILLs the process group led by pid (Setpgid makes the group
// id equal the leader pid).
func killProcGroup(pid int) error {
	if pid <= 0 {
		return nil
	}

	return syscall.Kill(-pid, syscall.SIGKILL)
}
