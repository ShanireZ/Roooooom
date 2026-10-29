package launcher

import (
	"context"

	"roooooom.local/roooooom/internal/i18n"
)

type Menu struct {
	Open     func() error
	Stop     func() error
	Activate func() error
	// Language reports the owner language when the service can provide it.
	Language func() i18n.Locale
}
type Platform interface {
	StartOwnService(context.Context, string, []string) error
	OpenOwnUI(string) error
	ShowTray(Menu) error
}

type trayItem struct {
	id    uintptr
	label string
}

// language resolves the owner language, then the OS UI language, then English.
func (m Menu) language() i18n.Locale {
	var owner i18n.Locale
	if m.Language != nil {
		owner = m.Language()
	}
	os, _ := i18n.OS()
	return i18n.First(owner, os)
}

func trayLabels(l i18n.Locale) []trayItem {
	return []trayItem{{1, i18n.T(l, i18n.TrayOpen)}, {4, i18n.T(l, i18n.TrayReactivate)}, {2, i18n.T(l, i18n.TrayExit)}, {3, i18n.T(l, i18n.TrayStop)}}
}
