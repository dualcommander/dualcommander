//go:build darwin && cgo

package desktop

import (
	"testing"

	"github.com/dualcommander/dualcommander/assets"
)

func TestRunningAppIcon(t *testing.T) {
	if SetAppIcon(nil) || SetAppIcon([]byte("not an image")) {
		t.Fatal("invalid icon accepted")
	}
	if !SetAppIcon(assets.IconPNG) {
		t.Fatal("embedded app icon rejected by AppKit")
	}
}
