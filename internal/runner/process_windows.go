//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	killTree := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
	if err := killTree.Run(); err == nil {
		return nil
	}
	return command.Process.Kill()
}
