// Command dualcommander runs the two-pane desktop file manager.
package main

import (
	"log/slog"
	"os"

	"github.com/dualcommander/dualcommander/internal/commander"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)).With("app", "DualCommander"))
	if handled, err := graphicsStartup(); handled || err != nil {
		if err != nil {
			slog.Error("Graphics startup failed", "error", err)
			os.Exit(1)
		}
		return
	}
	commander.Run()
}
