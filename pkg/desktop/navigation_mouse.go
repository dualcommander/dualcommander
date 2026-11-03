//go:build !ci

package desktop

import (
	"fyne.io/fyne/v2"
	"github.com/go-gl/glfw/v3.4/glfw"
)

// InstallNavigationMouse must run on the native event thread after window creation.
// Fyne 2.8 discards buttons 4/5, so intercept those before its mouse callback.
func InstallNavigationMouse(window fyne.Window, navigate func(int)) {
	context, ok := window.(interface{ RunWithContext(func()) })
	if !ok {
		return
	}
	context.RunWithContext(func() {
		if native := glfw.GetCurrentContext(); native != nil {
			previous := native.SetMouseButtonCallback(nil)
			native.SetMouseButtonCallback(navigationMouseCallback(previous, func(step int) {
				fyne.Do(func() { navigate(step) })
			}))
		}
	})
}

func navigationMouseCallback(previous glfw.MouseButtonCallback, navigate func(int)) glfw.MouseButtonCallback {
	return func(window *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		if button == glfw.MouseButton4 || button == glfw.MouseButton5 {
			if action == glfw.Press {
				step := -1
				if button == glfw.MouseButton5 {
					step = 1
				}
				navigate(step)
			}
			return
		}
		if previous != nil {
			previous(window, button, action, mods)
		}
	}
}
