package commander

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func dialogFocusables(w fyne.Window) []fyne.Focusable {
	w.Canvas().Unfocus()
	var controls []fyne.Focusable
	for {
		w.Canvas().FocusNext()
		f := w.Canvas().Focused()
		if f == nil || slices.Contains(controls, f) {
			return controls
		}
		controls = append(controls, f)
	}
}

type shiftApp struct{ fyne.App }
type shiftDriver struct{ fyne.Driver }

func (a shiftApp) Driver() fyne.Driver                      { return shiftDriver{a.App.Driver()} }
func (d shiftDriver) CurrentKeyModifiers() fyne.KeyModifier { return fyne.KeyModifierShift }

func TestDialogEscape(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	for _, tc := range []struct {
		title string
		open  func()
	}{
		{"About " + appName, u.about},
		{"Settings", u.settings},
		{"Copy 1 item(s) to", func() { u.copyOrMove(false) }},
		{"Move/Rename 1 item(s) to", func() { u.copyOrMove(true) }},
		{"Rename", u.rename},
		{"New file", u.newFile},
		{"New directory", u.mkdir},
		{"Move to " + trashName(), u.delete},
		{"Error", func() { u.showErr(errors.New("test error")) }},
		{"Find Files", func() { u.search() }},
	} {
		t.Run(tc.title, func(t *testing.T) {
			tc.open()
			w := windowByTitle(t, u.app, tc.title)
			focused := dialogFocusables(w)
			w.Canvas().Unfocus()
			w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
			if slices.Contains(u.app.Driver().AllWindows(), w) {
				t.Fatal("Escape without focus did not close window")
			}
			// Every focusable control must also handle Escape itself.
			for i := range focused {
				tc.open()
				current := windowByTitle(t, u.app, tc.title)
				controls := dialogFocusables(current)
				current.Canvas().Focus(controls[i])
				controls[i].TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				if slices.Contains(u.app.Driver().AllWindows(), current) {
					t.Fatalf("Escape with focus on %T did not close window", controls[i])
				}
			}
			if !hasWindow(u.app, appName) || !exists(filepath.Join(root, "src", "readme.md")) {
				t.Fatal("Escape closed main window or confirmed operation")
			}
		})
	}
}

func TestShiftF6Rename(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, shiftApp{test.NewTempApp(t)}, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "song.mp3")
	u.left.toggleMark()
	cursorTo(t, u.left, "readme.md")
	press(u, fyne.KeyF6)
	w := windowByTitle(t, u.app, "Rename")
	e := w.Canvas().Focused().(*escEntry)
	if e.Text != "readme.md" || hasWindow(u.app, "Move/Rename 1 item(s) to") {
		t.Fatal("Shift+F6 must rename cursor item, not marked files")
	}
	for _, invalid := range []string{"", "..", "../escaped", "song.mp3"} {
		e.SetText(invalid)
		e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		if !hasWindow(u.app, "Rename") || e.Validate() == nil {
			t.Fatalf("accepted invalid name %q", invalid)
		}
	}
	e.SetText("renamed.md")
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	data, err := os.ReadFile(filepath.Join(root, "src", "renamed.md"))
	if err != nil || len(data) == 0 || exists(filepath.Join(root, "src", "readme.md")) || !exists(filepath.Join(root, "src", "song.mp3")) || u.left.current().name != "renamed.md" {
		t.Fatalf("rename failed: %s %v", data, err)
	}
}

func TestShiftF4NewFile(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, shiftApp{test.NewTempApp(t)}, filepath.Join(root, "src"), filepath.Join(root, "dst"))
		for _, p := range []*pane{u.left, u.right} {
			u.activate(p)
			p.setBrief(brief)
			press(u, fyne.KeyF4)
			w := windowByTitle(t, u.app, "New file")
			e := w.Canvas().Focused().(*escEntry)
			for _, invalid := range []string{"", " ", ".", "..", "../escaped", "sub/file", `sub\file`, "C:escaped", "bad\x00name", p.current().name} {
				e.SetText(invalid)
				e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
				if !hasWindow(u.app, "New file") || e.Validate() == nil {
					t.Fatalf("accepted invalid name %q", invalid)
				}
			}
			e.SetText("new file 世界.txt")
			e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			info, err := os.Stat(filepath.Join(p.path, e.Text))
			if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
				t.Fatalf("empty file not created: %v", err)
			}
			if p.current().name != e.Text || u.active != p || u.win.Canvas().Focused() != p.focusable() {
				t.Fatal("new file not selected and focused in current pane")
			}
			// Exercise registered shortcut as well as focused-pane key handling.
			// Fyne's test canvas embeds the software canvas but hides TypedShortcut.
			canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
			canvas.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF4, Modifier: fyne.KeyModifierShift})
			w = windowByTitle(t, u.app, "New file")
			e = w.Canvas().Focused().(*escEntry)
			e.SetText("cancelled.txt")
			tapDialogButton(t, u, "Cancel")
			if exists(filepath.Join(p.path, "cancelled.txt")) {
				t.Fatal("Cancel created file")
			}
		}
		body, err := os.ReadFile(filepath.Join(root, "src", "readme.md"))
		if err != nil || string(body) != "hello" {
			t.Fatal("existing file changed")
		}
	}
}

func TestNewFileExistingDestination(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	u.newFile()
	w := windowByTitle(t, u.app, "New file")
	e := w.Canvas().Focused().(*escEntry)
	for _, name := range []string{"readme.md", "docs"} {
		e.SetText(name)
		tapDialogButton(t, u, "Create")
		if e.Validate() == nil || !hasWindow(u.app, "New file") {
			t.Fatal("existing destination accepted")
		}
	}
	w.Close()
	u.left.navigate(archiveFixture(t, root), "")
	waitJobs(t, u)
	u.newFile()
	if !hasWindow(u.app, "Error") || hasWindow(u.app, "New file") {
		t.Fatal("archive creation not rejected")
	}
}
