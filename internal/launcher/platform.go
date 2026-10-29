package launcher

import (
	"context"
	"os/exec"
)

type Native struct{}

func (Native) StartOwnService(ctx context.Context, path string, args []string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	cmd := exec.Command(path, args...)
	hide(cmd)
	if e := cmd.Start(); e != nil {
		return e
	}
	return cmd.Process.Release()
}
