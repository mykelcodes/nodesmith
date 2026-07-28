//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package runner

import (
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	processGroupID, err := syscall.Getpgid(command.Process.Pid)
	if err == nil {
		return syscall.Kill(-processGroupID, syscall.SIGKILL)
	}
	return command.Process.Kill()
}
