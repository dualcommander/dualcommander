package commander

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func historyShortcut(u *ui, key fyne.KeyName) {
	canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
	canvas.TypedShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierShortcutDefault})
}

func TestFolderHistory(t *testing.T) {
	root := fixture(t)
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	u := newTestUI(t, test.NewTempApp(t), src, dst)
	p := u.left
	cursorTo(t, p, "photos")
	press(u, fyne.KeyReturn)
	p.refresh() // Refresh must not add history.
	historyShortcut(u, fyne.KeyLeft)
	if p.path != src || p.current().name != "photos" || u.right.path != dst {
		t.Fatalf("back lost folder, cursor, or pane isolation: %s %s", p.path, p.current().name)
	}
	historyShortcut(u, fyne.KeyLeft) // Start boundary.
	if p.path != src {
		t.Fatal("back moved past initial folder")
	}
	historyShortcut(u, fyne.KeyRight)
	if p.path != filepath.Join(src, "photos") {
		t.Fatal("forward did not restore folder")
	}
	historyShortcut(u, fyne.KeyRight) // End boundary.
	historyShortcut(u, fyne.KeyLeft)
	cursorTo(t, p, "docs")
	press(u, fyne.KeyReturn)
	historyShortcut(u, fyne.KeyRight)
	if p.path != filepath.Join(src, "docs") {
		t.Fatal("new navigation retained old forward branch")
	}
	u.activate(u.right)
	u.right.navigate(root, "")
	historyShortcut(u, fyne.KeyLeft)
	if u.right.path != dst || p.path != filepath.Join(src, "docs") {
		t.Fatal("right history changed left pane")
	}
	if u.win.Canvas().Focused() != u.right.focusable() {
		t.Fatal("history lost pane focus")
	}
}

func TestFolderHistoryFailedNavigation(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	p := u.left
	docs := filepath.Join(src, "docs")
	p.navigate(docs, "")
	p.navigate(filepath.Join(src, "photos"), "")
	if err := os.Remove(docs); err != nil {
		t.Fatal(err)
	}
	historyShortcut(u, fyne.KeyLeft)
	if p.path != filepath.Join(src, "photos") || !hasWindow(u.app, "Error") {
		t.Fatal("missing history target changed folder or hid error")
	}
	tapDialogButton(t, u, "OK")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	historyShortcut(u, fyne.KeyLeft)
	if p.path != docs {
		t.Fatal("failed history navigation consumed entry")
	}
	p.navigate(filepath.Join(root, "missing"), "")
	tapDialogButton(t, u, "OK")
	historyShortcut(u, fyne.KeyRight)
	if p.path != filepath.Join(src, "photos") {
		t.Fatal("failed normal navigation discarded forward history")
	}
}

func TestFolderHistoryArchives(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	src := filepath.Join(root, "src")
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	u.left.navigate(name, "")
	waitJobs(t, u)
	u.left.navigate(filepath.Join(name, "docs"), "")
	historyShortcut(u, fyne.KeyLeft)
	waitJobs(t, u)
	if u.left.path != name {
		t.Fatal("back did not restore archive root")
	}
	historyShortcut(u, fyne.KeyLeft)
	waitJobs(t, u)
	if u.left.path != src {
		t.Fatal("back did not leave archive")
	}
	historyShortcut(u, fyne.KeyRight)
	waitJobs(t, u)
	historyShortcut(u, fyne.KeyRight)
	waitJobs(t, u)
	if u.left.path != filepath.Join(name, "docs") {
		t.Fatal("forward did not restore archive subfolder")
	}
}
