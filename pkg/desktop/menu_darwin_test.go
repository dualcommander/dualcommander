//go:build darwin && cgo

package desktop

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestServicesMenuRemoval(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "menu-test")
	cmd := exec.Command("clang", "-framework", "AppKit", "menu_darwin.m", "testdata/menu_darwin.m", "-o", binary)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile native menu check: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native menu check: %v\n%s", err, output)
	}
}
