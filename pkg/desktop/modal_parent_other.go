//go:build !linux || !cgo

package desktop

// Native input blocking is handled by SetWindowBlocked on Windows.
// macOS keeps normal framework window activation.
func SetModalParent(parent, child any) {}
