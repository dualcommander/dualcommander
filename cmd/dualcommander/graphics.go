package main

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

const graphicsProbeTimeout = 2 * time.Second
const graphicsModeEnv = "DUALCOMMANDER_INTERNAL_GRAPHICS"

func graphicsCommand(exe, mode string, args ...string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), graphicsModeEnv+"=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, graphicsModeEnv+"="+mode)
	return cmd
}

func runGraphics(probe func() error, launch func(bool) error) error {
	return launch(probe() != nil)
}
