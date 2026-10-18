//go:build !windows

package codex

import (
	"errors"
	"os/exec"
	"syscall"
)

// isolateProcessGroup makes the app-server its own process-group leader, so the
// whole tree can be killed at once.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree kills the app-server and every child it started. The codex
// CLI is a Node shim: the real binary is a child of it, so killing only the
// shim leaves the child holding the stdout pipe and the RPC read and
// cmd.Wait block until that child exits.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
