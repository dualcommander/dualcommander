//go:build (!darwin || !cgo) && !windows && (!linux || !cgo)

package desktop

// Other backends retain the input overlay; Fyne has no portable decorated-modal API.
func SetWindowBlocked(context any, blocked bool) {}
