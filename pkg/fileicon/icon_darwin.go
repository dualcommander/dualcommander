//go:build darwin && cgo

package fileicon

/*
#cgo LDFLAGS: -framework AppKit -framework UniformTypeIdentifiers -framework QuickLookThumbnailing
#include <stdlib.h>
void *fileIcon(const char *extension, int folder, size_t *length);
void *filePreview(const char *path, size_t *length);
*/
import "C"

import (
	"sync"
	"unsafe"
)

var cache sync.Map

// PNG returns the system icon for a file extension or folder, cached for this run.
func PNG(extension string, folder bool) []byte {
	key := extension
	if folder {
		key = "/"
	}
	if data, ok := cache.Load(key); ok {
		return data.([]byte)
	}
	ext := C.CString(extension)
	defer C.free(unsafe.Pointer(ext))
	var length C.size_t
	var dir C.int
	if folder {
		dir = 1
	}
	ptr := C.fileIcon(ext, dir, &length)
	defer C.free(ptr)
	data := C.GoBytes(ptr, C.int(length))
	cache.Store(key, data)
	return data
}

// Preview returns a Quick Look thumbnail, falling back to the file's Finder icon.
// Call off the UI thread; Quick Look may invoke an OS thumbnail provider.
func Preview(path string) []byte {
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	var length C.size_t
	ptr := C.filePreview(name, &length)
	defer C.free(ptr)
	return C.GoBytes(ptr, C.int(length))
}
