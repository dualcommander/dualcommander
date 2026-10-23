//go:build !windows

package commander

import (
	"bytes"

	"github.com/natefinch/atomic"
)

func writeSettingsData(path string, data []byte) error {
	return atomic.WriteFile(path, bytes.NewReader(data))
}
