//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
int startFileDrag(unsigned long window, const char *json);
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// StartFileDrag hands file references to AppKit; the destination performs copying.
func StartFileDrag(window fyne.Window, paths []string) bool {
	native, ok := window.(driver.NativeWindow)
	if !ok || len(paths) == 0 {
		return false
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return false
	}
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	started := false
	native.RunNative(func(context any) {
		if mac, ok := context.(driver.MacWindowContext); ok {
			started = C.startFileDrag(C.ulong(mac.NSWindow), text) != 0
		}
	})
	return started
}
