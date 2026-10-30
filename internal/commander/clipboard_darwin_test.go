//go:build darwin && cgo

package commander

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestCopyFilesPublishesFinderURLs(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	cursorTo(t, u.active, "readme.md")
	u.active.markAll(true)
	want := u.active.selection()
	u.captureFiles(false)
	// Read native pasteboard independently, as another desktop app would.
	output, err := exec.Command("swift", "-e", `import AppKit
let urls = NSPasteboard.general.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL] ?? []
let data = try JSONSerialization.data(withJSONObject: urls.map { $0.path })
print(String(data: data, encoding: .utf8)!)`).CombinedOutput()
	if err != nil {
		t.Fatalf("read native clipboard: %v: %s", err, output)
	}
	var got []string
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("clipboard output: %v: %s", err, output)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Finder file URLs: got %v, want %v", got, want)
	}
}
