//go:build screenshots

// Command screenshots renders the user manual's app screenshots headlessly.
// Usage: go run -tags screenshots ./cmd/screenshots DIR
package main

import (
	"fmt"
	"os"

	"github.com/dualcommander/dualcommander/internal/commander"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: screenshots DIR")
		os.Exit(2)
	}
	if err := commander.CaptureManual(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
