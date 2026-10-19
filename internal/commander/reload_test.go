package commander

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestReloadActionActivePane(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	// No watcher: shortcut itself must reload, not a delayed filesystem event.
	u.left.watcher.Close()
	u.left.watcher = nil
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"] = true
	if err := os.WriteFile(filepath.Join(u.left.path, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	rightGeneration := u.right.iconGeneration
	for _, menu := range u.win.MainMenu().Items {
		for _, item := range menu.Items {
			if item.Label == "Refresh" {
				if item.Shortcut == nil {
					t.Fatal("refresh shortcut missing")
				}
				item.Action()
			}
		}
	}
	if !slices.Contains(names(u.left), "new.txt") || u.right.iconGeneration != rightGeneration {
		t.Fatal("Ctrl+R did not reload only active pane")
	}
	if u.left.current().name != "readme.md" || !u.left.marked["readme.md"] {
		t.Fatal("reload lost cursor or marks")
	}
}

func TestNativeWatchFileWriteAndRename(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	p := u.left
	cursorTo(t, p, "readme.md")
	p.marked["readme.md"] = true
	path := filepath.Join(p.path, "readme.md")
	if err := os.WriteFile(path, []byte("changed content"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return p.current().size == int64(len("changed content")) })
	if !p.marked["readme.md"] {
		t.Fatal("auto-reload lost mark")
	}
	if err := os.Rename(path, filepath.Join(p.path, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		return slices.Contains(names(p), "renamed.md") && !slices.Contains(names(p), "readme.md")
	})
	p.cd(filepath.Join(root, "dst"), "")
	path = filepath.Join(p.path, "created.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return slices.Contains(names(p), "created.txt") })
	cursorTo(t, p, "created.txt")
	if err := os.WriteFile(path, []byte("longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return p.current().size == 6 })
}
