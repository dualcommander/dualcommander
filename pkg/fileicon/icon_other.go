//go:build !darwin || !cgo

package fileicon

// PNG leaves unsupported platforms to the caller's fallback icons.
func PNG(extension string, folder bool) []byte { return nil }

// Preview leaves unsupported platforms to the caller's fallback icons.
func Preview(path string) []byte { return nil }
