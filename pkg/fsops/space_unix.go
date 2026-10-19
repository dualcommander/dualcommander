//go:build darwin || linux

package fsops

import "golang.org/x/sys/unix"

// Space returns occupied and total bytes on the volume containing path.
func Space(path string) (used, total uint64, err error) {
	var stat unix.Statfs_t
	if err = unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	return (stat.Blocks - stat.Bfree) * uint64(stat.Bsize), total, nil
}
