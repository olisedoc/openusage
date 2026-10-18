//go:build windows

package codex

import "os/exec"

// isolateProcessGroup is a no-op: Windows has no process groups in the Unix
// sense.
func isolateProcessGroup(cmd *exec.Cmd) {}

// killProcessTree kills the app-server. A surviving grandchild cannot hold the
// pipes open because cmd.WaitDelay closes them.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
