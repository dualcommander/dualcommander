package commander

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func archiveFixture(t *testing.T, root string) string {
	t.Helper()
	name := filepath.Join(root, "src", "browse.zip")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "docs/readme.txt", Method: zip.Deflate}
	h.SetMode(0o644)
	h.SetModTime(time.Date(2024, 1, 2, 3, 4, 0, 0, time.UTC))
	out, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write([]byte("archive contents")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestArchiveNavigation(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "browse.zip")
	press(u, fyne.KeyReturn)
	waitJobs(t, u)
	if u.left.path != name || !reflect.DeepEqual(names(u.left), []string{"..", "docs"}) {
		t.Fatalf("archive root: %s %v", u.left.path, names(u.left))
	}
	cursorTo(t, u.left, "docs")
	// Double-click follows the same navigation path as a real row.
	r := newRowWidget(u.left, false)
	r.id = u.left.cursor
	r.DoubleTapped(nil)
	waitJobs(t, u)
	if !reflect.DeepEqual(names(u.left), []string{"..", "readme.txt"}) {
		t.Fatalf("archive subfolder: %v", names(u.left))
	}
	u.left.refresh()
	cursorTo(t, u.left, "readme.txt")
	press(u, fyne.KeyInsert)
	if !u.left.marked["readme.txt"] {
		t.Fatal("archive selection not marked")
	}
	u.save()
	if got := a.Preferences().String(prefLeft); got != filepath.Join(root, "src") {
		t.Fatalf("saved virtual path instead of containing folder: %s", got)
	}
	for _, key := range []fyne.KeyName{fyne.KeyF4, fyne.KeyF6, fyne.KeyF7, fyne.KeyF8} {
		press(u, key)
		if !hasWindow(u.app, "Error") {
			t.Fatalf("%s did not report read-only archive", key)
		}
		tapDialogButton(t, u, "OK")
	}
	if _, err := u.externalCommand(true); err == nil {
		t.Fatal("terminal accepted virtual folder")
	}
	press(u, fyne.KeyBackspace)
	waitJobs(t, u)
	if u.left.path != name || u.left.current().name != "docs" || len(u.left.marked) != 0 {
		t.Fatal("parent navigation lost cursor or marks")
	}
	cursorTo(t, u.left, "..")
	press(u, fyne.KeyReturn)
	if u.left.path != filepath.Join(root, "src") || u.left.current().name != "browse.zip" {
		t.Fatal("leaving archive did not select archive file")
	}
	if u.win.Canvas().Focused() != u.left.focusable() {
		t.Fatal("archive navigation lost pane focus")
	}
}

func TestCopyArchiveSelection(t *testing.T) {
	for _, folder := range []bool{false, true} {
		t.Run(fmt.Sprint(folder), func(t *testing.T) {
			root := fixture(t)
			name := archiveFixture(t, root)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			from, selected, output := name, "docs", "docs/readme.txt"
			if !folder {
				from, selected, output = filepath.Join(name, "docs"), "readme.txt", "readme.txt"
			}
			u.left.navigate(from, selected)
			waitJobs(t, u)
			u.left.markAll(true)
			press(u, fyne.KeyF5)
			if !hasWindow(u.app, "Copy 1 item(s) to") {
				t.Fatal("archive copy must offer normal target dialog")
			}
			tapDialogButton(t, u, "Copy")
			waitJobs(t, u)
			body, err := os.ReadFile(filepath.Join(root, "dst", output))
			if err != nil || string(body) != "archive contents" {
				t.Fatalf("selected archive output: %q, %v", body, err)
			}
			if u.left.path != from || len(u.left.marked) != 0 || u.win.Canvas().Focused() != u.left.focusable() || !exists(name) {
				t.Fatal("copy changed source, marks, focus, or archive")
			}
			if !slices.Contains(names(u.right), strings.Split(output, "/")[0]) {
				t.Fatal("destination pane not refreshed")
			}
		})
	}
}

func TestCopyEncryptedArchive(t *testing.T) {
	for _, action := range []string{"Unlock", "Cancel", "Escape", "Close"} {
		t.Run(action, func(t *testing.T) {
			root := fixture(t)
			file := protectedArchive(t, root, "password.zip")
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.left.navigate(file, "secret.txt")
			waitJobs(t, u)
			press(u, fyne.KeyF5)
			tapDialogButton(t, u, "Copy")
			waitUntil(t, func() bool { return hasWindow(u.app, "Archive password") })
			w := windowByTitle(t, u.app, "Archive password")
			switch action {
			case "Unlock":
				w.Canvas().Focused().(*escEntry).SetText("secret")
				tapDialogButton(t, u, "Unlock")
			case "Close":
				w.Close()
			case "Escape":
				w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			default:
				tapDialogButton(t, u, "Cancel")
			}
			waitJobs(t, u)
			if exists(filepath.Join(root, "dst/secret.txt")) != (action == "Unlock") || hasWindow(u.app, "Error") || hasWindow(u.app, "Archive password") || u.win.Canvas().Overlays().Top() != nil {
				t.Fatal("password workflow left unexpected output, prompt, or blocker")
			}
		})
	}
}

func TestArchiveCopyTargetCancellationAndWriteGuard(t *testing.T) {
	for _, action := range []string{"Cancel", "Escape", "Close"} {
		t.Run(action, func(t *testing.T) {
			root := fixture(t)
			name := archiveFixture(t, root)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.left.navigate(name, "docs")
			waitJobs(t, u)
			press(u, fyne.KeyF5)
			w := windowByTitle(t, u.app, "Copy 1 item(s) to")
			before := u.left.path
			press(u, fyne.KeyBackspace)
			if u.left.path != before {
				t.Fatal("target dialog did not block parent")
			}
			switch action {
			case "Close":
				w.Close()
			case "Escape":
				w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			default:
				tapDialogButton(t, u, "Cancel")
			}
			waitJobs(t, u)
			if exists(filepath.Join(root, "dst/docs")) || u.win.Canvas().Overlays().Top() != nil {
				t.Fatal("cancelled target created output or retained blocker")
			}
			u.activate(u.right)
			cursorTo(t, u.right, "..")
			press(u, fyne.KeyF5)
			if !hasWindow(u.app, "Error") {
				t.Fatal("copy into archive allowed")
			}
		})
	}
}

func TestCopyArchiveConflictAndCancel(t *testing.T) {
	for _, choice := range []string{"Skip", "Overwrite", "Cancel", "Escape", "Close"} {
		t.Run(choice, func(t *testing.T) {
			root := fixture(t)
			name := archiveFixture(t, root)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.left.navigate(filepath.Join(name, "docs"), "readme.txt")
			waitJobs(t, u)
			target := filepath.Join(root, "dst/readme.txt")
			if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			press(u, fyne.KeyF5)
			if !hasWindow(u.app, "Copy 1 item(s) to") {
				t.Fatal("copy dialog missing")
			}
			tapDialogButton(t, u, "Copy")
			waitUntil(t, func() bool { return hasWindow(u.app, "Target already exists") })
			w := windowByTitle(t, u.app, "Target already exists")
			switch choice {
			case "Close":
				w.Close()
			case "Escape":
				w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			default:
				tapDialogButton(t, u, choice)
			}
			waitJobs(t, u)
			want := "keep"
			if choice == "Overwrite" {
				want = "archive contents"
			}
			body, err := os.ReadFile(target)
			if err != nil || string(body) != want || hasWindow(u.app, "Error") {
				t.Fatalf("conflict %s: %q, %v", choice, body, err)
			}
		})
	}
}

func TestArchiveFailureAndStaleNavigation(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	cursorTo(t, u.left, "app.zip") // deliberately corrupt fixture
	press(u, fyne.KeyReturn)
	waitJobs(t, u)
	if u.left.path != filepath.Join(root, "src") || !hasWindow(u.app, "Error") {
		t.Fatal("corrupt archive changed folder or hid error")
	}
	tapDialogButton(t, u, "OK")
	u.left.navigate(name, "")
	u.left.navigate(root, "")
	waitJobs(t, u)
	if u.left.path != root {
		t.Fatal("stale archive job replaced newer navigation")
	}
}

func TestArchiveUIContent(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	applyTheme(u.app, "dark", u.density)
	u.left.navigate(filepath.Join(name, "docs"), "readme.txt")
	waitJobs(t, u)
	// Parent metadata on disk is irrelevant to virtual-folder rendering.
	u.left.entries[0].attrs = "drwxr-xr-x"
	u.left.entries[0].mod = u.left.entries[1].mod
	u.left.list.Refresh()
	w := test.NewWindow(u.left.body)
	defer w.Close()
	w.Resize(fyne.NewSize(480, 180))
	assertUIText(t, w.Content(), "..", "readme.txt", "16 B", "-rw-r--r--")
	assertPaneRows(t, u.left)
	if u.left.current().name != "readme.txt" || !u.left.favourite.Disabled() {
		t.Fatal("archive cursor or read-only favourite state incorrect")
	}
}
