//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package runner

import "os/exec"

func configureProcess(command *exec.Cmd) {}

func terminateProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}
