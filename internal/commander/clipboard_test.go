package commander

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNativeFileClipboard(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	var exported []string
	u.writeFileClipboard = func(paths []string) error { exported = slices.Clone(paths); return nil }
	cursorTo(t, u.left, "readme.md")
	u.captureFiles(false)
	waitJobs(t, u)
	if !slices.Equal(exported, []string{filepath.Join(root, "src/readme.md")}) {
		t.Fatalf("native export: %v", exported)
	}
	u.readFileClipboard = func(context.Context) ([]string, error) {
		return []string{filepath.Join(root, "src/song.mp3")}, nil
	}
	u.activate(u.right)
	u.pasteFiles()
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst/song.mp3")) || exists(filepath.Join(root, "dst/readme.md")) || !exists(filepath.Join(root, "src/song.mp3")) {
		t.Fatal("external clipboard must override stale copy and preserve source")
	}
	u.fileClipboard = nil
	u.right.navigate(filepath.Join(root, "src/docs"), "")
	u.pasteFiles()
	waitJobs(t, u)
	if !exists(filepath.Join(root, "src/docs/song.mp3")) {
		t.Fatal("external paste requires app-local clipboard")
	}
}

func TestNativeClipboardFailureAndLocalCut(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.readFileClipboard = func(context.Context) ([]string, error) { return nil, errors.New("clipboard unavailable") }
	u.pasteFiles()
	waitJobs(t, u)
	if !hasWindow(u.app, "Error") {
		t.Fatal("native clipboard read error hidden")
	}
	tapDialogButton(t, u, "OK")
	u.writeFileClipboard = func([]string) error { t.Error("cut exported as copy"); return nil }
	cursorTo(t, u.left, "readme.md")
	u.captureFiles(true)
	u.activate(u.right)
	u.pasteFiles()
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst/readme.md")) || exists(filepath.Join(root, "src/readme.md")) {
		t.Fatal("native clipboard interfered with local cut")
	}
}

func TestNativeClipboardCopyBeforeImmediatePaste(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	writing := make(chan struct{})
	release := make(chan struct{})
	u.writeFileClipboard = func([]string) error { close(writing); <-release; return nil }
	u.readFileClipboard = func(context.Context) ([]string, error) {
		select {
		case <-release:
			return []string{filepath.Join(root, "src/readme.md")}, nil
		default:
			return nil, errors.New("paste read clipboard before copy finished")
		}
	}
	cursorTo(t, u.left, "readme.md")
	u.captureFiles(false)
	<-writing
	u.activate(u.right)
	u.pasteFiles()
	// Complete write after paste was queued, before draining UI work.
	close(release)
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst/readme.md")) {
		t.Fatal("immediate paste lost new copy")
	}
}
