//go:build !darwin || !cgo

package desktop

// ponytail: native file clipboard export is macOS-only; other platforms retain app-local copies.
func CopyFiles(paths []string) error { return nil }
