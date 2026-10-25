package desktop

import (
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var enableWindow = windows.NewLazySystemDLL("user32.dll").NewProc("EnableWindow")

// SetWindowBlocked disables native parent input and activation until modal closes.
func SetWindowBlocked(context any, blocked bool) {
	if window, ok := context.(driver.WindowsWindowContext); ok && window.HWND != 0 {
		enabled := uintptr(1)
		if blocked {
			enabled = 0
		}
		enableWindow.Call(window.HWND, enabled)
	}
}
