//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework AppKit
void setWindowBlocked(unsigned long window, int blocked);
*/
import "C"

import "fyne.io/fyne/v2/driver"

// SetWindowBlocked prevents parent activation without changing dialog decorations.
func SetWindowBlocked(context any, blocked bool) {
	if window, ok := context.(driver.MacWindowContext); ok && window.NSWindow != 0 {
		value := 0
		if blocked {
			value = 1
		}
		C.setWindowBlocked(C.ulong(window.NSWindow), C.int(value))
	}
}
