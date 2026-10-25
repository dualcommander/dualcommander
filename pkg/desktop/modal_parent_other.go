//go:build !linux || !cgo

package desktop

// Native input blocking is handled by SetWindowBlocked on macOS and Windows.
func SetModalParent(parent, child any) {}
