//go:build darwin || linux || windows

package fsops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMounts(t *testing.T) {
	paths, err := Mounts()
	if err != nil || len(paths) == 0 {
		t.Fatalf("mount list: %v, %v", paths, err)
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			t.Fatalf("relative mount: %q", path)
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			t.Fatalf("mount not a directory: %q", path)
		}
	}
}

func TestSpace(t *testing.T) {
	path := t.TempDir()
	used, total, err := Space(path)
	if err != nil || total == 0 || used > total {
		t.Fatalf("invalid volume usage: %d/%d: %v", used, total, err)
	}
	if _, _, err := Space(filepath.Join(path, "missing")); err == nil {
		t.Fatal("missing path accepted")
	}
}
