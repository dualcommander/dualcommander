//go:build !windows

package commander

import "os"

func openSettingsRead(path string) (*os.File, error) { return os.Open(path) }
