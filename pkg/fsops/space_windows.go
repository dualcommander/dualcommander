package fsops

import "golang.org/x/sys/windows"

// Space returns occupied and total bytes on the volume containing path.
func Space(path string) (used, total uint64, err error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var free uint64
	err = windows.GetDiskFreeSpaceEx(name, nil, &total, &free)
	if err != nil {
		return 0, 0, err
	}
	return total - free, total, nil
}
