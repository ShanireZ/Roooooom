//go:build windows

package launcher

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
}
func ShowError(err error) {
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(utf16("Roooooom 未能完成本機操作。\n"+err.Error()))), uintptr(unsafe.Pointer(utf16("Roooooom"))), 0x10)
}
func (Native) OpenOwnUI(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return e
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
