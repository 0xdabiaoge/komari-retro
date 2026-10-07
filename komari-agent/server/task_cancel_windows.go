//go:build windows

package server

import (
	"os/exec"
	"strconv"
)

func configureTaskCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() }
}
