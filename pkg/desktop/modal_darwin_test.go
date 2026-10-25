//go:build darwin && cgo

package desktop

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeModalActivation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "modal-test")
	if output, err := exec.Command("clang", "-framework", "AppKit", "modal_darwin.m", "testdata/modal_darwin.m", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile native modal check: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native modal check: %v\n%s", err, output)
	}
}
