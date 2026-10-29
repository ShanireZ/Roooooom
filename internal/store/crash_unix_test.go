//go:build !windows

package store

import "os/exec"

func hideCrash(c *exec.Cmd) {}
