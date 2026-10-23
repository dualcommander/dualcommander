package commander

import (
	"bytes"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/natefinch/atomic"
	"golang.org/x/sys/windows"
)

var replaceSettingsFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func writeSettingsAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Close(); err != nil {
		return err
	}
	if err := atomic.WriteFile(f.Name(), bytes.NewReader(data)); err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	src, err := windows.UTF16PtrFromString(f.Name())
	if err != nil {
		return err
	}
	// MoveFileEx cannot replace an open destination; ReplaceFile supports shared readers.
	ok, _, err := replaceSettingsFile.Call(uintptr(unsafe.Pointer(dst)), uintptr(unsafe.Pointer(src)), 0, 0, 0, 0)
	if ok != 0 {
		return nil
	}
	if err == windows.ERROR_FILE_NOT_FOUND {
		return atomic.ReplaceFile(f.Name(), path) // First save has no destination to replace.
	}
	return &os.LinkError{Op: "replace", Old: f.Name(), New: path, Err: err}
}
