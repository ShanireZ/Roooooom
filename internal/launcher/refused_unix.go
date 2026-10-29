//go:build !windows

package launcher

import (
	"errors"
	"syscall"
)

func connectionRefused(e error) bool { return errors.Is(e, syscall.ECONNREFUSED) }
