package desktop

import (
	"os/exec"
	"syscall"
)

func prepareTerminal(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x00000010 // CREATE_NEW_CONSOLE
}
