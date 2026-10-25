//go:build linux && cgo

package desktop

/*
#cgo pkg-config: x11
void blockX11Window(unsigned long window, int blocked);
void setX11ModalParent(unsigned long parent, unsigned long child);
*/
import "C"

import "fyne.io/fyne/v2/driver"

func SetWindowBlocked(context any, blocked bool) {
	if window, ok := context.(driver.X11WindowContext); ok && window.WindowHandle != 0 {
		value := C.int(0)
		if blocked {
			value = 1
		}
		C.blockX11Window(C.ulong(window.WindowHandle), value)
	}
}

func SetModalParent(parent, child any) {
	p, parentOK := parent.(driver.X11WindowContext)
	c, childOK := child.(driver.X11WindowContext)
	if parentOK && childOK && p.WindowHandle != 0 && c.WindowHandle != 0 {
		C.setX11ModalParent(C.ulong(p.WindowHandle), C.ulong(c.WindowHandle))
	}
	// ponytail: Fyne's Wayland context exposes no xdg_toplevel; retain input overlay
	// there until its driver exposes transient/modal relationships.
}
