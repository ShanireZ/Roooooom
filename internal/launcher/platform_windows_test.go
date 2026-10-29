//go:build windows

package launcher

import (
	"os/exec"
	"testing"
)

func TestWindowsLaunchHidden(t *testing.T) {
	cmd := exec.Command("not-started")
	hide(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("visible console")
	}
}
