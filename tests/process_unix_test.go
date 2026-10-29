//go:build !windows

package tests

import "os/exec"

func hideProcess(cmd *exec.Cmd) {}
