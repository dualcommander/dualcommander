//go:build !darwin || !cgo

package desktop

// SetAppIcon leaves application icons to Fyne on other platforms.
func SetAppIcon(png []byte) bool { return false }
