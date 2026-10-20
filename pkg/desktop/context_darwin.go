//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
#include <stdlib.h>
int showContextMenu(unsigned long window, const char *items, float x, float y);
char *chooseApplication(unsigned long window);
*/
import "C"

import (
	"encoding/json"
	"unsafe"
)

// ContextMenu blocks its worker until native menu closes; -1 means cancellation.
func ContextMenu(window uintptr, items []ContextItem, x, y float32) int {
	if window == 0 || len(items) == 0 {
		return -1
	}
	data, _ := json.Marshal(items)
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	return int(C.showContextMenu(C.ulong(window), text, C.float(x), C.float(y)))
}

// ChooseApplication runs an app-modal panel on AppKit's main thread.
// Call from a worker; an empty result means Cancel, Escape, or window close.
func ChooseApplication(window uintptr) string {
	if window == 0 {
		return ""
	}
	path := C.chooseApplication(C.ulong(window))
	if path == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(path))
	return C.GoString(path)
}
