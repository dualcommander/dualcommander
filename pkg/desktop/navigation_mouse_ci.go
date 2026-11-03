//go:build ci

package desktop

import "fyne.io/fyne/v2"

// Headless CI driver has no GLFW window or native mouse events.
func InstallNavigationMouse(fyne.Window, func(int)) {}
