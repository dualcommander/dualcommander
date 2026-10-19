package fsops

import (
	"strings"

	"golang.org/x/sys/unix"
)

// Mounts lists mounted filesystem roots using the native mount table.
func Mounts() ([]string, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	stats := make([]unix.Statfs_t, n+16)
	n, err = unix.Getfsstat(stats, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, n)
	for _, stat := range stats[:min(n, len(stats))] {
		paths = append(paths, strings.TrimRight(string(stat.Mntonname[:]), "\x00"))
	}
	return paths, nil
}
