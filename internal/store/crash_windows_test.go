//go:build windows

package store

import (
	"os/exec"
	"syscall"
)

func hideCrash(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
