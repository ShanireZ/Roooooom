//go:build darwin

package launcher

func (Native) OpenOwnUI(target string) error { return openBrowser("open", target) }
func (Native) ShowTray(Menu) error           { return ErrTrayUnavailable }
