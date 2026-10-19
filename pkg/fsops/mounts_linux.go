package fsops

import (
	"bufio"
	"os"
	"slices"
	"strings"
)

// Mounts reads the current process's native mount table.
func Mounts() ([]string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var paths []string
	unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) < 6 {
			continue
		}
		path := unescape.Replace(fields[4])
		if info, err := os.Stat(path); err == nil && info.IsDir() && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths, scan.Err()
}
