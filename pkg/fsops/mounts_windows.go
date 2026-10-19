package fsops

import "golang.org/x/sys/windows"

// Mounts lists drive roots using the native Windows API.
func Mounts() ([]string, error) {
	n, err := windows.GetLogicalDriveStrings(0, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]uint16, n+1)
	if _, err = windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0]); err != nil {
		return nil, err
	}
	var paths []string
	start := 0
	for i, c := range buf {
		if c == 0 {
			if i == start {
				break
			}
			paths = append(paths, windows.UTF16ToString(buf[start:i]))
			start = i + 1
		}
	}
	return paths, nil
}
