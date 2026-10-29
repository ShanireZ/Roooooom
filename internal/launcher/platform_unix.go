//go:build !windows

package launcher

import (
	"os/exec"
	"syscall"
)

func hide(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func openBrowser(command, target string) error {
	cmd := exec.Command(command, target)
	if e := cmd.Start(); e != nil {
		return e
	}
	return cmd.Process.Release()
}
func ShowError(error) {}
