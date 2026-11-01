//go:build darwin && cgo

package fileicon

import (
	"os"
	"path/filepath"
	"testing"
)

// Go tests have no AppKit application or main event loop. Native lookups must
// leave callers to their fallback instead of waiting for AppKit services.
func TestHeadlessSystemIcons(t *testing.T) {
	for _, tc := range []struct {
		extension string
		folder    bool
	}{{"", true}, {"", false}, {"txt", false}, {"zip", false}, {"unknown-extension", false}} {
		for range 2 {
			if data := PNG(tc.extension, tc.folder); len(data) != 0 {
				t.Fatalf("extension=%q folder=%v: headless lookup returned native icon", tc.extension, tc.folder)
			}
		}
	}
}

func TestHeadlessFilePreview(t *testing.T) {
	name := filepath.Join(t.TempDir(), "preview.txt")
	if err := os.WriteFile(name, []byte("preview"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{name, filepath.Join(t.TempDir(), "missing.txt")} {
		if data := Preview(path); len(data) != 0 {
			t.Fatalf("%q: headless lookup returned native preview", path)
		}
	}
}
