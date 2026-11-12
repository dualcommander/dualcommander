// Command dualcommander runs the two-pane desktop file manager.
package main

import (
	"log/slog"
	"os"

	"github.com/dualcommander/dualcommander/internal/commander"
)

func main() {
	exe, _ := os.Executable()
	closeLog, err := setupLogging(os.Getenv, exe, os.Stderr)
	defer closeLog()
	if err != nil {
		slog.Error("Cannot open debug log", "error", err)
	}
	if handled, err := graphicsStartup(); handled || err != nil {
		if err != nil {
			slog.Error("Graphics startup failed", "error", err)
			os.Exit(1)
		}
		return
	}
	commander.Run()
}
