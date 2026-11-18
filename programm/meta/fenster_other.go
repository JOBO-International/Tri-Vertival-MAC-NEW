//go:build !windows

package meta

import "os/exec"

func versteckeFenster(cmd *exec.Cmd) {}
