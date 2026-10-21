package commander

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type inputReadyApp struct {
	fyne.App
	t *testing.T
}

func (a *inputReadyApp) NewWindow(title string) fyne.Window {
	if title != "New directory" {
		return a.App.NewWindow(title)
	}
	return &inputReadyWindow{Window: a.App.NewWindow(title), t: a.t}
}

type inputReadyWindow struct {
	fyne.Window
	t *testing.T
}

func (w *inputReadyWindow) SetIcon(fyne.Resource) {
	w.t.Error("dialog must inherit app icon without setting it again")
}

func (w *inputReadyWindow) Show() {
	if _, ok := w.Canvas().Focused().(*escEntry); !ok {
		w.t.Fatal("input must have focus before native window is shown")
	}
	w.Window.Show()
}

func TestNewDirectoryReadyForTyping(t *testing.T) {
	for _, cancel := range []string{"submit", "escape", "close", "cancel"} {
		t.Run(cancel, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), root, root)
			t.Cleanup(func() {
				for _, p := range []*pane{u.left, u.right} {
					if p.watcher != nil {
						p.watcher.Close()
						p.watcher = nil
					}
				}
			})
			u.app = &inputReadyApp{App: u.app, t: t}
			press(u, fyne.KeyF7)
			w := windowByTitle(t, u.app, "New directory")
			e := w.Canvas().Focused().(*escEntry)
			test.Type(e, "new-folder")
			switch cancel {
			case "submit":
				e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			case "escape":
				e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			case "close":
				w.Close()
			case "cancel":
				tapDialogButton(t, u, "Cancel")
			}
			if exists(filepath.Join(root, "new-folder")) != (cancel == "submit") || hasWindow(u.app, "New directory") {
				t.Fatal("early typing or dialog completion lost")
			}
			if u.win.Canvas().Focused() != u.active.focusable() {
				t.Fatal("pane focus not restored")
			}
		})
	}
}

func TestInputDialogReusesNativeWindow(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	var native fyne.Window
	for _, action := range []string{"escape", "cancel", "native close", "submit"} {
		u.mkdir()
		w := windowByTitle(t, u.app, "New directory")
		if native == nil {
			native = w
		} else if native != w {
			t.Fatal("input dialog recreated native window")
		}
		e := w.Canvas().Focused().(*escEntry)
		if e.Text != "" || u.passwordWindow != u.inputWindow {
			t.Fatal("stale input or parent not blocked")
		}
		test.Type(e, "created-"+action)
		switch action {
		case "escape":
			e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		case "cancel":
			tapDialogButton(t, u, "Cancel")
		case "native close":
			u.inputWindow.Window.Close() // headless driver delivers OnClosed directly
		case "submit":
			e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		}
		if exists(filepath.Join(root, "created-"+action)) != (action == "submit") {
			t.Fatal("incorrect confirmation/cancellation")
		}
		if u.passwordWindow != nil || u.win.Canvas().Overlays().Top() != nil || hasWindow(u.app, "New directory") {
			t.Fatal("dialog not dismissed or parent still blocked")
		}
		if action == "native close" {
			native = nil
		}
	}
	u.rename()
	if u.inputWindow != nil && u.inputWindow.open {
		u.inputWindow.Close()
	}
}

func TestDialogCenteredDefaultButtons(t *testing.T) {
	for _, key := range []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter} {
		for _, focus := range []string{"field", "default", "cancel", "canvas"} {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			u.copyOrMove(false)
			w := windowByTitle(t, u.app, "Copy 1 item(s) to")
			var primary, cancel *dialogButton
			for _, o := range test.LaidOutObjects(w.Content()) {
				if b, ok := o.(*dialogButton); ok {
					if b.Text == "Copy" {
						primary = b
					} else if b.Text == "Cancel" {
						cancel = b
					}
				}
			}
			if primary == nil || cancel == nil || primary.Importance != widget.HighImportance || cancel.Importance == widget.HighImportance {
				t.Fatal("exactly one default action required")
			}
			outer := w.Content().(*container.ThemeOverride).Content.(*fyne.Container)
			body := outer.Objects[0].(*fyne.Container)
			center := body.Objects[1].(*fyne.Container)
			group := center.Objects[0]
			if delta := 2*group.Position().X + group.Size().Width - center.Size().Width; delta < -0.01 || delta > 0.01 {
				t.Fatal("button group not centered")
			}
			switch focus {
			case "default":
				w.Canvas().Focus(primary)
			case "cancel":
				w.Canvas().Focus(cancel)
			case "canvas":
				w.Canvas().Unfocus()
			}
			if focused := w.Canvas().Focused(); focused != nil {
				focused.TypedKey(&fyne.KeyEvent{Name: key})
			} else {
				w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: key})
			}
			waitJobs(t, u)
			if exists(filepath.Join(root, "dst", "readme.md")) != (focus != "cancel") || hasWindow(u.app, "Copy 1 item(s) to") {
				t.Fatalf("Enter action wrong: focus=%s key=%s", focus, key)
			}
		}
	}
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	confirmed := false
	u.confirm("Confirm", "Continue?", func() { confirmed = true })
	w := windowByTitle(t, u.app, "Confirm")
	if b, ok := w.Canvas().Focused().(*dialogButton); !ok || b.Text != "Cancel" {
		t.Fatal("confirmation must default to safe action")
	}
	w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if confirmed || hasWindow(u.app, "Confirm") {
		t.Fatal("default confirmation must cancel")
	}
	u.showErr(errors.New("test"))
	w = windowByTitle(t, u.app, "Error")
	w.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if hasWindow(u.app, "Error") {
		t.Fatal("Enter must dismiss error")
	}
}

func TestFileDialogSpacing(t *testing.T) {
	for _, density := range densities {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
		u.setDensity(density)
		cursorTo(t, u.left, "readme.md")
		panePadding := u.left.list.Theme().Size(theme.SizeNamePadding)
		for _, move := range []bool{false, true} {
			for _, closeBy := range []string{"Cancel", "Escape", "close"} {
				u.copyOrMove(move)
				title := "Copy 1 item(s) to"
				if move {
					title = "Move/Rename 1 item(s) to"
				}
				w := windowByTitle(t, u.app, title)
				e := w.Canvas().Focused().(*escEntry)
				if e.Theme().Size(theme.SizeNamePadding) != 12 || e.Theme().Size(theme.SizeNameInnerPadding) != 8 {
					t.Fatal("form inherited compact pane spacing")
				}
				if e.Theme().Size(theme.SizeNameText) != u.app.Settings().Theme().Size(theme.SizeNameText) || u.left.list.Theme().Size(theme.SizeNamePadding) != panePadding {
					t.Fatal("dialog spacing changed fonts or pane density")
				}
				outer := w.Content().(*container.ThemeOverride).Content.(*fyne.Container)
				body := outer.Objects[0].(*fyne.Container)
				if body.Position() != fyne.NewPos(16, 16) || outer.Size().Width-body.Size().Width != 32 || outer.Size().Height-body.Size().Height != 32 {
					t.Fatal("dialog needs 16px outer margins")
				}
				form, buttons := body.Objects[0], body.Objects[1]
				if buttons.Position().Y-(form.Position().Y+form.Size().Height) < 12 {
					t.Fatal("form and buttons need breathing room")
				}
				switch closeBy {
				case "Cancel":
					tapDialogButton(t, u, "Cancel")
				case "Escape":
					e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				default:
					w.Close()
				}
				if hasWindow(u.app, title) || !exists(filepath.Join(root, "src", "readme.md")) || exists(filepath.Join(root, "dst", "readme.md")) || u.win.Canvas().Focused() != u.left.focusable() {
					t.Fatal("closing spaced dialog changed files or lost pane focus")
				}
			}
		}
	}
}

func TestTrashConfirmationLayoutAndCancel(t *testing.T) {
	for _, closeBy := range []string{"Cancel", "close", "canvas", "Cancel escape", "action escape"} {
		t.Run(closeBy, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			press(u, fyne.KeyF8)
			title := "Move to " + trashName()
			w := windowByTitle(t, u.app, title)
			assertUIText(t, w.Content(), title+` "readme.md"?`, "Cancel", title)
			outer := w.Content().(*container.ThemeOverride).Content.(*fyne.Container)
			body := outer.Objects[0].(*fyne.Container)
			label, buttons := body.Objects[0], body.Objects[1]
			gap := buttons.Position().Y - label.Position().Y - label.Size().Height
			if body.Position() != fyne.NewPos(16, 16) || gap < 11.99 || gap > 12.01 {
				t.Fatalf("confirmation must use shared margins and compact stacked spacing: position=%v gap=%v", body.Position(), gap)
			}
			controls := dialogFocusables(w)
			if len(controls) != 2 || controls[0].(*dialogButton).Text != "Cancel" || controls[0].(*dialogButton).Importance != widget.HighImportance || controls[1].(*dialogButton).Text != title {
				t.Fatal("confirmation needs safe default and named action")
			}
			cursor := u.left.cursor
			press(u, fyne.KeyDown)
			if u.passwordWindow != w || u.left.cursor != cursor || u.win.Canvas().Overlays().Top() == nil {
				t.Fatal("confirmation must block parent input")
			}
			switch closeBy {
			case "Cancel":
				tapDialogButton(t, u, "Cancel")
			case "close":
				w.Close()
			case "canvas":
				w.Canvas().Unfocus()
				w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
			default:
				i := 0
				if closeBy == "action escape" {
					i = 1
				}
				w.Canvas().Focus(controls[i])
				controls[i].TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			}
			if hasWindow(u.app, title) || !exists(filepath.Join(root, "src", "readme.md")) || u.passwordWindow != nil || u.win.Canvas().Overlays().Top() != nil || u.win.Canvas().Focused() != u.left.focusable() {
				t.Fatal("cancel must preserve file and restore parent input")
			}
		})
	}
}

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
			if slices.Contains(u.app.Driver().AllWindows(), w) && w.Title() == tc.title {
				t.Fatal("Escape without focus did not close window")
			}
			// Every focusable control must also handle Escape itself.
			for i := range focused {
				tc.open()
				current := windowByTitle(t, u.app, tc.title)
				controls := dialogFocusables(current)
				current.Canvas().Focus(controls[i])
				controls[i].TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				if slices.Contains(u.app.Driver().AllWindows(), current) && current.Title() == tc.title {
					t.Fatalf("Escape with focus on %T did not close window", controls[i])
				}
			}
			if !hasWindow(u.app, appName) || !exists(filepath.Join(root, "src", "readme.md")) {
				t.Fatal("Escape closed main window or confirmed operation")
			}
		})
	}
}

func TestF2RenameAndParentPath(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
		for _, p := range []*pane{u.left, u.right} {
			u.activate(p)
			p.setBrief(brief)
			for _, name := range []string{"src", "song.mp3"} {
				if name == "src" {
					p.cd(root, name)
				} else {
					p.cd(filepath.Join(root, "src"), name)
				}
				press(u, fyne.KeyF2)
				w := windowByTitle(t, u.app, "Rename")
				e := w.Canvas().Focused().(*escEntry)
				if e.Text != name {
					t.Fatalf("F2 rename target=%q want %q", e.Text, name)
				}
				e.SetText("renamed-" + name)
				e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
				if !exists(filepath.Join(p.path, "renamed-"+name)) || exists(filepath.Join(p.path, name)) {
					t.Fatal("F2 did not rename cursor item")
				}
				if err := os.Rename(filepath.Join(p.path, "renamed-"+name), filepath.Join(p.path, name)); err != nil {
					t.Fatal(err)
				}
			}
			p.cd(filepath.Join(root, "src"), "..")
			p.marked["song.mp3"] = true
			press(u, fyne.KeyF2)
			if u.win.Canvas().Focused() != p.pathBar || p.pathBar.SelectedText() != p.path || !p.marked["song.mp3"] || hasWindow(u.app, "Rename") {
				t.Fatal("F2 on parent must select full path without rename or clearing marks")
			}
			test.Type(p.pathBar, root)
			p.pathBar.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if p.path != root {
				t.Fatal("selected path could not be replaced and submitted")
			}
		}
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
