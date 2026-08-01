//go:build windows

package runner

import (
	"os"
	"os/exec"
	"path/filepath"
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
	killTree := exec.Command(
		taskkillPath(os.Getenv("SystemRoot")),
		"/T",
		"/F",
		"/PID",
		strconv.Itoa(command.Process.Pid),
	)
	if err := killTree.Run(); err == nil {
		return nil
	}
	return command.Process.Kill()
}

// taskkillPath builds an absolute path under the system root instead of letting
// the name resolve through PATH. Nodesmith lets the user replace PATH at
// runtime, and the command used to stop untrusted generator processes must not
// be selectable through that setting.
func taskkillPath(systemRoot string) string {
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32", "taskkill.exe")
}
