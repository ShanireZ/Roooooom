package launcher

import "context"

type Menu struct {
	Open     func() error
	Stop     func() error
	Activate func() error
}
type Platform interface {
	StartOwnService(context.Context, string, []string) error
	OpenOwnUI(string) error
	ShowTray(Menu) error
}
