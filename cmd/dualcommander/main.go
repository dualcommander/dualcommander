// Command dualcommander runs the two-pane desktop file manager.
package main

import (
	"log/slog"
	"os"

	"github.com/dualcommander/dualcommander/internal/commander"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)).With("app", "DualCommander"))
	commander.Run()
}
