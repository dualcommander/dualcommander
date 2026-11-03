//go:build (!darwin && !linux && !windows) || (darwin && !cgo)

package desktop

import "context"

// Unsupported platforms retain app-local copies.
func CopyFiles(paths []string) error { return nil }

func ReadFiles(context.Context) ([]string, error) { return nil, nil }
