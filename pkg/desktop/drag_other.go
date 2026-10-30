//go:build !darwin || !cgo

package desktop

import "fyne.io/fyne/v2"

// ponytail: outbound OS dragging is macOS-only; pane-to-pane copying works elsewhere.
func StartFileDrag(window fyne.Window, paths []string) bool { return false }
