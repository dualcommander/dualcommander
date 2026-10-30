package commander

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestFileClipboardShortcuts(t *testing.T) {
	for _, cut := range []bool{false, true} {
		for _, selection := range []bool{false, true} {
			t.Run(fmt.Sprintf("cut=%v/selection=%v", cut, selection), func(t *testing.T) {
				root := fixture(t)
				u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
				canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
				cursorTo(t, u.left, "readme.md")
				want := []string{"readme.md"}
				if selection {
					u.left.marked["readme.md"], u.left.marked["photos"] = true, true
					want = append(want, "photos/a.png")
				}
				if cut {
					canvas.TypedShortcut(&fyne.ShortcutCut{Clipboard: u.win.Clipboard()})
				} else {
					canvas.TypedShortcut(&fyne.ShortcutCopy{Clipboard: u.win.Clipboard()})
				}
				if !exists(filepath.Join(root, "src/readme.md")) || exists(filepath.Join(root, "dst/readme.md")) {
					t.Fatal("copy/cut changed disk before paste")
				}
				// Capture paths now, not the cursor/selection at paste time.
				u.left.markAll(false)
				cursorTo(t, u.left, "song.mp3")
				u.activate(u.right)
				canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: u.win.Clipboard()})
				waitJobs(t, u)
				for _, name := range want {
					data, err := os.ReadFile(filepath.Join(root, "dst", name))
					if err != nil || len(data) == 0 {
						t.Fatalf("paste missing %s: %v", name, err)
					}
					if exists(filepath.Join(root, "src", name)) == cut {
						t.Fatalf("wrong source state after cut=%v: %s", cut, name)
					}
				}
				if exists(filepath.Join(root, "dst/song.mp3")) || !slices.Contains(names(u.right), "readme.md") || u.active != u.right || u.win.Canvas().Focused() != u.right.focusable() {
					t.Fatal("paste changed selection snapshot, pane state, or focus")
				}
				if err := u.right.cd(filepath.Join(root, "src/docs"), ""); err != nil {
					t.Fatal(err)
				}
				canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: u.win.Clipboard()})
				waitJobs(t, u)
				if exists(filepath.Join(root, "src/docs/readme.md")) == cut {
					t.Fatal("copy must remain reusable; successful cut must be consumed")
				}
			})
		}
	}
}

func TestFileClipboardTextAndModal(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
	clipboard := u.win.Clipboard()
	canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clipboard}) // empty clipboard is harmless
	cursorTo(t, u.left, "readme.md")
	canvas.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clipboard})
	u.activate(u.right)
	press(u, fyne.KeyF7)
	w := windowByTitle(t, u.app, "New directory")
	e := w.Canvas().Focused().(*escEntry)
	e.SetText("text clipboard")
	e.TypedShortcut(&fyne.ShortcutSelectAll{})
	menu := u.win.MainMenu().Items[1]
	for _, label := range []string{"Copy", "Cut", "Paste"} {
		var action *fyne.MenuItem
		for _, item := range menu.Items {
			if item.Label == label {
				action = item
			}
		}
		if action == nil || action.Disabled {
			t.Fatalf("text menu %s unavailable in dialog", label)
		}
		action.Action()
	}
	if e.Text != "text clipboard" || exists(filepath.Join(root, "dst/readme.md")) {
		t.Fatal("clipboard menu interfered with dialog or pasted files behind modal")
	}
	w.Close()
	canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clipboard})
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst/readme.md")) {
		t.Fatal("dialog text editing lost pending file clipboard")
	}
}

func TestFileClipboardCutConflict(t *testing.T) {
	for _, choice := range []string{"Skip", "Cancel", "Escape", "Close", "Overwrite"} {
		t.Run(choice, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			if err := os.WriteFile(filepath.Join(root, "dst/readme.md"), []byte("existing"), 0o644); err != nil {
				t.Fatal(err)
			}
			canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
			cursorTo(t, u.left, "readme.md")
			canvas.TypedShortcut(&fyne.ShortcutCut{Clipboard: u.win.Clipboard()})
			u.activate(u.right)
			canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: u.win.Clipboard()})
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
			want := "existing"
			if choice == "Overwrite" {
				want = "hello"
			}
			data, err := os.ReadFile(filepath.Join(root, "dst/readme.md"))
			if err != nil || string(data) != want || exists(filepath.Join(root, "src/readme.md")) != (choice != "Overwrite") {
				t.Fatalf("%s: unexpected source/target state: %q, %v", choice, data, err)
			}
			if err := u.right.cd(filepath.Join(root, "src/docs"), ""); err != nil {
				t.Fatal(err)
			}
			canvas.TypedShortcut(&fyne.ShortcutPaste{Clipboard: u.win.Clipboard()})
			waitJobs(t, u)
			if exists(filepath.Join(root, "src/docs/readme.md")) != (choice != "Overwrite") {
				t.Fatal("skipped/cancelled cut must remain available; overwritten cut must be consumed")
			}
		})
	}
}

func TestShiftArrowMarksAndMoves(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			t.Run(fmt.Sprintf("brief=%v/right=%v", brief, right), func(t *testing.T) {
				root := fixture(t)
				u := newTestUI(t, shiftApp{test.NewTempApp(t)}, filepath.Join(root, "src"), filepath.Join(root, "src"))
				p := u.left
				if right {
					p = u.right
				}
				p.setBrief(brief)
				u.activate(p)
				key := func(k fyne.KeyName) { u.win.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: k}) }
				check := func(from string, k fyne.KeyName, to string, marked bool) {
					t.Helper()
					cursorTo(t, p, from)
					key(k)
					if p.current().name != to || p.marked[from] != marked {
						t.Fatalf("%s %s: cursor=%q marked=%v; want cursor=%q marked=%v", from, k, p.current().name, p.marked[from], to, marked)
					}
				}
				check("docs", fyne.KeyDown, "photos", true)
				check("photos", fyne.KeyDown, "app.zip", true)
				check("app.zip", fyne.KeyUp, "photos", true)
				check("photos", fyne.KeyUp, "docs", false)
				check("docs", fyne.KeyUp, "..", false)
				check("..", fyne.KeyUp, "..", false)
				check("..", fyne.KeyDown, "docs", false)
				check("song.mp3", fyne.KeyDown, "song.mp3", true)
				if len(p.marked) != 2 || len(u.other().marked) != 0 {
					t.Fatalf("unexpected marks: active=%v other=%v", p.marked, u.other().marked)
				}
				// Releasing Shift restores navigation without changing marks.
				u.app = u.app.(shiftApp).App
				check("readme.md", fyne.KeyUp, "app.zip", false)
				check("app.zip", fyne.KeyDown, "readme.md", true)
				assertPaneRows(t, p)
			})
		}
	}
}

func TestShiftPageAndBoundaryMarks(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			for _, key := range []fyne.KeyName{fyne.KeyPageUp, fyne.KeyPageDown, fyne.KeyHome, fyne.KeyEnd} {
				t.Run(fmt.Sprintf("brief=%v/right=%v/%s", brief, right, key), func(t *testing.T) {
					root := fixture(t)
					u := newTestUI(t, shiftApp{test.NewTempApp(t)}, filepath.Join(root, "src"), filepath.Join(root, "src"))
					p := u.left
					if right {
						p = u.right
					}
					p.setBrief(brief)
					u.activate(p)
					cursorTo(t, p, "app.zip")
					start, page := p.cursor, 2
					if brief {
						page = p.grid.rows * p.grid.cols
					}
					target := map[fyne.KeyName]int{
						fyne.KeyPageUp: max(0, start-page), fyne.KeyPageDown: min(len(p.entries)-1, start+page),
						fyne.KeyHome: 0, fyne.KeyEnd: len(p.entries) - 1,
					}[key]
					want := map[string]bool{}
					step := 1
					if target < start {
						step = -1
					}
					for i := start; i != target; i += step {
						if p.entries[i].name != ".." {
							want[p.entries[i].name] = true
						}
					}
					for repeat := 0; repeat < 2; repeat++ {
						p.moveTo(start)
						u.key(p, &fyne.KeyEvent{Name: key}, 2)
						if p.cursor != target || !reflect.DeepEqual(p.marked, want) || len(u.other().marked) != 0 {
							t.Fatalf("cursor=%d marks=%v; want cursor=%d marks=%v", p.cursor, p.marked, target, want)
						}
						want = map[string]bool{}
					}
					// Plain navigation moves without marking.
					u.app = u.app.(shiftApp).App
					p.moveTo(start)
					u.key(p, &fyne.KeyEvent{Name: key}, 2)
					if p.cursor != target || len(p.marked) != 0 {
						t.Fatalf("plain navigation: cursor=%d marks=%v", p.cursor, p.marked)
					}
				})
			}
		}
	}
}

func TestFavouriteButtonWidth(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, density := range []string{"standard", "compact"} {
		applyTheme(u.app, "system", density)
		for _, starred := range []bool{false, true} {
			if starred {
				test.Tap(u.left.favourite)
			}
			for _, p := range []*pane{u.left, u.right} {
				laidOutObjects(p.box)
				for _, button := range []*widget.Button{p.favourites, p.viewButton} {
					if p.favourite.Size().Width < button.Size().Width {
						t.Errorf("%s starred=%v: favourite width %v smaller than neighboring width %v", density, starred, p.favourite.Size().Width, button.Size().Width)
					}
				}
			}
			if starred {
				test.Tap(u.left.favourite)
			}
		}
	}
}

func TestPaneDividerPercentagesAndReset(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.win.Resize(fyne.NewSize(1600, 700))
	applyTheme(u.app, "dark", "standard")
	assertNoPercentages := func() {
		t.Helper()
		for _, o := range laidOutObjects(u.win.Content()) {
			if label, ok := o.(*widget.Label); ok && strings.HasSuffix(label.Text, "%") && label.Visible() {
				t.Errorf("percentage remains in pane layout: %q", label.Text)
			}
		}
		if len(u.win.Canvas().Overlays().List()) != 0 {
			t.Error("percentage tooltips visible outside drag")
		}
	}
	assertNoPercentages()
	var divider fyne.Draggable
	var target fyne.CanvasObject
	for _, o := range test.WidgetRenderer(u.split).Objects() {
		if d, ok := o.(*paneDivider); ok {
			divider, target = d, o
			break
		}
	}
	if divider == nil {
		t.Fatal("missing draggable divider")
	}
	assertMarker := func() {
		t.Helper()
		var marker *canvas.Rectangle
		for _, o := range laidOutObjects(u.split) {
			if rect, ok := o.(*canvas.Rectangle); ok && reflect.DeepEqual(rect.FillColor, theme.ForegroundColor()) {
				marker = rect
			}
		}
		if marker == nil || marker.Size().Width <= 0 || marker.Size().Height <= 0 {
			t.Error("native divider foreground marker missing")
		} else {
			pos := u.app.Driver().AbsolutePositionForObject(marker)
			left, right := u.left.box.Size().Width, u.right.box.Position().X
			if pos.X < left || pos.X+marker.Size().Width > right {
				t.Errorf("native marker outside divider: position=%v size=%v divider=%v..%v", pos, marker.Size(), left, right)
			}
		}
	}
	assertMarker()
	for _, delta := range []float32{120, -240, 10000, -10000} {
		divider.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(2+delta, 100)}, Dragged: fyne.NewDelta(delta, 0)})
		left, right := u.left.box.Size().Width, u.right.box.Size().Width
		percent := int(float64(left/(left+right))*100 + 0.5)
		overlays := u.win.Canvas().Overlays().List()
		if len(overlays) != 1 {
			t.Fatalf("drag tooltip overlay: got %d, want 1", len(overlays))
		}
		assertUIText(t, overlays[0], fmt.Sprintf("%d%%", percent))
		assertUIText(t, overlays[0], fmt.Sprintf("%d%%", 100-percent))
		var labels []*widget.Label
		for _, o := range laidOutObjects(overlays[0]) {
			if l, ok := o.(*widget.Label); ok && strings.HasSuffix(l.Text, "%") {
				labels = append(labels, l)
			}
		}
		if len(labels) != 2 {
			t.Fatalf("tooltip labels: got %d, want 2", len(labels))
		}
		for i, pane := range []*pane{u.left, u.right} {
			label := labels[i]
			pos := u.app.Driver().AbsolutePositionForObject(label)
			panePos := u.app.Driver().AbsolutePositionForObject(pane.box)
			if pos.X < panePos.X || pos.X+label.Size().Width > panePos.X+pane.box.Size().Width {
				t.Errorf("pane %d tooltip outside pane: position=%v size=%v pane=%v size=%v", i, pos, label.Size(), panePos, pane.box.Size())
			}
		}
		assertMarker()
		if left < u.left.box.MinSize().Width-1 || right < u.right.box.MinSize().Width-1 {
			t.Fatal("divider ignored pane minimum width")
		}
		// Repeated drag events must update existing tooltips, not accumulate overlays.
		divider.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(2, 100)}})
		if len(u.win.Canvas().Overlays().List()) != 1 {
			t.Fatal("drag accumulated tooltip overlays")
		}
		divider.DragEnd()
		assertNoPercentages()
	}
	doubleTap, ok := target.(fyne.DoubleTappable)
	if !ok {
		t.Fatal("divider does not handle double-click")
	}
	test.DoubleTap(doubleTap)
	if u.split.Offset != 0.5 || u.left.box.Size().Width != u.right.box.Size().Width {
		t.Fatalf("double-click did not center panes: offset=%v widths=%v/%v", u.split.Offset, u.left.box.Size().Width, u.right.box.Size().Width)
	}
	assertNoPercentages()
}

func TestNativeDeletionWording(t *testing.T) {
	name := "Trash"
	switch runtime.GOOS {
	case "darwin":
		name = "Bin"
	case "windows":
		name = "Recycle Bin"
	}
	if trashName() != name {
		t.Fatalf("native deletion name: got %q, want %q", trashName(), name)
	}
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	press(u, fyne.KeyF8)
	w := windowByTitle(t, u.app, "Move to "+name)
	assertUIText(t, w.Content(), "Move to "+name+` "readme.md"?`, "Move to "+name)
	tapDialogButton(t, u, "Cancel")
	u.settings()
	w = windowByTitle(t, u.app, "Settings")
	defer w.Close()
	for _, o := range test.LaidOutObjects(w.Content()) {
		if c, ok := o.(*dialogCheck); ok && c.Text == "Move deleted files to "+name {
			return
		}
	}
	t.Fatalf("settings missing native deletion wording: %s", name)
}

func TestSettingsCommandArrayValidation(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, root, root)
	u.settings()
	w := windowByTitle(t, a, "Settings")
	keys := []string{prefRevealCmd, prefTerminalCmd, prefEditorCmd, prefCompareFilesCmd, prefCompareFoldersCmd}
	var fields []*escEntry
	for _, o := range test.LaidOutObjects(w.Content()) {
		if e, ok := o.(*escEntry); ok {
			fields = append(fields, e)
		}
	}
	if len(fields) != len(keys) {
		t.Fatalf("command fields: got %d want %d", len(fields), len(keys))
	}
	for i, e := range fields {
		w.Canvas().Focus(e)
		var reportedError error
		e.SetOnValidationChanged(func(err error) { reportedError = err })
		valid := `["tool","{left}","{right}","argument with spaces",""]`
		e.SetText(valid)
		if err := e.Validate(); err != nil || a.Preferences().String(keys[i]) != valid {
			t.Fatalf("%s: valid array rejected: %v", keys[i], err)
		}
		for _, invalid := range []string{
			`["tool","{left}","{right}",null]`,
			`tool`, `"tool"`, `{}`, `null`, `[]`, `[""]`, `[" "]`,
			`["tool",`, `["tool",1]`, `["tool",true]`, `["tool",{}]`, `["tool",[]]`,
			`["tool"] trailing`, `["tool","\u0000"]`,
		} {
			e.SetText(invalid)
			if reportedError == nil {
				t.Errorf("%s: focused invalid input still has OK validation state: %s", keys[i], invalid)
			}
			if e.Validate() == nil {
				t.Errorf("%s: accepted invalid command %s", keys[i], invalid)
			}
			if a.Preferences().String(keys[i]) != valid {
				t.Errorf("%s: invalid command replaced saved value: %s", keys[i], invalid)
			}
			e.SetText(valid)
			if reportedError != nil {
				t.Errorf("%s: corrected input retains validation error: %v", keys[i], reportedError)
			}
		}
		e.SetText(" ")
		if err := e.Validate(); err != nil || a.Preferences().String(keys[i]) != " " {
			t.Fatalf("%s: blank field rejected: %v", keys[i], err)
		}
	}
}

func TestExternalCommandsSettings(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.settings()
	w := windowByTitle(t, a, "Settings")
	u.settings()
	if windowByTitle(t, a, "Settings") != w || u.win.Canvas().Overlays().Top() == nil {
		t.Fatal("settings must reuse its decorated window and block parent input")
	}
	entries := []*escEntry{}
	labels := []string{}
	var separators, infoIcons int
	for _, o := range test.LaidOutObjects(w.Content()) {
		if form, ok := o.(*widget.Form); ok {
			for _, item := range form.Items {
				labels = append(labels, item.Text)
			}
		}
		if l, ok := o.(*widget.Label); ok {
			labels = append(labels, l.Text)
			switch l.Text {
			case "Appearance", "File views", "File deletion", "Archives", "External applications":
				if !l.TextStyle.Bold {
					t.Fatalf("section heading not bold: %s", l.Text)
				}
			default:
				if strings.Contains(l.Text, "Changes are saved automatically") || strings.Contains(l.Text, "When unchecked,") || strings.Contains(l.Text, "deleted files cannot be restored") || strings.Contains(l.Text, "Custom commands use") || strings.Contains(l.Text, "Enter or double-click") {
					if l.Importance != widget.MediumImportance || l.Wrapping != fyne.TextWrapWord {
						t.Fatalf("guidance must be readable and wrap: %s", l.Text)
					}
				}
			}
		}
		if _, ok := o.(*widget.Separator); ok {
			separators++
		}
		if _, ok := o.(*dialogInfoIcon); ok {
			infoIcons++
		}
		if e, ok := o.(*escEntry); ok {
			entries = append(entries, e)
		}
	}
	if len(entries) != 5 {
		t.Fatalf("command fields: %d", len(entries))
	}
	text := strings.Join(labels, "\n")
	for _, label := range []string{"Appearance", "Theme", "Density", "Font size", "File views", "Left pane view", "Right pane view", "File deletion", "Archives", "External applications", "Show in file manager", "Open folder in terminal", "Edit file (F4)"} {
		if !slices.Contains(labels, label) {
			t.Fatalf("missing settings label: %s", label)
		}
	}
	if separators != 5 || infoIcons != 5 {
		t.Fatalf("settings grouping/hints: %d separators, %d info icons", separators, infoIcons)
	}
	if !strings.Contains(text, "Changes are saved automatically") || !strings.Contains(text, "Leave a field blank") {
		t.Fatalf("missing settings guidance: %s", text)
	}
	for _, e := range entries {
		if e.Text != "" || e.PlaceHolder == "" {
			t.Fatal("missing default command hint")
		}
	}
	reveal := `["custom-manager","{path}"]`
	terminal := `["custom-terminal","--directory","{dir}"]`
	entries[0].SetText(reveal)
	entries[1].SetText(terminal)
	if entries[0].Text != reveal || entries[1].Text != terminal || a.Preferences().String(prefRevealCmd) != reveal || a.Preferences().String(prefTerminalCmd) != terminal {
		t.Fatal("command fields or saved values differ from input")
	}
	entries[0].SetText("invalid")
	if entries[0].Validate() == nil || a.Preferences().String(prefRevealCmd) != reveal {
		t.Fatal("invalid command replaced saved command")
	}
	entries[0].SetText("")
	if entries[0].Validate() != nil || a.Preferences().String(prefRevealCmd) != "" {
		t.Fatal("blank command did not restore OS default")
	}
	entries[0].SetText(reveal)
	editor := `["custom-editor","--wait","{path}"]`
	entries[2].SetText(editor)
	if entries[2].Validate() != nil || a.Preferences().String(prefEditorCmd) != editor {
		t.Fatal("editor command not saved")
	}
	entries[2].SetText("invalid")
	if entries[2].Validate() == nil || a.Preferences().String(prefEditorCmd) != editor {
		t.Fatal("invalid editor replaced saved command")
	}
	entries[2].SetText(" ")
	if entries[2].Validate() != nil {
		t.Fatal("blank editor rejected")
	}
	cmd, err := u.editorCommand(filepath.Join(u.left.path, "readme.md"))
	if err != nil || !slices.Equal(cmd.Args, editCmd(runtime.GOOS, filepath.Join(u.left.path, "readme.md"))) {
		t.Fatalf("blank editor must use OS default: %v %v", cmd, err)
	}
	entries[2].SetText(editor)
	w.Close()
	if slices.Contains(a.Driver().AllWindows(), w) || windowByTitle(t, a, appName) == nil {
		t.Fatal("closing settings must leave main window open")
	}
	u = newTrackedUI(t, a)
	if a.Preferences().String(prefEditorCmd) != editor {
		t.Fatal("editor setting not restored")
	}
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		path := filepath.Join(p.path, `a b;$(echo unsafe)".txt`)
		cmd, err := u.editorCommand(path)
		if err != nil || !slices.Equal(cmd.Args, []string{"custom-editor", "--wait", path}) || cmd.Dir != p.path {
			t.Fatalf("editor command: %v %v", cmd, err)
		}
	}
	if a.Preferences().String(prefTerminalCmd) != terminal {
		t.Fatal("terminal setting not restored")
	}
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		cmd, err := u.externalCommand(true)
		if err != nil || !slices.Equal(cmd.Args, []string{"custom-terminal", "--directory", p.path}) || cmd.Dir != p.path {
			t.Fatalf("terminal command: %v %v", cmd, err)
		}
	}
	u.activate(u.left)
	cursorTo(t, u.left, "readme.md")
	a.Preferences().SetString(prefEditorCmd, "invalid")
	press(u, fyne.KeyF4)
	if !hasWindow(a, "Error") {
		t.Fatal("F4 must report invalid editor command")
	}
	tapDialogButton(t, u, "OK")
	a.Preferences().SetString(prefEditorCmd, editor)
	cmd, err = u.externalCommand(false)
	if err != nil || !slices.Equal(cmd.Args, []string{"custom-manager", filepath.Join(u.left.path, "readme.md")}) {
		t.Fatalf("reveal command: %v %v", cmd, err)
	}
	a.Preferences().SetString(prefTerminalCmd, "")
	if _, err := u.externalCommand(true); err != nil {
		t.Fatalf("blank must use default: %v", err)
	}
	for _, label := range []string{revealLabel(), "Open in Terminal"} {
		found := false
		for _, item := range u.win.MainMenu().Items[0].Items {
			if item.Label == label {
				found = true
				// Bad stored config must show an error, not start an OS app.
				a.Preferences().SetString(prefRevealCmd, "invalid")
				a.Preferences().SetString(prefTerminalCmd, "invalid")
				item.Action()
				if !hasWindow(u.app, "Error") {
					t.Fatal("command error not shown")
				}
				tapDialogButton(t, u, "OK")
			}
		}
		if !found {
			t.Fatalf("missing menu action %q", label)
		}
	}
}

func TestSettingsTogglesWithoutTooltips(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.settings()
	w := windowByTitle(t, u.app, "Settings")
	var toggles []*dialogCheck
	for _, object := range test.LaidOutObjects(w.Content()) {
		if b, ok := object.(*dialogButton); ok && b.Icon == theme.HelpIcon() {
			t.Fatal("help button remains")
		}
		if c, ok := object.(*dialogCheck); ok && (c.Text == "Move deleted files to "+trashName() || c.Text == "Browse archives in DualCommander") {
			toggles = append(toggles, c)
		}
	}
	if len(toggles) != 2 {
		t.Fatal("settings toggles missing")
	}
	for _, c := range toggles {
		for _, keyboard := range []bool{false, true} {
			if keyboard {
				w.Canvas().Focus(c)
				c.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
			} else {
				c.MouseIn(&desktop.MouseEvent{})
			}
			if w.Canvas().Overlays().Top() != nil {
				t.Fatal("toggle created tooltip overlay")
			}
			if keyboard {
				c.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
			} else {
				c.MouseOut()
			}
			if w.Canvas().Overlays().Top() != nil || !hasWindow(u.app, "Settings") {
				t.Fatal("toggle interaction created overlay or closed Settings")
			}
		}
		before := c.Checked
		c.TypedRune(' ')
		if c.Checked == before {
			t.Fatal("tooltip prevents toggling")
		}
	}
	w.Canvas().Focus(toggles[0])
	toggles[0].TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if hasWindow(u.app, "Settings") {
		t.Fatal("Escape left Settings open")
	}
}

func TestSettingsHoverHints(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, closeWithEscape := range []bool{false, true} {
		u.settings()
		w := windowByTitle(t, u.app, "Settings")
		for _, tc := range []struct{ label, hint string }{
			{"Show free disk space", "When unchecked, used disk space is shown instead."},
			{"Browse archives in DualCommander", "Browse supported archives like folders (read-only). When unchecked, archives open in the default application."},
			{"Move deleted files to " + trashName(), "When unchecked, files bypass the trash and are permanently deleted."},
		} {
			var toggle *dialogCheck
			for _, object := range test.LaidOutObjects(w.Content()) {
				if c, ok := object.(*dialogCheck); ok && c.Text == tc.label {
					toggle = c
				}
			}
			if toggle == nil {
				t.Fatalf("missing checkbox %q", tc.label)
			}
			hintVisible := func() bool {
				if toggle.hint != nil && !toggle.hint.Visible() {
					return false
				}
				for _, object := range test.LaidOutObjects(w.Content()) {
					if l, ok := object.(*widget.Label); ok && l.Text == tc.hint {
						return true
					}
				}
				return false
			}
			if hintVisible() {
				t.Fatal("hint visible before hover")
			}
			toggle.MouseIn(&desktop.MouseEvent{})
			if !hintVisible() || w.Canvas().Overlays().Top() != nil {
				t.Fatal("hover hint missing or intercepts checkbox input")
			}
			icons := 0
			var icon *dialogInfoIcon
			var text *widget.Label
			for _, object := range test.LaidOutObjects(toggle.hint) {
				if i, ok := object.(*dialogInfoIcon); ok {
					icons++
					icon = i
				}
				if l, ok := object.(*widget.Label); ok && l.Text == tc.hint {
					text = l
				}
			}
			if icons != 1 {
				t.Fatal("tooltip must contain same Info icon as dialog guidance")
			}
			driver := u.app.Driver()
			gap := driver.AbsolutePositionForObject(text).X - driver.AbsolutePositionForObject(icon).X - icon.Size().Width
			if gap < 8 {
				t.Fatalf("tooltip icon/text gap = %g, want at least 8", gap)
			}
			toggle.MouseOut()
			if hintVisible() {
				t.Fatal("hint remained after mouse exit")
			}
			w.Canvas().Focus(toggle)
			if !hintVisible() {
				t.Fatal("hint unavailable to keyboard users")
			}
			before := toggle.Checked
			toggle.TypedRune(' ')
			if toggle.Checked == before || hintVisible() {
				t.Fatal("hint blocks toggle or remains after toggling")
			}
			w.Canvas().Unfocus()
			toggle.MouseIn(&desktop.MouseEvent{})
			before = toggle.Checked
			test.Tap(toggle)
			if toggle.Checked == before || hintVisible() {
				t.Fatal("hint blocks mouse toggle or remains after tapping")
			}
			w.Canvas().Unfocus()
		}
		if closeWithEscape {
			w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
		} else {
			w.Close()
		}
		if hasWindow(u.app, "Settings") {
			t.Fatal("Settings did not close")
		}
	}
}

func TestAppearanceSettings(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.right.setBrief(true)
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"] = true
	u.settings()
	w := windowByTitle(t, a, "Settings")
	var choices []*dialogSelect
	for _, object := range test.LaidOutObjects(w.Content()) {
		if s, ok := object.(*dialogSelect); ok {
			choices = append(choices, s)
		}
	}
	if len(choices) != 6 {
		t.Fatalf("appearance controls: %d", len(choices))
	}
	for i, group := range []struct {
		options []string
		value   *string
		key     string
		items   []*fyne.MenuItem
	}{
		{themeModes, &u.themeMode, prefTheme, u.themeItems},
		{densities, &u.density, prefDensity, u.densItems},
		{fontSizes, &u.fontSize, prefFontSize, u.fontItems},
	} {
		s := choices[i]
		if !slices.Equal(s.Options, group.options) || s.Selected != *group.value {
			t.Fatalf("incorrect choices/default: %v, %s", s.Options, s.Selected)
		}
		for range group.options {
			w.Canvas().Focus(s)
			s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
			if *group.value != s.Selected || a.Preferences().String(group.key) != s.Selected || w.Canvas().Focused() != s {
				t.Fatalf("choice not applied/saved or focus lost: %s", s.Selected)
			}
			for j, item := range group.items {
				if item.Checked != (group.options[j] == s.Selected) {
					t.Fatal("View menu out of sync")
				}
			}
			content := w.Content().(*fyne.Container).Objects[0].(*container.Scroll).Content.(*container.ThemeOverride)
			if content.Theme.(dialogTheme).Theme != a.Settings().Theme() {
				t.Fatal("settings theme not updated live")
			}
			if u.left.current().name != "readme.md" || !u.left.marked["readme.md"] || !u.right.brief {
				t.Fatal("appearance change lost pane state")
			}
		}
	}
	choices[0].SetSelected("dark")
	choices[1].SetSelected("compact")
	choices[2].SetSelected("large")
	w.Close()
	u = newTrackedUI(t, a)
	u.settings()
	w = windowByTitle(t, a, "Settings")
	var restored []string
	for _, object := range test.LaidOutObjects(w.Content()) {
		if s, ok := object.(*dialogSelect); ok {
			restored = append(restored, s.Selected)
		}
	}
	if !slices.Equal(restored, []string{"dark", "compact", "large", "System", "Detailed", "Brief"}) || u.themeMode != "dark" || u.density != "compact" || u.fontSize != "large" {
		t.Fatalf("appearance preferences not restored: %v", restored)
	}
	w.Close()
}

func TestSettingsDropdownAlternatives(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.settings()
	w := windowByTitle(t, u.app, "Settings")
	var choices []*dialogSelect
	for _, object := range test.LaidOutObjects(w.Content()) {
		if s, ok := object.(*dialogSelect); ok {
			choices = append(choices, s)
		}
	}
	if len(choices) != 6 {
		t.Fatal("settings choices missing")
	}
	for _, s := range choices {
		before := s.Selected
		test.Tap(s)
		popup, ok := w.Canvas().Focused().(*widget.PopUpMenu)
		if !ok || len(popup.Items) != len(s.Options)-1 {
			t.Fatal("popup must contain alternatives only")
		}
		var alternatives []string
		for _, option := range s.Options {
			if option != before {
				alternatives = append(alternatives, option)
			}
		}
		for i, item := range popup.Items {
			var labels []string
			for _, object := range laidOutObjects(item) {
				if text, ok := object.(*canvas.Text); ok && text.Text != "" {
					labels = append(labels, text.Text)
				}
			}
			if !slices.Equal(labels, []string{alternatives[i]}) {
				t.Fatalf("popup label: %v, want %s", labels, alternatives[i])
			}
		}
		popup.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		if s.Selected != before || w.Canvas().Overlays().Top() != nil || !hasWindow(u.app, "Settings") {
			t.Fatal("dismissing popup changed value or closed settings")
		}
		w.Canvas().Focus(s)
		s.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
		popup = w.Canvas().Focused().(*widget.PopUpMenu)
		test.Tap(popup.Items[0].(fyne.Tappable))
		if s.Selected != alternatives[0] || w.Canvas().Overlays().Top() != nil {
			t.Fatal("popup selection failed")
		}
		s.Disable()
		test.Tap(s)
		if w.Canvas().Overlays().Top() != nil {
			t.Fatal("disabled select opened popup")
		}
		s.Enable()
	}
	w.Close()
}

func TestFileViewSettingsAndMenu(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, root, root)
	if err := os.WriteFile(filepath.Join(root, ".hidden"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	checkMenu := func() {
		t.Helper()
		if u.briefItem.Checked != u.active.brief || u.detailedItem.Checked == u.active.brief || u.hiddenItem.Checked != u.showHidden {
			t.Fatal("View menu does not reflect active pane and hidden files")
		}
	}
	controls := func() (fyne.Window, []*dialogSelect, *dialogCheck) {
		t.Helper()
		u.settings()
		w := windowByTitle(t, a, "Settings")
		var views []*dialogSelect
		var hidden *dialogCheck
		for _, object := range test.LaidOutObjects(w.Content()) {
			switch c := object.(type) {
			case *dialogSelect:
				if slices.Equal(c.Options, []string{"Brief", "Detailed"}) {
					views = append(views, c)
				}
			case *dialogCheck:
				if c.Text == "Show hidden files" {
					hidden = c
				}
			}
		}
		if len(views) != 2 || hidden == nil {
			t.Fatal("missing file view settings")
		}
		return w, views, hidden
	}
	checkMenu()
	w, views, hidden := controls()
	if views[0].Selected != "Detailed" || views[1].Selected != "Detailed" || hidden.Checked {
		t.Fatal("incorrect default file view settings")
	}
	views[0].SetSelected("Brief")
	if !u.left.brief || u.right.brief || !a.Preferences().Bool(prefLeftBrief) {
		t.Fatal("left view setting not applied independently or saved")
	}
	checkMenu()
	views[1].SetSelected("Brief")
	views[0].SetSelected("Detailed")
	checkMenu()
	test.Tap(hidden)
	if !u.showHidden || !a.Preferences().Bool(prefHidden) {
		t.Fatal("hidden setting not applied or saved immediately")
	}
	for _, p := range []*pane{u.left, u.right} {
		if !slices.ContainsFunc(p.entries, func(e entry) bool { return e.name == ".hidden" }) {
			t.Fatal("hidden file missing from pane")
		}
	}
	checkMenu()
	w.Close()
	u.activate(u.right)
	checkMenu()
	u.detailedItem.Action()
	checkMenu()
	u.briefItem.Action()
	checkMenu()
	test.Tap(u.right.viewButton)
	checkMenu()
	canvas := reflect.ValueOf(u.win.Canvas()).Elem().FieldByName("WindowlessCanvas").Interface().(fyne.Shortcutable)
	canvas.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.Key1, Modifier: fyne.KeyModifierShortcutDefault})
	if !u.right.brief {
		t.Fatal("Brief shortcut not applied")
	}
	checkMenu()
	u.hiddenItem.Action()
	if u.showHidden || a.Preferences().Bool(prefHidden) {
		t.Fatal("menu hidden toggle not saved")
	}
	checkMenu()
	u.toggleHidden()
	u = newTrackedUI(t, a)
	checkMenu()
	w, views, hidden = controls()
	if u.active != u.right || views[0].Selected != "Detailed" || views[1].Selected != "Brief" || !hidden.Checked {
		t.Fatal("file view preferences not restored")
	}
	w.Close()
}

func TestArchiveOpeningSetting(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	// Compare complete URIs: Windows drive letters live in the URI host, not Path.
	wantURL := storage.NewFileURI(name).String()
	a := &brandingApp{App: test.NewTempApp(t)}
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	setBrowse := func(w fyne.Window, checked bool) {
		t.Helper()
		for _, object := range test.LaidOutObjects(w.Content()) {
			if c, ok := object.(*dialogCheck); ok && c.Text == "Browse archives in DualCommander" {
				if c.Checked == checked {
					t.Fatalf("expected previous archive preference %v", !checked)
				}
				test.Tap(c)
				if c.Checked != checked || a.Preferences().Bool(prefBrowseArchives) != checked {
					t.Fatal("archive choice not saved immediately")
				}
				return
			}
		}
		t.Fatal("archive choice missing")
	}
	u.settings()
	w := windowByTitle(t, a, "Settings")
	setBrowse(w, false) // Existing browse behavior remains default.
	w.Close()
	u.save()
	u = newTrackedUI(t, a)
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		if err := p.cd(filepath.Join(root, "src"), "browse.zip"); err != nil {
			t.Fatal(err)
		}
		for _, doubleClick := range []bool{false, true} {
			a.openedURL = nil
			if doubleClick {
				r := newRowWidget(p, false)
				r.id = p.cursor
				r.DoubleTapped(nil)
			} else {
				press(u, fyne.KeyReturn)
			}
			waitJobs(t, u)
			if a.openedURL == nil || a.openedURL.Scheme != "file" || a.openedURL.String() != wantURL || p.path != filepath.Dir(name) || p.inArchive() || p.current().name != "browse.zip" {
				t.Fatalf("archive must open externally without changing pane: %v, %s", a.openedURL, p.path)
			}
		}
	}
	u.settings()
	w = windowByTitle(t, a, "Settings")
	setBrowse(w, true)
	w.Close()
	a.openedURL = nil
	press(u, fyne.KeyReturn)
	waitJobs(t, u)
	if u.active.path != name || !u.active.inArchive() || a.openedURL != nil || !slices.Equal(names(u.active), []string{"..", "docs"}) {
		t.Fatal("re-enabling archive browsing did not take effect")
	}
}

// Fyne's headless app/window discard SetIcon; record calls at that boundary.
type brandingApp struct {
	fyne.App
	icon      fyne.Resource
	window    *brandingWindow
	openedURL *url.URL
	openErr   error
	metadata  fyne.AppMetadata
}

type brandingWindow struct {
	fyne.Window
	icon fyne.Resource
}

func (a *brandingApp) SetIcon(icon fyne.Resource)    { a.icon = icon }
func (a *brandingApp) OpenURL(target *url.URL) error { a.openedURL = target; return a.openErr }
func (a *brandingApp) Metadata() fyne.AppMetadata    { return a.metadata }
func (a *brandingApp) NewWindow(title string) fyne.Window {
	a.window = &brandingWindow{Window: a.App.NewWindow(title)}
	return a.window
}
func (w *brandingWindow) SetIcon(icon fyne.Resource) { w.icon = icon }

func TestAppBranding(t *testing.T) {
	previous := buildVersion
	buildVersion = "0.1.0+abcdef123456"
	t.Cleanup(func() { buildVersion = previous })
	root := fixture(t)
	a := &brandingApp{App: test.NewTempApp(t)}
	u := newTestUI(t, a, root, root)
	if u.win.Title() != "DualCommander" || a.icon != appIcon || a.window.icon != appIcon {
		t.Fatal("app name or icon missing")
	}
	menus := u.win.MainMenu().Items
	help := menus[len(menus)-1]
	label := "About DualCommander"
	if runtime.GOOS == "darwin" {
		label = "About"
	}
	if help.Label != "Help" || len(help.Items) != 1 || help.Items[0].Label != label {
		t.Fatal("About missing from Help menu")
	}
	help.Items[0].Action()
	w := windowByTitle(t, a, "About DualCommander")
	help.Items[0].Action()
	if windowByTitle(t, a, "About DualCommander") != w || u.win.Canvas().Overlays().Top() == nil {
		t.Fatal("About must reuse its decorated window and block parent input")
	}
	var website *dialogButton
	labels := []string{}
	var iconFound bool
	for _, o := range test.LaidOutObjects(w.Content()) {
		if l, ok := o.(*widget.Label); ok {
			labels = append(labels, l.Text)
			if strings.Contains(l.Text, "two-pane file manager") && (l.Alignment != fyne.TextAlignCenter || l.Wrapping != fyne.TextWrapWord) {
				t.Fatal("About description must be centered and wrap within its margins")
			}
		}
		if image, ok := o.(*canvas.Image); ok && image.Resource == appIcon {
			iconFound = true
		}
		if b, ok := o.(*dialogButton); ok && b.Text == "https://dualcommander.github.io/" {
			website = b
		}
	}
	if website == nil {
		t.Fatal("website missing from About dialog")
	}
	if !iconFound || !slices.Contains(labels, "Version 0.1.0+abcdef123456") || !slices.Contains(labels, appName) || !strings.Contains(strings.Join(labels, "\n"), "two-pane file manager") {
		t.Fatalf("About content incomplete: %v", labels)
	}
	text := strings.Join(labels, "\n")
	findKey := "Ctrl+F / Alt+F7"
	if runtime.GOOS == "darwin" {
		findKey = "Cmd+F / Alt+F7"
	}
	for _, label := range []string{"Keyboard shortcuts", "Tab", "Switch active pane", "Space", "Mark item and show size", "Insert", "Mark or unmark item", "F4", "F5", "F6", "F2 / Shift+F6", "Rename selected item", "F7", "F8 / Delete", findKey, "Find files", "F10"} {
		if !slices.Contains(labels, label) {
			t.Fatalf("About shortcut missing: %q", label)
		}
	}
	if !strings.Contains(text, "Copy, move, and organize") || strings.Contains(text, "Fyne") || strings.Contains(text, "Built with") {
		t.Fatalf("About should describe user benefits, not implementation: %s", text)
	}
	buildVersion = "2.4.0"
	u.about()
	versionFound := false
	for _, o := range test.LaidOutObjects(w.Content()) {
		if l, ok := o.(*widget.Label); ok && l.Text == "Version 2.4.0" {
			versionFound = true
		}
	}
	if !versionFound {
		t.Fatal("release version ignored")
	}
	test.Tap(website)
	if a.openedURL == nil || a.openedURL.String() != "https://dualcommander.github.io/" {
		t.Fatalf("wrong website: %v", a.openedURL)
	}
	a.openErr = errors.New("browser unavailable")
	test.Tap(website)
	if !hasWindow(u.app, "Error") {
		t.Fatal("browser error not shown")
	}
	tapDialogButton(t, u, "OK")
	w.Close()
	if slices.Contains(a.Driver().AllWindows(), w) || windowByTitle(t, a, appName) == nil {
		t.Fatal("closing About must leave main window open")
	}
	config, err := png.DecodeConfig(bytes.NewReader(appIcon.Content()))
	if err != nil || config.Width != 1024 || config.Height != 1024 {
		t.Fatalf("invalid runtime icon: %+v, %v", config, err)
	}
	for _, ext := range []string{"ico", "icns"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "assets", "dualcommander."+ext))
		if err != nil || len(data) < 8 {
			t.Fatalf("missing native icon %s: %v", ext, err)
		}
		if ext == "ico" && (!bytes.Equal(data[:4], []byte{0, 0, 1, 0}) || binary.LittleEndian.Uint16(data[4:6]) != 7) {
			t.Fatal("Windows icon must contain seven sizes, 16–256px")
		}
		if ext == "icns" && (string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data)) {
			t.Fatal("invalid macOS icon")
		}
	}
	metadata, err := os.ReadFile("../../cmd/dualcommander/FyneApp.toml")
	if !strings.Contains(string(metadata), `Version = "`+appVersion+`"`) {
		t.Fatal("development version differs from package metadata")
	}
	if err != nil || !strings.Contains(string(metadata), `Name = "`+appName+`"`) || !strings.Contains(string(metadata), `ID = "`+appID+`"`) || !strings.Contains(string(metadata), `Icon = "../../assets/dualcommander.png"`) {
		t.Fatalf("package branding differs from runtime: %v", err)
	}
}

func TestListingSortAndHidden(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	want := []string{"..", "docs", "photos", "app.zip", "readme.md", "song.mp3"}
	if got := names(u.left); !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	u.toggleHidden()
	if got := names(u.left); !slices.Contains(got, ".secret") {
		t.Fatalf("hidden not shown: %v", got)
	}
}

func TestTypeToNavigate(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
	for _, p := range []*pane{u.left, u.right} {
		for _, brief := range []bool{false, true} {
			p.setBrief(brief)
			u.activate(p)
			test.Type(u.win.Canvas().Focused(), "Re")
			if p.current().name != "readme.md" || p.status.Text != "Search: Re" {
				t.Fatalf("prefix match: %q, %q", p.current().name, p.status.Text)
			}
			test.Type(u.win.Canvas().Focused(), "不存在")
			press(u, fyne.KeyBackspace)
			if p.searchPrefix != "Re不存" || p.current().name != "readme.md" {
				t.Fatal("unmatched search or Unicode Backspace changed cursor")
			}
			press(u, fyne.KeyEscape)
			if p.searchPrefix != "" || strings.Contains(p.status.Text, "Search:") || strings.Contains(p.status.Text, "\n") {
				t.Fatal("Escape must restore single-line details")
			}
			test.Type(u.win.Canvas().Focused(), "PH")
			if p.current().name != "photos" {
				t.Fatal("folder prefix did not match")
			}
			press(u, fyne.KeyReturn)
			if p.searchPrefix != "" {
				t.Fatal("directory navigation must clear search")
			}
			p.cd(filepath.Join(root, "src"), "")
			u.win.Canvas().Focus(p.pathBar)
			test.Type(p.pathBar, "xyz")
			if p.searchPrefix != "" {
				t.Fatal("path input triggered pane search")
			}
		}
	}
}

func TestPaneDetailsAndSelectionTotals(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	if err := os.Chmod(filepath.Join(src, "readme.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), src, src)
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		cursorTo(t, p, "readme.md")
		fi, err := os.Stat(filepath.Join(src, "readme.md"))
		if err != nil {
			t.Fatal(err)
		}
		if p.current().attrs != fi.Mode().String() || p.status.Text != "readme.md · 5 B · 2024-01-02 03:04 · "+fi.Mode().String() {
			t.Fatalf("file details: %q", p.status.Text)
		}
		r := newRowWidget(p, true)
		p.fillRow(p.cursor, r)
		attrs := r.content.(*fyne.Container).Objects[2].(*fyne.Container).Objects[2].(*fyne.Container).Objects[0].(*widget.Label)
		if attrs.Text != fi.Mode().String() {
			t.Fatalf("attributes column: %q", attrs.Text)
		}
		press(u, fyne.KeyInsert)
		cursorTo(t, p, "docs")
		press(u, fyne.KeyInsert)
		waitUntil(t, func() bool { return !p.sizeBusy })
		if got, want := p.status.Text, "Selected: 5 B of 9 B · Files: 1 of 3 · Folders: 1 of 2"; got != want {
			t.Fatalf("summary %q want %q", got, want)
		}
		p.markAll(true)
		if got := p.status.Text; got != "Selected: 9 B of 9 B · Files: 3 of 3 · Folders: 2 of 2" {
			t.Fatal(got)
		}
		p.markAll(false)
		if !strings.HasPrefix(p.status.Text, p.current().name+" · ") || strings.Contains(p.status.Text, "\n") {
			t.Fatalf("clearing selection must restore cursor details: %q", p.status.Text)
		}
		p.setBrief(true)
		cursorTo(t, p, "docs")
		press(u, fyne.KeySpace)
		waitUntil(t, func() bool { return !p.sizeBusy })
		if p.status.Text != "0 B · 0 files" || strings.Contains(p.status.Text, "\n") {
			t.Fatalf("folder details: %q", p.status.Text)
		}
		if err := p.cd(filepath.Join(root, "dst"), ""); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(p.status.Text, "Selected:") || !strings.HasPrefix(p.status.Text, p.current().name+" · ") || strings.Contains(p.status.Text, "\n") {
			t.Fatal(p.status.Text)
		}
	}
}

func TestFunctionKeyBar(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	cursorTo(t, u.active, "readme.md")
	bar := u.fkeyBar().(*fyne.Container)
	var labels []string
	for _, object := range bar.Objects {
		labels = append(labels, object.(*widget.Button).Text)
	}
	if !slices.Equal(labels, []string{"F4 Edit", "F5 Copy", "F6 Move", "F7 MkDir", "F8 Delete", "F9 Terminal", "F10 Quit"}) {
		t.Fatalf("unexpected function buttons: %v", labels)
	}
	w := test.NewWindow(bar)
	defer w.Close()
	w.Resize(fyne.NewSize(900, 50))
	assertUIText(t, w.Content(), labels...)
	for _, object := range bar.Objects {
		button := object.(*widget.Button)
		if button.Disabled() || button.OnTapped == nil || button.Size().Width <= 0 {
			t.Errorf("function button %q unavailable", button.Text)
		}
	}
}

func TestPathEditingDisablesFunctionActions(t *testing.T) {
	for _, right := range []bool{false, true} {
		for _, exit := range []string{"escape", "blur"} {
			t.Run(fmt.Sprintf("right=%v/%s", right, exit), func(t *testing.T) {
				root := fixture(t)
				u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
				p := u.left
				if right {
					p = u.right
				}
				menus := map[string]*fyne.MenuItem{}
				for _, item := range u.win.MainMenu().Items[0].Items {
					menus[item.Label] = item
				}
				check := func(disabled bool) {
					t.Helper()
					for index, label := range map[int]string{3: "New Folder", 5: "Open in Terminal", 6: "Quit"} {
						if u.functionBar.Objects[index].(*widget.Button).Disabled() != disabled {
							t.Errorf("%s button: want disabled=%v", label, disabled)
						}
						if menus[label] == nil || menus[label].Disabled != disabled {
							t.Errorf("%s menu: want disabled=%v", label, disabled)
						}
					}
				}
				u.win.Canvas().Focus(p.pathBar)
				p.pathBar.SetText("unfinished path")
				check(true)
				if t.Failed() {
					return
				}
				for _, label := range []string{"New Folder", "Open in Terminal", "Quit"} {
					menus[label].Action()
				}
				if u.passwordWindow != nil || p.pathBar.Text != "unfinished path" || u.win.Canvas().Focused() != p.pathBar {
					t.Fatal("disabled actions interrupted path editing")
				}
				if exit == "escape" {
					p.pathBar.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				} else {
					u.activate(p)
				}
				check(false)
			})
		}
	}
}

func TestFunctionButtonsFollowActiveCursor(t *testing.T) {
	for _, brief := range []bool{false, true} {
		t.Run(fmt.Sprintf("brief=%v", brief), func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
			buttons := map[string]*widget.Button{}
			menuItems := map[string]*fyne.MenuItem{}
			for _, item := range u.win.MainMenu().Items[0].Items {
				menuItems[item.Label] = item
			}
			for _, o := range laidOutObjects(u.win.Content()) {
				if b, ok := o.(*widget.Button); ok && strings.HasPrefix(b.Text, "F") {
					buttons[b.Text] = b
				}
			}
			check := func(edit, operations bool) {
				t.Helper()
				for label, enabled := range map[string]bool{"Edit": edit, "Copy…": operations, "Move…": operations, "Delete…": operations} {
					item := menuItems[label]
					if item == nil || item.Disabled == enabled {
						t.Errorf("menu %s: want enabled=%v, got %v", label, enabled, item)
					}
				}
				for label, enabled := range map[string]bool{
					"F4 Edit": edit, "F5 Copy": operations, "F6 Move": operations,
					"F8 Delete": operations, "F7 MkDir": true, "F9 Terminal": true, "F10 Quit": true,
				} {
					b := buttons[label]
					if b == nil || b.Disabled() == enabled {
						t.Errorf("%s: want enabled=%v, got %v", label, enabled, b)
					}
				}
			}
			cursorTo(t, u.active, "..")
			check(false, false)
			for _, p := range []*pane{u.left, u.right} {
				p.setBrief(brief)
				u.activate(p)
				cursorTo(t, p, "readme.md")
				check(true, true)
				cursorTo(t, u.other(), "..")
				check(true, true)
				cursorTo(t, p, "docs")
				check(false, true)
				p.markAll(true)
				cursorTo(t, p, "..")
				check(false, false)
				input := u.inputWindow
				windows := len(u.app.Driver().AllWindows())
				overlay := u.win.Canvas().Overlays().Top()
				for _, label := range []string{"Copy…", "Move…", "Delete…"} {
					menuItems[label].Action()
					if u.inputWindow != input || len(u.app.Driver().AllWindows()) != windows || u.win.Canvas().Overlays().Top() != overlay {
						t.Fatalf("disabled menu %s opened dialog", label)
					}
				}
				press(u, fyne.KeyDown)
				check(false, true)
				u.activate(u.other())
				check(false, false)
				u.activate(p)
				check(false, true)
				if err := p.cd(filepath.Join(root, "dst"), ""); err != nil {
					t.Fatal(err)
				}
				check(false, false)
			}
		})
	}
}

func TestBackspaceWithMarkedFiles(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			for _, search := range []bool{false, true} {
				root := fixture(t)
				u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
				p := u.left
				if right {
					p = u.right
				}
				p.setBrief(brief)
				u.activate(p)
				if search {
					test.Type(p.focusable(), "read")
				} else {
					cursorTo(t, p, "readme.md")
				}
				p.toggleMark()
				p.marked["song.mp3"] = true
				p.focusable().TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
				if p.path != root || p.current().name != "src" || len(p.marked) != 0 || p.searchPrefix != "" || u.win.Canvas().Focused() != p.focusable() {
					t.Fatalf("Backspace did not navigate up: brief=%v right=%v search=%v path=%s cursor=%s", brief, right, search, p.path, p.current().name)
				}
				if !exists(filepath.Join(root, "src", "readme.md")) || !exists(filepath.Join(root, "src", "song.mp3")) {
					t.Fatal("Backspace removed marked files")
				}
			}
		}
	}
}

func TestKeyboardNavigation(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	cursorTo(t, u.left, "photos")
	press(u, fyne.KeyReturn)
	if u.left.path != filepath.Join(root, "src", "photos") {
		t.Fatalf("enter: %s", u.left.path)
	}
	press(u, fyne.KeyBackspace)
	if u.left.current().name != "photos" {
		t.Fatalf("back should land on photos, got %q", u.left.current().name)
	}
	press(u, fyne.KeyTab)
	if u.active != u.right {
		t.Fatal("tab did not switch pane")
	}
	press(u, fyne.KeyTab)
	press(u, fyne.KeyEnd)
	if u.left.current().name != "song.mp3" {
		t.Fatalf("end: %q", u.left.current().name)
	}
}

func TestRepeatedArrowNavigation(t *testing.T) {
	for _, brief := range []bool{false, true} {
		root := fixture(t)
		u := newTestUI(t, test.NewTempApp(t), root, root)
		p := u.left
		p.setBrief(brief)
		p.entries = make([]entry, 200)
		for i := range p.entries {
			p.entries[i].name = fmt.Sprintf("file-%03d.txt", i)
		}
		p.list.Refresh()
		p.grid.Refresh()
		p.grid.Resize(fyne.NewSize(600, 400))
		u.activate(p)
		p.moveTo(0)
		if allocs := testing.AllocsPerRun(20, func() { p.previewVisible(0) }); allocs != 0 {
			t.Fatalf("brief=%v: viewport check allocates %g objects", brief, allocs)
		}
		// Cross column and viewport boundaries in both directions, including a partial column.
		steps := min(len(p.entries)-1, p.grid.rows*(p.grid.cols+1))
		for _, key := range []fyne.KeyName{fyne.KeyDown, fyne.KeyUp} {
			for i := 1; i <= steps; i++ {
				p.focusable().TypedKey(&fyne.KeyEvent{Name: key})
				want := i
				if key == fyne.KeyUp {
					want = steps - i
				}
				if p.cursor != want || !strings.HasPrefix(p.status.Text, p.entries[want].name+" · ") {
					t.Fatalf("brief=%v key=%s: cursor=%d want=%d status=%q", brief, key, p.cursor, want, p.status.Text)
				}
				if brief && !p.previewVisible(want) {
					t.Fatalf("cursor %d outside horizontal viewport", want)
				}
			}
		}
		if len(p.marked) != 0 || u.win.Canvas().Focused() != p.focusable() {
			t.Fatal("repeated arrows changed marks or focus")
		}
	}
}

func TestBriefNavigationClampsAtEdges(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, "src", "extra.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
	for _, p := range []*pane{u.left, u.right} {
		for _, brief := range []bool{false, true} {
			p.setBrief(brief)
			u.activate(p)
			for _, width := range []float32{400, 800, 1200} {
				p.grid.Resize(fyne.NewSize(width, 90))
				cols := p.cols()
				for i := range p.entries {
					for _, key := range []fyne.KeyName{fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown} {
						want := i
						if key == fyne.KeyUp {
							want = max(0, i-1)
						} else if key == fyne.KeyDown {
							want = min(i+1, len(p.entries)-1)
						} else if brief {
							if key == fyne.KeyLeft {
								want = max(0, i-p.grid.rows)
							} else {
								want = min(i+p.grid.rows, len(p.entries)-1)
							}
						}
						p.moveTo(i)
						p.focusable().TypedKey(&fyne.KeyEvent{Name: key})
						if p.cursor != want {
							t.Fatalf("brief=%v cols=%d index=%d key=%s: got %d want %d", brief, cols, i, key, p.cursor, want)
						}
					}
				}
			}
		}
	}
}

func TestPathEscapeDiscardsEdits(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			t.Run(fmt.Sprintf("brief=%v/right=%v", brief, right), func(t *testing.T) {
				root := fixture(t)
				u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
				p := u.left
				if right {
					p = u.right
				}
				p.setBrief(brief)
				cursorTo(t, p, "readme.md")
				p.marked["photos"] = true
				before := names(p)
				// Focusing either path field must return to its own file list.
				u.win.Canvas().Focus(p.pathBar)
				p.pathBar.SetText(filepath.Join(root, "dst"))
				u.win.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
				if p.pathBar.Text != p.path || p.path != filepath.Join(root, "src") || p.pathBar.editing {
					t.Fatal("Escape did not discard path edits")
				}
				if u.active != p || u.win.Canvas().Focused() != p.focusable() {
					t.Fatal("Escape did not return focus to owning pane")
				}
				if !slices.Equal(names(p), before) || p.current().name != "readme.md" || len(p.marked) != 1 || !p.marked["photos"] {
					t.Fatal("Escape changed directory contents, cursor, or selection")
				}
			})
		}
	}
}

func TestShortcutsRespectTextFocus(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	editMenu := u.win.MainMenu().Items[1]
	selectAll, deselectAll := editMenu.Items[0], editMenu.Items[1]
	if _, ok := selectAll.Shortcut.(*fyne.ShortcutSelectAll); !ok {
		t.Fatal("Select All must use native text shortcut")
	}
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		selectAll.Action()
		if len(p.marked) != len(p.entries)-1 {
			t.Fatal("pane Select All did not mark all entries")
		}
		u.win.Canvas().Focus(p.pathBar)
		deselectAll.Action()
		if len(p.marked) != len(p.entries)-1 {
			t.Fatal("text shortcut changed file marks")
		}
		p.markAll(false)
		selectAll.Action() // native menus invoke Action before widget shortcuts
		if p.pathBar.SelectedText() != p.path || len(p.marked) != 0 {
			t.Fatal("path Select All did not stay in text field")
		}
		u.win.Canvas().Focused().(fyne.Shortcutable).TypedShortcut(selectAll.Shortcut)
		if p.pathBar.SelectedText() != p.path || len(p.marked) != 0 {
			t.Fatal("widget Select All did not stay in text field")
		}
		u.win.MainMenu().Items[0].Items[0].Action() // Open must not open selected directory
		if p.path != p.pathBar.Text || u.win.Canvas().Focused() != p.pathBar {
			t.Fatal("file command ran while editing path")
		}
		u.activate(p)
		selectAll.Action()
		deselectAll.Action()
		if len(p.marked) != 0 {
			t.Fatal("pane Deselect All did not clear marks")
		}
	}
	u.mkdir()
	e := windowByTitle(t, u.app, "New directory").Canvas().Focused().(*escEntry)
	e.SetText("new folder")
	e.TypedShortcut(selectAll.Shortcut)
	if e.SelectedText() != e.Text || len(u.active.marked) != 0 {
		t.Fatal("dialog Select All did not stay in text field")
	}
	tapDialogButton(t, u, "Cancel")
}

func TestSwapPreservesPaneState(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, rightActive := range []bool{false, true} {
			root := fixture(t)
			if err := os.WriteFile(filepath.Join(root, "dst", "existing.txt"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			left, right := u.left, u.right
			left.setBrief(brief)
			right.setBrief(!brief)
			cursorTo(t, left, "readme.md")
			left.toggleMark()
			cursorTo(t, left, "song.mp3")
			cursorTo(t, right, "existing.txt")
			right.toggleMark()
			cursorTo(t, right, "existing.txt")
			left.searchPrefix = "song"
			active := left
			if rightActive {
				active = right
			}
			u.activate(active)
			leftCursor, rightCursor := left.cursor, right.cursor
			var swap *fyne.MenuItem
			for _, menu := range u.win.MainMenu().Items {
				for _, item := range menu.Items {
					if item.Label == "Swap Panes" {
						swap = item
					}
				}
			}
			if swap == nil {
				t.Fatal("Swap Panes menu missing")
			}
			for i := 0; i < 2; i++ {
				swap.Action()
				wantLeft, wantRight := right, left
				if i == 1 {
					wantLeft, wantRight = left, right
				}
				if u.left != wantLeft || u.right != wantRight || u.split.Leading != wantLeft.box || u.split.Trailing != wantRight.box {
					t.Fatal("swap did not exchange complete panes")
				}
				if left.cursor != leftCursor || right.cursor != rightCursor || !left.marked["readme.md"] || !right.marked["existing.txt"] || left.searchPrefix != "song" {
					t.Fatal("swap lost cursor, marks, or search state")
				}
				if left.brief != brief || right.brief == brief || u.active != active || u.win.Canvas().Focused() != active.focusable() {
					t.Fatal("swap lost view mode or active pane focus")
				}
				pr := u.app.Preferences()
				if pr.String(prefLeft) != wantLeft.path || pr.String(prefRight) != wantRight.path {
					t.Fatal("swapped paths not saved")
				}
			}
		}
	}
}

func TestBriefMouseWheelScroll(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	p := u.left
	p.setBrief(true)
	p.entries = make([]entry, 100)
	for i := range p.entries {
		p.entries[i].name = fmt.Sprintf("file%03d.txt", i)
	}
	u.win.SetContent(p.grid)
	u.win.Resize(fyne.NewSize(500, 200))
	p.grid.Refresh()
	p.moveTo(0)
	p.marked[p.current().name] = true
	g := p.grid
	u.win.Canvas().Focus(g)
	if g.scroll.Content.MinSize().Width <= g.scroll.Size().Width {
		t.Fatal("test requires overflowing columns")
	}
	pos := u.app.Driver().AbsolutePositionForObject(g).Add(fyne.NewPos(20, 20))
	for _, delta := range []fyne.Delta{
		{DY: -40}, {DY: 20}, {DX: -30}, {DX: 10, DY: -80},
		{DY: -100000}, {DY: -40}, {DY: 100000}, {DY: 40},
	} {
		before := g.scroll.Offset.X
		dx := delta.DX
		if dx == 0 {
			dx = delta.DY
		}
		limit := max(float32(0), g.scroll.Content.MinSize().Width-g.scroll.Size().Width)
		want := min(limit, max(float32(0), before-dx))
		test.Scroll(u.win.Canvas(), pos, delta.DX, delta.DY)
		if g.scroll.Offset.X != want || g.scroll.Offset.Y != 0 {
			t.Fatalf("wheel %v: offset=%v want=(%v, 0)", delta, g.scroll.Offset, want)
		}
		if p.cursor != 0 || len(p.marked) != 1 || !p.marked[p.entries[0].name] || u.win.Canvas().Focused() != g {
			t.Fatal("scroll changed cursor, marks, or focus")
		}
	}
}

func TestBriefHomeEndPartialColumn(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		u.activate(p)
		p.list.OnSelected = func(int) { t.Fatal("brief navigation selected hidden detailed view") }
		g := p.grid
		cell, pad := g.MinSize(), theme.Padding()
		for _, count := range []int{9, 10, 11, 12, 13, 31} {
			p.entries = make([]entry, count)
			g.Resize(fyne.NewSize(2*(cell.Width+pad)-pad, 4*(cell.Height+pad)-pad))
			for start := range p.entries {
				p.moveTo(start)
				for _, key := range []fyne.KeyName{fyne.KeyEnd, fyne.KeyHome} {
					u.win.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: key})
					want := 0
					if key == fyne.KeyEnd {
						want = count - 1
					}
					if p.cursor != want || !p.previewVisible(want) {
						t.Fatalf("count=%d start=%d key=%s: cursor=%d want=%d", count, start, key, p.cursor, want)
					}
					var selected widget.TableCellID
					callback := g.OnUnselected
					g.OnUnselected = func(id widget.TableCellID) { selected = id }
					g.UnselectAll()
					g.OnUnselected = callback
					if selected != g.slot(want) {
						t.Fatalf("count=%d start=%d key=%s: selected=%v want=%v", count, start, key, selected, g.slot(want))
					}
					p.showCursor()
				}
			}
		}
	}
}

func TestBriefPageKeysPartialColumn(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		u.activate(p)
		g := p.grid
		cell, pad := g.MinSize(), theme.Padding()
		for _, count := range []int{9, 10, 11, 12, 13, 31} {
			p.entries = make([]entry, count)
			g.Resize(fyne.NewSize(2*(cell.Width+pad)-pad, 4*(cell.Height+pad)-pad))
			for _, fallback := range []bool{false, true} {
				for start := range p.entries {
					for _, key := range []fyne.KeyName{fyne.KeyPageDown, fyne.KeyPageUp} {
						p.moveTo(start)
						event := &fyne.KeyEvent{Name: key}
						if fallback {
							u.win.Canvas().Unfocus()
							u.win.Canvas().OnTypedKey()(event)
						} else {
							u.win.Canvas().Focus(g)
							g.TypedKey(event)
						}
						want := min(count-1, start+g.rows*g.cols)
						if key == fyne.KeyPageUp {
							want = max(0, start-g.rows*g.cols)
						}
						if p.cursor != want || !p.previewVisible(want) || g.scroll.Offset.Y != 0 {
							t.Fatalf("count=%d start=%d key=%s fallback=%v: cursor=%d want=%d", count, start, key, fallback, p.cursor, want)
						}
					}
				}
			}
		}
	}
}

func TestTabSelectsParentInEmptyPane(t *testing.T) {
	for _, brief := range []bool{false, true} {
		for _, side := range []string{"left", "right"} {
			t.Run(fmt.Sprintf("brief=%v/active=%s", brief, side), func(t *testing.T) {
				root := fixture(t)
				a := test.NewTempApp(t)
				a.Preferences().SetBool(prefLeftBrief, brief)
				a.Preferences().SetBool(prefRightBrief, brief)
				a.Preferences().SetString(prefActive, side)
				u := newTestUI(t, a, filepath.Join(root, "src", "docs"), filepath.Join(root, "dst"))
				for range 4 {
					press(u, fyne.KeyTab)
					p := u.active
					if len(p.entries) != 1 || p.current().name != ".." || p.cursor != 0 || u.win.Canvas().Focused() != p.focusable() {
						t.Fatal("Tab did not focus empty pane's parent entry")
					}
					visible := false
					view := fyne.CanvasObject(p.list)
					if brief {
						view = p.grid
					}
					var visit func(fyne.CanvasObject)
					visit = func(o fyne.CanvasObject) {
						if !o.Visible() {
							return
						}
						if r, ok := o.(*canvas.Rectangle); ok && r.FillColor == theme.Color(theme.ColorNameSelection) && r.Size().Width > 0 && r.Size().Height > 0 {
							visible = true
						}
						switch obj := o.(type) {
						case *fyne.Container:
							for _, child := range obj.Objects {
								visit(child)
							}
						case fyne.Widget:
							for _, child := range test.WidgetRenderer(obj).Objects() {
								visit(child)
							}
						}
					}
					visit(view)
					if !visible || len(p.marked) != 0 {
						t.Fatal("parent selection indicator missing or marks changed")
					}
				}
			})
		}
	}
}

func TestBriefCursorIndicatorVisible(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	p.setBrief(true)
	u.activate(p)
	for _, key := range []fyne.KeyName{fyne.KeyEnd, fyne.KeyHome, fyne.KeyDown} {
		press(u, key)
		visible := false
		var visit func(fyne.CanvasObject)
		visit = func(o fyne.CanvasObject) {
			if !o.Visible() {
				return
			}
			if r, ok := o.(*canvas.Rectangle); ok && r.FillColor == theme.Color(theme.ColorNameSelection) {
				visible = r.Size().Width > 0 && r.Size().Height > 0
			}
			switch obj := o.(type) {
			case *fyne.Container:
				for _, child := range obj.Objects {
					visit(child)
				}
			case fyne.Widget:
				for _, child := range test.WidgetRenderer(obj).Objects() {
					visit(child)
				}
			}
		}
		visit(p.grid)
		if !visible || len(p.marked) != 0 {
			t.Fatalf("key=%s: cursor indicator hidden or multi-selection changed", key)
		}
	}
}

func TestBriefEmptyCellsDoNotHover(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		u.activate(p)
		g := p.grid
		cell, pad := g.MinSize(), theme.Padding()
		p.entries = make([]entry, 5)
		g.Resize(fyne.NewSize(2*(cell.Width+pad)-pad, 4*(cell.Height+pad)-pad))
		p.moveTo(0)
		u.win.Canvas().Unfocus() // Isolate mouse hover from the keyboard cursor marker.
		p.marked["kept"] = true
		hoverVisible := func() bool {
			for _, o := range test.LaidOutObjects(g) {
				if r, ok := o.(*canvas.Rectangle); ok && r.Visible() && r.FillColor == theme.Color(theme.ColorNameHover) {
					return true
				}
			}
			return false
		}
		for row := 1; row < g.rows; row++ {
			g.MouseIn(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(g.cell.Width+pad+1, 1)}})
			if !hoverVisible() {
				t.Fatal("populated cell must retain hover highlight")
			}
			g.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(g.cell.Width+pad+1, float32(row)*(g.cell.Height+pad)+1)}})
			if hoverVisible() || p.cursor != 0 || !p.marked["kept"] {
				t.Fatal("empty cell must clear hover without changing cursor or marks")
			}
		}
	}
}

func TestBriefHoverFollowsMouseWithFocus(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		g := p.grid
		p.entries = make([]entry, 5)
		cell, pad := g.MinSize(), theme.Padding()
		g.Resize(fyne.NewSize(2*(cell.Width+pad)-pad, 4*(cell.Height+pad)-pad))
		for _, active := range []bool{true, false} {
			u.activate(p)
			p.moveTo(0)
			if !active {
				u.activate(u.other())
			}
			p.marked["kept"] = true
			hover := func() *canvas.Rectangle {
				for _, o := range test.LaidOutObjects(g) {
					if r, ok := o.(*canvas.Rectangle); ok && r.Visible() && r.FillColor == theme.Color(theme.ColorNameHover) {
						return r
					}
				}
				return nil
			}
			g.MouseIn(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(1, g.cell.Height+pad+1)}})
			if marker := hover(); marker == nil || marker.Position().Y < g.cell.Height {
				t.Fatalf("active=%v: hover did not follow mouse to second row", active)
			}
			g.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(g.cell.Width+pad+1, 1)}})
			if marker := hover(); marker == nil || marker.Position().X < g.cell.Width || marker.Position().Y != 0 {
				t.Fatalf("active=%v: hover did not follow mouse to next column", active)
			}
			g.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(g.cell.Width+pad+1, g.cell.Height+pad+1)}})
			if hover() != nil {
				t.Fatalf("active=%v: empty cell highlighted", active)
			}
			g.MouseOut()
			if hover() != nil || p.cursor != 0 || !p.marked["kept"] || (u.win.Canvas().Focused() == g) != active {
				t.Fatal("hover changed cursor, marks, or focus, or remained after exit")
			}
		}
	}
}

func TestBriefScrollsHorizontally(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		u.activate(p)
		p.entries = make([]entry, 31)
		g := p.grid
		cell, pad := g.MinSize(), theme.Padding()
		g.Resize(fyne.NewSize(2*(cell.Width+pad)-pad, 4*(cell.Height+pad)-pad))
		rows, cols := g.Length()
		if rows != 4 || cols != 8 || p.cols() != 2 {
			t.Fatalf("wrong column flow: rows=%d total columns=%d visible columns=%d", rows, cols, p.cols())
		}
		if g.scroll.Direction != container.ScrollHorizontalOnly || g.scroll.Content.MinSize().Height > g.Size().Height {
			t.Fatal("brief content must fit viewport height and scroll horizontally only")
		}
		p.moveTo(0)
		press(u, fyne.KeyRight)
		if p.cursor != rows || g.scroll.Offset.X != 0 {
			t.Fatal("Right must move to next visible column")
		}
		press(u, fyne.KeyEnd)
		if p.cursor != len(p.entries)-1 || g.scroll.Offset.X <= 0 || g.scroll.Offset.Y != 0 || !p.previewVisible(p.cursor) || p.previewVisible(0) {
			t.Fatal("End must reveal last column horizontally, including preview visibility")
		}
		press(u, fyne.KeyHome)
		if g.scroll.Offset != (fyne.Position{}) {
			t.Fatal("Home must return to first column")
		}
		g.scroll.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -cell.Width}})
		if g.scroll.Offset.X <= 0 || g.scroll.Offset.Y != 0 {
			t.Fatal("mouse wheel must scroll columns horizontally")
		}
		g.Resize(fyne.NewSize(g.Size().Width, 6*(cell.Height+pad)-pad))
		rows, cols = g.Length()
		if rows != 6 || cols != 6 || g.scroll.Offset.Y != 0 {
			t.Fatal("resize must reflow columns without vertical scrolling")
		}
		g.UpdateCell(widget.TableCellID{Row: 5, Col: 5}, g.CreateCell())
	}
}

func TestBriefColumnsFollowVisibleCapacity(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		p.setBrief(true)
		cell := newRowWidget(p, false).MinSize()
		pad := p.grid.Theme().Size(theme.SizeNamePadding)
		for _, tc := range []struct{ count, rows, width, want int }{
			{3, 5, 3, 1}, {5, 5, 3, 1}, {6, 5, 3, 2},
			{10, 5, 3, 2}, {11, 5, 3, 3}, {16, 5, 3, 3},
			{11, 6, 3, 2}, {11, 11, 3, 1}, {11, 5, 1, 1}, {0, 5, 3, 1},
		} {
			p.entries = make([]entry, tc.count)
			p.grid.Refresh() // directory contents changed without resizing
			p.grid.Resize(fyne.NewSize(float32(tc.width)*(cell.Width+pad)-pad, float32(tc.rows)*(cell.Height+pad)-pad))
			if got := p.cols(); got != tc.want {
				t.Fatalf("count=%d rows=%d width=%d: columns=%d want %d", tc.count, tc.rows, tc.width, got, tc.want)
			}
			if p.grid.MinSize().Width != cell.Width {
				t.Fatal("expanded columns prevent pane shrinking")
			}
			for id := range p.entries {
				slot := p.grid.slot(id)
				rows, cols := p.grid.Length()
				if slot.Row >= rows || slot.Col >= cols || p.grid.entryID(slot) != id {
					t.Fatalf("entry %d missing from grid: slot=%v size=%dx%d", id, slot, rows, cols)
				}
				r := p.grid.CreateCell().(*row)
				p.grid.UpdateCell(slot, r)
				if r.id != id || !r.Visible() {
					t.Fatalf("entry %d not rendered", id)
				}
			}
			rows, cols := p.grid.Length()
			for index := 0; index < rows*cols; index++ {
				slot := widget.TableCellID{Row: index % rows, Col: index / rows}
				if p.grid.entryID(slot) >= len(p.entries) {
					r := p.grid.CreateCell().(*row)
					p.grid.UpdateCell(slot, r)
					if r.Visible() {
						t.Fatal("empty slot shows stale entry")
					}
				}
			}
		}
		p.entries = make([]entry, 6)
		p.grid.Resize(fyne.NewSize(3*(cell.Width+pad)-pad, 5*(cell.Height+pad)-pad))
		for _, count := range []int{3, 11, 6} {
			p.entries = make([]entry, count)
			p.grid.Refresh()
			if got, want := p.cols(), (count+4)/5; got != want {
				t.Fatalf("refresh count=%d: columns=%d want %d", count, got, want)
			}
		}
	}
}

func TestBriefColumnsFitLongestName(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		p.setBrief(true)
		base := p.grid.MinSize()
		pad := theme.Padding()
		p.entries = make([]entry, 6)
		long := strings.Repeat("W", 30) + "世界.txt"
		p.entries[0].name = long
		width := widget.NewLabel(long).MinSize().Width + theme.IconInlineSize() + pad
		p.grid.Resize(fyne.NewSize(2*(width+pad), 3*(base.Height+pad)-pad))
		if p.grid.cell.Width < width || p.cols() != 2 || p.grid.rows != 3 {
			t.Fatalf("long name: cell=%v width=%v cols=%d rows=%d", p.grid.cell, width, p.cols(), p.grid.rows)
		}
		p.moveTo(1)
		press(u, fyne.KeyRight)
		if p.cursor != 4 {
			t.Fatalf("right after width change: cursor=%d", p.cursor)
		}
		p.entries[0].name = "short"
		p.grid.Resize(fyne.NewSize(2*(base.Width+pad)-pad, 3*(base.Height+pad)-pad))
		if p.cols() != 2 {
			t.Fatal("columns did not shrink after removing long name")
		}
		p.entries[0].name = long
		p.grid.Refresh()
		if p.cols() != 2 || p.grid.cell.Width > (p.grid.Size().Width-pad)/2 || p.grid.MinSize() != base {
			t.Fatal("long name must truncate without collapsing columns or increasing minimum size")
		}
		p.entries[0].name = "short"
		p.grid.Refresh()
		if p.cols() != 2 {
			t.Fatal("refresh did not restore narrower columns")
		}
	}
}

func TestBriefLongNamesAcrossFolders(t *testing.T) {
	root := t.TempDir()
	for _, folder := range []string{"short", "long"} {
		dir := filepath.Join(root, folder)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 20; i++ {
			name := string(rune('a'+i)) + ".txt"
			if folder == "long" && i == 0 {
				name = strings.Repeat("long-name-", 20) + "世界.txt"
			}
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	u := newTestUI(t, test.NewTempApp(t), root, root)
	p := u.left
	p.setBrief(true)
	u.activate(p)
	g := p.grid
	base, pad := g.MinSize(), theme.Padding()
	g.Resize(fyne.NewSize(3*(base.Width+pad)-pad, 4*(base.Height+pad)-pad))
	for _, folder := range []string{"short", "long", "short", "long"} {
		if err := p.cd(filepath.Join(root, folder), ""); err != nil {
			t.Fatal(err)
		}
		if p.cols() < 2 || g.rows != 4 || g.scroll.Content.MinSize().Height > g.Size().Height {
			t.Fatalf("%s: long filenames collapsed columns or introduced vertical overflow", folder)
		}
		press(u, fyne.KeyEnd)
		if g.scroll.Offset.X <= 0 || g.scroll.Offset.Y != 0 || !p.previewVisible(p.cursor) {
			t.Fatalf("%s: last column not revealed horizontally", folder)
		}
		press(u, fyne.KeyHome)
		press(u, fyne.KeyRight)
		if p.cursor != g.rows {
			t.Fatalf("%s: column navigation changed with filename length", folder)
		}
	}
}

func TestEditMenuShortcut(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	var editItem *fyne.MenuItem
	for _, item := range u.win.MainMenu().Items[0].Items {
		if item.Label == "Edit (F4)" {
			t.Fatal("shortcut remains in menu label")
		}
		if item.Label == "Edit" {
			editItem = item
		}
	}
	if editItem == nil || editItem.Action == nil {
		t.Fatal("Edit menu action missing")
	}
	shortcut, ok := editItem.Shortcut.(*desktop.CustomShortcut)
	if !ok || shortcut.KeyName != fyne.KeyF4 || shortcut.Modifier != 0 {
		t.Fatalf("Edit shortcut: %#v", editItem.Shortcut)
	}
}

func TestFileOperationMenuShortcuts(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	file := u.win.MainMenu().Items[0]
	for _, tc := range []struct {
		label    string
		key      fyne.KeyName
		modifier fyne.KeyModifier
		title    string
	}{
		{"Copy…", fyne.KeyF5, 0, "Copy 1 item(s) to"},
		{"New Folder", fyne.KeyF7, 0, "New directory"},
		{"Move…", fyne.KeyF6, 0, "Move/Rename 1 item(s) to"},
		{"Rename…", fyne.KeyF6, fyne.KeyModifierShift, "Rename"},
		{"Delete…", fyne.KeyF8, 0, "Move to " + trashName()},
	} {
		t.Run(tc.label, func(t *testing.T) {
			var found *fyne.MenuItem
			for _, item := range file.Items {
				if item.Label == "Move / Rename…" {
					t.Fatal("combined Move / Rename item remains")
				}
				if item.Label == tc.label {
					found = item
				}
			}
			if found == nil {
				t.Fatal("menu item missing")
			}
			shortcut, ok := found.Shortcut.(*desktop.CustomShortcut)
			if !ok || shortcut.KeyName != tc.key || shortcut.Modifier != tc.modifier {
				t.Fatalf("shortcut: %#v", found.Shortcut)
			}
			found.Action()
			windowByTitle(t, u.app, tc.title)
			tapDialogButton(t, u, "Cancel")
			if !exists(filepath.Join(root, "src", "readme.md")) || exists(filepath.Join(root, "dst", "readme.md")) {
				t.Fatal("cancel changed files")
			}
		})
	}
}

func TestMarkAndCopyWorkflow(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	press(u, fyne.KeyInsert) // marks readme, cursor to song
	press(u, fyne.KeyInsert) // marks song
	if len(u.left.selection()) != 2 {
		t.Fatalf("marked: %v", u.left.selection())
	}
	press(u, fyne.KeyF5)
	tapDialogButton(t, u, "Copy")
	waitJobs(t, u)
	for _, n := range []string{"readme.md", "song.mp3"} {
		if !exists(filepath.Join(root, "dst", n)) {
			t.Fatalf("%s not copied", n)
		}
	}
	if !slices.Equal(names(u.right), []string{"..", "readme.md", "song.mp3"}) {
		t.Fatalf("target pane not refreshed: %v", names(u.right))
	}
	if len(u.left.marked) != 0 {
		t.Fatal("marks not cleared")
	}
}

func TestMoveDeleteMkdirWorkflow(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))

	cursorTo(t, u.left, "docs")
	press(u, fyne.KeyF6)
	tapDialogButton(t, u, "Move/Rename")
	waitJobs(t, u)
	if exists(filepath.Join(root, "src", "docs")) || !exists(filepath.Join(root, "dst", "docs")) {
		t.Fatal("move failed")
	}

	cursorTo(t, u.left, "app.zip")
	press(u, fyne.KeyF8)
	tapDialogButton(t, u, "Cancel")
	if !exists(filepath.Join(root, "src", "app.zip")) {
		t.Fatal("deleted despite Cancel")
	}
	trashed.Store(false)
	press(u, fyne.KeyF8)
	tapDialogButton(t, u, "Move to "+trashName())
	waitJobs(t, u)
	if exists(filepath.Join(root, "src", "app.zip")) || !trashed.Load() {
		t.Fatal("not moved to trash (default)")
	}
	// Settings off: permanent delete, no trash.
	u.settings()
	w := windowByTitle(t, u.app, "Settings")
	foundTrash := false
	for _, o := range test.LaidOutObjects(w.Content()) {
		if c, ok := o.(*dialogCheck); ok && c.Text == "Move deleted files to "+trashName() {
			foundTrash = true
			if !c.Checked {
				t.Fatal("trash not default in settings")
			}
			test.Tap(c)
		}
	}
	if !foundTrash {
		t.Fatal("trash setting missing")
	}
	w.Close()
	trashed.Store(false)
	cursorTo(t, u.left, "song.mp3")
	press(u, fyne.KeyF8)
	tapDialogButton(t, u, "Delete")
	waitJobs(t, u)
	if exists(filepath.Join(root, "src", "song.mp3")) || trashed.Load() {
		t.Fatal("permanent delete failed or used trash")
	}

	press(u, fyne.KeyF7)
	e := windowByTitle(t, u.app, "New directory").Canvas().Focused()
	test.Type(e, "new")
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // Enter submits
	if !exists(filepath.Join(root, "src", "new")) || u.left.current().name != "new" {
		t.Fatalf("mkdir failed, cursor %q", u.left.current().name)
	}
	if u.win.Canvas().Focused() != u.left.focusable() {
		t.Fatal("focus not returned to pane after dialog")
	}
}

func TestStateRestoredAfterRestart(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), root)
	u.left.cd(filepath.Join(root, "src", "photos"), "")
	u.right.setBrief(true)
	u.activate(u.right)
	u.setTheme("dark")
	u.save()

	u2 := newTrackedUI(t, a) // same prefs store = next launch
	if u2.left.path != filepath.Join(root, "src", "photos") || !u2.right.brief || u2.active != u2.right || u2.themeMode != "dark" {
		t.Fatalf("not restored: %s brief=%v right-active=%v theme=%s",
			u2.left.path, u2.right.brief, u2.active == u2.right, u2.themeMode)
	}
}

func TestMissingSavedPathFallsBackHome(t *testing.T) {
	u := newTestUI(t, test.NewTempApp(t), "/definitely/not/here", "/definitely/not/here")
	if home, _ := os.UserHomeDir(); u.left.path != home {
		t.Fatalf("got %s", u.left.path)
	}
}

func TestCurrentFolderActions(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "src"))
	u.app.Preferences().SetString(prefTerminalCmd, "invalid")
	u.app.Preferences().SetString(prefRevealCmd, `["file-manager","{path}"]`)
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
		cursorTo(t, p, "readme.md")
		cmd, err := u.externalCommand(false)
		if err != nil || !slices.Equal(cmd.Args, []string{"file-manager", filepath.Join(p.path, "readme.md")}) {
			t.Fatalf("reveal must target selected item: %v, %v", cmd, err)
		}
		p.moveTo(0) // ".." is not a selected item; reveal falls back to current folder.
		cmd, err = u.externalCommand(false)
		if err != nil || !slices.Equal(cmd.Args, []string{"file-manager", p.path}) {
			t.Fatalf("reveal must fall back to current folder: %v, %v", cmd, err)
		}
		cursorTo(t, p, "readme.md")
		p.toggleMark()
		cursor := p.cursor
		// Exercise real key and button callbacks without launching native apps.
		press(u, fyne.KeyF9)
		if !hasWindow(u.app, "Error") {
			t.Fatal("F9 did not invoke terminal action")
		}
		tapDialogButton(t, u, "OK")
		test.Tap(u.fkeyBar().(*fyne.Container).Objects[5].(*widget.Button))
		if !hasWindow(u.app, "Error") {
			t.Fatal("F9 button did not invoke terminal action")
		}
		tapDialogButton(t, u, "OK")
		found := 0
		for _, item := range u.win.MainMenu().Items[0].Items {
			if strings.HasPrefix(item.Label, "Open Folder in ") {
				t.Fatal("duplicate file-manager action")
			}
			if item.Label != revealLabel() {
				continue
			}
			found++
			shortcut, ok := item.Shortcut.(*desktop.CustomShortcut)
			if !ok || shortcut.KeyName != fyne.KeyR || shortcut.Modifier != fyne.KeyModifierShortcutDefault|fyne.KeyModifierShift {
				t.Fatal("reveal action missing shortcut")
			}
			// Archive folders have no native directory; don't launch the OS app.
			path := p.path
			p.path = filepath.Join(root, "src", "app.zip", "docs")
			press(u, fyne.KeyF9)
			if !hasWindow(u.app, "Error") {
				t.Fatal("terminal accepted archive directory")
			}
			tapDialogButton(t, u, "OK")
			item.Action()
			if !hasWindow(u.app, "Error") {
				t.Fatal("reveal action accepted archive directory")
			}
			tapDialogButton(t, u, "OK")
			p.path = path
		}
		if found != 1 || p.cursor != cursor || !p.marked["readme.md"] {
			t.Fatal("expected one reveal action and unchanged pane state")
		}
	}
}

func TestPaneUIContent(t *testing.T) {
	root := fixture(t)
	// Permissions must not depend on umask; parent metadata must not depend on tmp directory creation.
	for _, name := range []string{"docs", "photos", "app.zip", "readme.md", "song.mp3"} {
		mode := os.FileMode(0o644)
		if name == "docs" || name == "photos" {
			mode = 0o755
		}
		if err := os.Chmod(filepath.Join(root, "src", name), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		brief   bool
		variant fyne.ThemeVariant
		height  float32
	}{
		{"detailed_light", false, theme.VariantLight, 280},
		{"detailed_compact", false, theme.VariantLight, 280},
		{"detailed_dark", false, theme.VariantDark, 280},
		{"brief_light", true, theme.VariantLight, 280},
		{"brief_two_columns", true, theme.VariantLight, 122},
		{"brief_partial_column", true, theme.VariantLight, 122},
		{"brief_long_names", true, theme.VariantLight, 122},
		{"details_footer", false, theme.VariantLight, 400},
		{"cursor_footer", false, theme.VariantLight, 400},
		{"search_footer", false, theme.VariantLight, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			u := newTestUI(t, a, filepath.Join(root, "src"), root)
			if tc.name == "detailed_compact" {
				u.setDensity("compact")
			}
			u.left.entries[0].attrs, u.left.entries[0].mod = "drwxr-xr-x", u.left.entries[1].mod
			applyTheme(a, map[fyne.ThemeVariant]string{theme.VariantLight: "light", theme.VariantDark: "dark"}[tc.variant], u.density)
			u.left.setBrief(tc.brief)
			if tc.name == "brief_partial_column" {
				u.left.entries = u.left.entries[:len(u.left.entries)-1]
				u.left.grid.Refresh()
			}
			cursorTo(t, u.left, "readme.md")
			u.left.toggleMark()
			if tc.name == "brief_long_names" {
				u.left.entries[1].name = "Folder with a much longer name"
				u.left.grid.Refresh()
			}
			w := test.NewWindow(u.left.body) // list only: path bar shows random tmp dir
			defer w.Close()
			w.Resize(fyne.NewSize(480, tc.height))
			if tc.name == "details_footer" || tc.name == "cursor_footer" || tc.name == "search_footer" {
				u.left.markAll(tc.name == "details_footer")
				cursorTo(t, u.left, "readme.md")
				if tc.name == "search_footer" {
					test.Type(u.left.list, "read")
				}
				w.SetContent(container.NewBorder(nil, container.NewVBox(container.NewStack(u.left.status, u.left.statusRow), u.fkeyBar()), nil, nil, u.left.body))
				w.Resize(fyne.NewSize(800, tc.height))
			}
			if tc.name == "brief_two_columns" || tc.name == "brief_partial_column" || tc.name == "brief_long_names" {
				w.Resize(fyne.NewSize(800, tc.height))
				if got := u.left.cols(); got != 2 {
					t.Fatalf("expected two columns, got %d", got)
				}
			}
			assertPaneRows(t, u.left)
			assertUIText(t, w.Content(), names(u.left)...)
			switch tc.name {
			case "details_footer":
				assertUIText(t, w.Content(), "Selected: 9 B of 9 B · Files: 3 of 3 · Folders: 2 of 2", "F5 Copy", "F8 Delete")
			case "cursor_footer":
				info, err := os.Stat(filepath.Join(root, "src", "readme.md"))
				if err != nil {
					t.Fatal(err)
				}
				assertUIText(t, u.left.statusRow, "readme.md", "5 B", "2024-01-02 03:04", info.Mode().String())
				assertUIText(t, w.Content(), "F5 Copy")
			case "search_footer":
				assertUIText(t, w.Content(), "Search: read", "F5 Copy")
				if u.left.current().name != "readme.md" {
					t.Fatal("search did not select matching row")
				}
			}
		})
	}
}
