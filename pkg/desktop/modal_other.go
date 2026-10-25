//go:build (!darwin || !cgo) && !windows

package desktop

// ponytail: other backends retain the input overlay; add native activation guards
// when Fyne exposes a portable decorated-modal API (including Wayland).
func SetWindowBlocked(context any, blocked bool) {}
