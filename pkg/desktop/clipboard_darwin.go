//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
int copyFileURLs(const char *json);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// CopyFiles publishes file references for pasting into Finder, not file contents.
func CopyFiles(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	if C.copyFileURLs(text) == 0 {
		return fmt.Errorf("could not write file URLs to macOS clipboard")
	}
	return nil
}
