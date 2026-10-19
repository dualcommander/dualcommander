//go:build !windows

package desktop

import "os/exec"

func prepareTerminal(cmd *exec.Cmd) {}
