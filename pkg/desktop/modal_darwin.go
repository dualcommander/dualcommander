//go:build darwin && cgo

package desktop

// ponytail: macOS uses normal Fyne window activation; parent actions are blocked
// in commander. Use a supported framework modal API if one becomes available.
func SetWindowBlocked(context any, blocked bool) {}
