//go:build !linux && !darwin && !windows

package fsops

import "errors"

func renameNoReplace(from, to string) error {
	return errors.New("safe batch rename is unsupported on this platform")
}
