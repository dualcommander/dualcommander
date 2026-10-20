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
	"runtime"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestExternalCommandsSettings(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.settings()
	w := windowByTitle(t, a, "Settings")
	u.settings()
	if windowByTitle(t, a, "Settings") != w || u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("settings must reuse its own window, not an overlay")
	}
	entries := []*escEntry{}
	labels := []string{}
	for _, o := range test.LaidOutObjects(w.Content()) {
		if l, ok := o.(*widget.Label); ok {
			labels = append(labels, l.Text)
		}
		if e, ok := o.(*escEntry); ok {
			entries = append(entries, e)
		}
	}
	if len(entries) != 2 {
		t.Fatalf("command fields: %d", len(entries))
	}
	text := strings.Join(labels, "\n")
	if !strings.Contains(text, "Changes are saved automatically") || !strings.Contains(text, "deleted files cannot be restored") || !strings.Contains(text, "Leave a field blank") {
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
	w.Close()
	if slices.Contains(a.Driver().AllWindows(), w) || windowByTitle(t, a, appName) == nil {
		t.Fatal("closing settings must leave main window open")
	}
	u = newUI(a)
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
	cmd, err := u.externalCommand(false)
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
	if windowByTitle(t, a, "About DualCommander") != w || u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("About must reuse its own window, not an overlay")
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
	for _, label := range []string{"Keyboard shortcuts", "Tab", "Switch active pane", "Space / Insert", "F4", "F5", "F6", "F2 / Shift+F6", "Rename selected item", "F7", "F8 / Delete", findKey, "Find files", "F10"} {
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
		if p.current().attrs != fi.Mode().String() || p.status.Text != "readme.md · "+fi.Mode().String()+" · 5 B · 2024-01-02 03:04" {
			t.Fatalf("file details: %q", p.status.Text)
		}
		r := newRowWidget(p, true)
		p.fillRow(p.cursor, r)
		attrs := r.content.(*fyne.Container).Objects[2].(*fyne.Container).Objects[2].(*fyne.Container).Objects[0].(*widget.Label)
		if attrs.Text != fi.Mode().String() {
			t.Fatalf("attributes column: %q", attrs.Text)
		}
		press(u, fyne.KeySpace)
		cursorTo(t, p, "docs")
		press(u, fyne.KeySpace)
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
		if !strings.HasPrefix(p.status.Text, "docs · d") || !strings.Contains(p.status.Text, "<DIR>") || strings.Contains(p.status.Text, "\n") {
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

func TestMarkAndCopyWorkflow(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	press(u, fyne.KeySpace) // marks readme, cursor to song
	press(u, fyne.KeySpace) // marks song
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
	tapDialogButton(t, u, "No")
	if !exists(filepath.Join(root, "src", "app.zip")) {
		t.Fatal("deleted despite No")
	}
	trashed.Store(false)
	press(u, fyne.KeyF8)
	tapDialogButton(t, u, "Yes")
	waitJobs(t, u)
	if exists(filepath.Join(root, "src", "app.zip")) || !trashed.Load() {
		t.Fatal("not moved to trash (default)")
	}
	// Settings off: permanent delete, no trash.
	u.settings()
	w := windowByTitle(t, u.app, "Settings")
	for _, o := range test.LaidOutObjects(w.Content()) {
		if c, ok := o.(*dialogCheck); ok {
			if !c.Checked {
				t.Fatal("trash not default in settings")
			}
			test.Tap(c)
		}
	}
	w.Close()
	trashed.Store(false)
	cursorTo(t, u.left, "song.mp3")
	press(u, fyne.KeyF8)
	tapDialogButton(t, u, "Yes")
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

	u2 := newUI(a) // same prefs store = next launch
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
	for _, p := range []*pane{u.left, u.right} {
		u.activate(p)
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
		found := false
		for _, item := range u.win.MainMenu().Items[0].Items {
			if item.Label != folderLabel() {
				continue
			}
			found = true
			shortcut, ok := item.Shortcut.(*desktop.CustomShortcut)
			if !ok || shortcut.KeyName != fyne.KeyO || shortcut.Modifier != fyne.KeyModifierShortcutDefault|fyne.KeyModifierShift {
				t.Fatal("folder action missing shortcut")
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
				t.Fatal("folder action accepted archive directory")
			}
			tapDialogButton(t, u, "OK")
			p.path = path
		}
		if !found || p.cursor != cursor || !p.marked["readme.md"] {
			t.Fatal("folder action missing or pane state changed")
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
				w.SetContent(container.NewBorder(nil, container.NewVBox(u.left.status, u.fkeyBar()), nil, nil, u.left.body))
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
				assertUIText(t, w.Content(), "readme.md · "+info.Mode().String()+" · 5 B · 2024-01-02 03:04", "F5 Copy")
			case "search_footer":
				assertUIText(t, w.Content(), "Search: read", "F5 Copy")
				if u.left.current().name != "readme.md" {
					t.Fatal("search did not select matching row")
				}
			}
		})
	}
}
