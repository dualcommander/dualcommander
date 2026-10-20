package commander

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

func protectedArchive(t *testing.T, root, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "pkg", "archive", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "src", name)
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestArchivePasswordRetry(t *testing.T) {
	for _, name := range []string{"password.zip", "password.7z", "password-headers.7z"} {
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			file := protectedArchive(t, root, name)
			a := &brandingApp{App: test.NewTempApp(t)}
			u := newTestUI(t, a, filepath.Join(root, "src"), root)
			cursorTo(t, u.left, name)
			press(u, fyne.KeyReturn)
			if name != "password-headers.7z" {
				waitJobs(t, u)
				cursorTo(t, u.left, "secret.txt")
				press(u, fyne.KeyReturn)
			}
			waitUntil(t, func() bool { return hasWindow(a, "Archive password") })
			w := windowByTitle(t, a, "Archive password")
			entry := w.Canvas().Focused().(*escEntry)
			if !entry.Password || u.win.Canvas().Overlays().Top() == nil {
				t.Fatal("password must be masked and parent modal")
			}
			assertUIText(t, w.Content(), "Enter password for "+name)
			var form *widget.Form
			for _, object := range test.LaidOutObjects(w.Content()) {
				if f, ok := object.(*widget.Form); ok {
					form = f
				}
			}
			if form == nil || len(form.Items) != 1 || form.Items[0].Text != "Password" {
				t.Fatal("password field missing accessible label")
			}
			path := u.left.path
			press(u, fyne.KeyBackspace)
			if u.left.path != path {
				t.Fatal("parent accepts input during password prompt")
			}
			entry.SetText("wrong")
			entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			waitUntil(t, func() bool { return hasWindow(a, "Archive password") })
			w = windowByTitle(t, a, "Archive password")
			assertUIText(t, w.Content(), "Password incorrect, or encrypted data damaged. Try again.")
			entry = w.Canvas().Focused().(*escEntry)
			if entry.Text != "" {
				t.Fatal("retry retained password")
			}
			entry.SetText("secret")
			tapDialogButton(t, u, "Unlock")
			waitJobs(t, u)
			if name == "password-headers.7z" {
				if u.left.path != file {
					t.Fatal("unlocked archive not entered")
				}
				cursorTo(t, u.left, "secret.txt")
				press(u, fyne.KeyReturn)
				waitJobs(t, u)
			}
			if hasWindow(a, "Error") || u.passwordWindow != nil || u.win.Canvas().Overlays().Top() != nil || a.openedURL == nil {
				t.Fatal("unlock did not finish cleanly")
			}
			body, err := os.ReadFile(filepath.FromSlash(a.openedURL.Path))
			if err != nil || string(body) != "secret contents" {
				t.Fatalf("extracted content: %q %v", body, err)
			}
			if u.win.Canvas().Focused() != u.left.focusable() {
				t.Fatal("pane focus not restored")
			}
			for _, dir := range u.archiveTemps {
				os.RemoveAll(dir)
			}
		})
	}
}

func TestArchivePasswordCancel(t *testing.T) {
	for _, close := range []string{"Cancel", "Escape", "close", "job"} {
		t.Run(close, func(t *testing.T) {
			root := fixture(t)
			file := protectedArchive(t, root, "password-headers.7z")
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			u.jobs.run("Password test", nil, func(context.Context, fsops.Progress) error {
				_, err := u.archivePassword(ctx, file, false)
				return err
			})
			waitUntil(t, func() bool { return hasWindow(u.app, "Archive password") })
			w := windowByTitle(t, u.app, "Archive password")
			switch close {
			case "Cancel":
				tapDialogButton(t, u, "Cancel")
			case "Escape":
				w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			case "close":
				w.Close()
			case "job":
				cancel()
			}
			waitJobs(t, u)
			if hasWindow(u.app, "Error") || hasWindow(u.app, "Archive password") || u.win.Canvas().Overlays().Top() != nil {
				t.Fatal("cancel left dialog or error")
			}
		})
	}
}

func TestRenderGoldenArchivePassword(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "archive-password.png"
		if retry {
			name = "archive-password-retry.png"
		}
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			a := test.NewTempApp(t)
			a.Settings().SetTheme(theme.LightTheme())
			u := newTestUI(t, a, filepath.Join(root, "src"), root)
			u.jobs.run("Password render", nil, func(ctx context.Context, _ fsops.Progress) error {
				_, err := u.archivePassword(ctx, "protected.zip", retry)
				return err
			})
			waitUntil(t, func() bool { return hasWindow(a, "Archive password") })
			w := windowByTitle(t, a, "Archive password")
			w.Canvas().Unfocus() // no blinking caret in golden
			test.AssertRendersToImage(t, name, w.Canvas())
			w.Close()
			waitJobs(t, u)
		})
	}
}
