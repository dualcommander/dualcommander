package fsops

import "golang.org/x/sys/windows"

func renameNoReplace(from, to string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFile(src, dst)
}
