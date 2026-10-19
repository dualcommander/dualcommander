//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stddef.h>
int setAppIcon(const void *data, size_t length);
*/
import "C"

import "unsafe"

// SetAppIcon sets the running application's Dock icon, including unpackaged runs.
func SetAppIcon(png []byte) bool {
	if len(png) == 0 {
		return false
	}
	return C.setAppIcon(unsafe.Pointer(&png[0]), C.size_t(len(png))) != 0
}
