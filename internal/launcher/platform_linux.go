//go:build linux

package launcher

func (Native) OpenOwnUI(target string) error { return openBrowser("xdg-open", target) }
func (Native) ShowTray(Menu) error           { return ErrTrayUnavailable }
