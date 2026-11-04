package commander

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/dualcommander/dualcommander/pkg/archive"
)

func openPack(t *testing.T, u *ui) fyne.Window {
	t.Helper()
	for _, item := range u.win.MainMenu().Items[0].Items {
		if item.Label == "Create Archive…" {
			item.Action()
			return windowByTitle(t, u.app, "Create Archive")
		}
	}
	t.Fatal("Create Archive menu missing")
	return nil
}

func packFields(w fyne.Window) (entries []*escEntry, format *dialogSelect) {
	for _, o := range test.LaidOutObjects(w.Content()) {
		switch v := o.(type) {
		case *escEntry:
			entries = append(entries, v)
		case *dialogSelect:
			format = v
		}
	}
	return
}

func TestCreateArchive(t *testing.T) {
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"} {
		t.Run(ext, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			press(u, fyne.KeySpace)
			cursorTo(t, u.left, "photos")
			press(u, fyne.KeySpace)
			cursorTo(t, u.left, "docs")
			press(u, fyne.KeySpace)
			w := openPack(t, u)
			entries, format := packFields(w)
			if len(entries) != 3 || format == nil || !entries[1].Password || !entries[2].Password {
				t.Fatal("name, format, masked password and confirmation required")
			}
			entries[1].SetText("discard")
			entries[2].SetText("discard")
			format.SetSelected(".tar")
			if entries[1].Text != "" || entries[2].Text != "" {
				t.Fatal("format change retained password")
			}
			format.SetSelected(ext)
			if ext == ".zip" {
				entries[1].SetText("secret")
				entries[2].SetText("wrong")
				tapDialogButton(t, u, "Create")
				if !hasWindow(u.app, "Create Archive") || len(u.jobs.active) != 0 {
					t.Fatal("mismatched passwords accepted")
				}
				entries[2].SetText("secret")
			} else if !entries[1].Disabled() || !entries[2].Disabled() {
				t.Fatal("unsupported encryption enabled")
			}
			entries[0].SetText("backup" + ext)
			tapDialogButton(t, u, "Create")
			waitJobs(t, u)
			file := filepath.Join(root, "dst", "backup"+ext)
			if !exists(file) || !slices.Contains(names(u.right), "backup"+ext) || u.passwordWindow != nil {
				t.Fatal("archive not created/refreshed or parent blocked")
			}
			if entries[1].Text != "" || entries[2].Text != "" {
				t.Fatal("password retained in dialog")
			}
			dst := t.TempDir()
			if err := archive.Unpack(context.Background(), file, dst, nil, "secret"); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(filepath.Join(dst, "backup", "docs")); err != nil || !info.IsDir() {
				t.Fatalf("empty directory lost: %v", err)
			}
			for name, want := range map[string]string{"readme.md": "hello", "photos/a.png": "png"} {
				body, err := os.ReadFile(filepath.Join(dst, "backup", name))
				if err != nil || string(body) != want {
					t.Fatalf("roundtrip %s: %q %v", name, body, err)
				}
			}
		})
	}
}

func TestCreateArchiveCancel(t *testing.T) {
	for _, control := range []string{"name", "format", "password", "confirmation", "Create", "Cancel", "canvas", "close", "cancel button"} {
		t.Run(control, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			w := openPack(t, u)
			entries, format := packFields(w)
			entries[1].SetText("secret")
			before := u.left.cursor
			press(u, fyne.KeyDown)
			if u.left.cursor != before || u.passwordWindow == nil {
				t.Fatal("parent not blocked")
			}
			var focus fyne.Focusable
			switch control {
			case "name":
				focus = entries[0]
			case "format":
				focus = format
			case "password":
				focus = entries[1]
			case "confirmation":
				focus = entries[2]
			case "Create", "Cancel":
				for _, o := range test.LaidOutObjects(w.Content()) {
					if b, ok := o.(*dialogButton); ok && b.Text == control {
						focus = b
					}
				}
			case "close":
				w.Close()
			case "cancel button":
				tapDialogButton(t, u, "Cancel")
			case "canvas":
				w.Canvas().Unfocus()
				w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
			}
			if focus != nil {
				w.Canvas().Focus(focus)
				focus.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			}
			files, err := os.ReadDir(filepath.Join(root, "dst"))
			if err != nil || len(files) != 0 || len(u.jobs.active) != 0 || u.passwordWindow != nil || hasWindow(u.app, "Create Archive") || entries[1].Text != "" {
				t.Fatal("cancel left output, work, blocker or password")
			}
			if u.win.Canvas().Focused() != u.left.focusable() {
				t.Fatal("focus not restored")
			}
		})
	}
}
