//go:build !darwin && !linux && !windows

package fsops

import "fmt"

func Space(path string) (used, total uint64, err error) {
	return 0, 0, fmt.Errorf("disk usage unavailable on this platform")
}

func Mounts() ([]string, error) {
	return nil, fmt.Errorf("mount list unavailable on this platform")
}
