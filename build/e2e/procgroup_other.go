//go:build !unix

package main

import (
	"os"
	"os/exec"
)

func setNewProcGroup(*exec.Cmd) {}

// killProcGroup kills the process only: process groups are a Unix notion.
func killProcGroup(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return p.Kill()
}
