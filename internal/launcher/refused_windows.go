//go:build windows

package launcher

import (
	"errors"
	"golang.org/x/sys/windows"
)

func connectionRefused(e error) bool { return errors.Is(e, windows.WSAECONNREFUSED) }
