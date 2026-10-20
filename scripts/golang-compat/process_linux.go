package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Killing only the Go driver leaves a compiler running after the deadline.
// The measured Linux profile owns an isolated process group for each compiler.
func configureCompilerProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
