package commander

import (
	"bytes"
	"fmt"
	"fyne.io/fyne/v2/lang"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	native "github.com/dualcommander/dualcommander/pkg/desktop"
)

// Preference keys; settingsFile splits settings, session state, and favourites into JSON files.
const (
	prefLeft, prefRight            = "left.path", "right.path"
	prefLeftBrief, prefRightBrief  = "left.brief", "right.brief"
	prefLeftSort, prefRightSort    = "left.sort", "right.sort"
	prefActive, prefTheme          = "active", "theme"
	prefHidden, prefDensity        = "hidden", "density"
	prefFontSize                   = "fontSize"
	prefTrash                      = "trash"
	prefBrowseArchives             = "browseArchives"
	prefRevealCmd, prefTerminalCmd = "revealCmd", "terminalCmd"
	prefEditorCmd                  = "editorCmd"
	prefCompareFilesCmd            = "compareFilesCmd"
	prefCompareFoldersCmd          = "compareFoldersCmd"
	prefW, prefH                   = "win.w", "win.h"
)

type ui struct {
	inputWindow                          *inputWindow
	app                                  fyne.App
	win                                  fyne.Window
	left, right                          *pane
	active                               *pane
	split                                *container.Split
	jobs                                 *jobs
	themeMode                            string
	themeItems                           []*fyne.MenuItem
	density                              string
	densItems                            []*fyne.MenuItem
	fontSize                             string
	fontItems                            []*fyne.MenuItem
	showHidden                           bool
	briefItem                            *fyne.MenuItem
	detailedItem                         *fyne.MenuItem
	hiddenItem                           *fyne.MenuItem
	starredItem                          *fyne.MenuItem
	compareFilesItem, compareFoldersItem *fyne.MenuItem
	ready                                bool
	archiveTemps                         []string
	filePreview                          func(string) []byte
	passwordGate                         chan struct{}
	passwordWindow                       fyne.Window
}

func newUI(a fyne.App) *ui {
	a.SetIcon(appIcon)
	pr := a.Preferences()
	home, _ := os.UserHomeDir()
	u := &ui{app: a, win: a.NewWindow(appName), density: pr.StringWithFallback(prefDensity, "standard"),
		themeMode: pr.StringWithFallback(prefTheme, "system"), showHidden: pr.Bool(prefHidden)}
	u.passwordGate = make(chan struct{}, 1)
	if u.density != "compact" {
		u.density = "standard" // migrate removed density modes to standard
	}
	u.fontSize = pr.String(prefFontSize)
	switch u.fontSize {
	case "very small":
		u.fontSize = "x-small"
	case "very large":
		u.fontSize = "x-large"
	}
	if !slices.Contains(fontSizes, u.fontSize) {
		u.fontSize = "medium"
	}
	pr.SetString(prefFontSize, u.fontSize)
	applyTheme(a, u.themeMode, u.density)
	u.win.SetIcon(appIcon)
	u.jobs = &jobs{ui: u, box: container.NewVBox()}
	u.left = newPane(u, pr.StringWithFallback(prefLeft, home), pr.Bool(prefLeftBrief), pr.String(prefLeftSort))
	u.right = newPane(u, pr.StringWithFallback(prefRight, home), pr.Bool(prefRightBrief), pr.String(prefRightSort))
	u.active = u.left
	if pr.String(prefActive) == "right" {
		u.active = u.right
	}

	u.win.SetMainMenu(u.mainMenu()) // macOS: global menu bar; Win/Linux: window menu
	if fyne.KeyModifierShortcutDefault != fyne.KeyModifierControl {
		u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl},
			func(fyne.Shortcut) { u.active.refresh() })
	}
	u.split = container.NewHSplit(u.left.box, u.right.box)
	u.win.SetContent(container.NewBorder(nil, container.NewVBox(u.jobs.box, u.fkeyBar()), nil, nil, u.split))
	u.win.Resize(fyne.NewSize(float32(pr.FloatWithFallback(prefW, 1100)), float32(pr.FloatWithFallback(prefH, 700))))
	u.win.SetCloseIntercept(u.quit)
	u.win.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { u.key(u.active, e, u.active.cols()*10) }) // no widget focused (after header/dialog click): keys still drive active pane
	// Alt+F7 opens Find Files.
	u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF7, Modifier: fyne.KeyModifierAlt},
		func(fyne.Shortcut) { u.search() })
	u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyBackspace, Modifier: fyne.KeyModifierShortcutDefault},
		func(fyne.Shortcut) { u.delete() })
	u.win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF4, Modifier: fyne.KeyModifierShift},
		func(fyne.Shortcut) { u.newFile() })
	u.ready = true
	u.activate(u.active)
	return u
}

// mainMenu builds menus; shortcuts use Cmd on macOS, Ctrl elsewhere, and are shown natively.
func (u *ui) mainMenu() *fyne.MainMenu {
	mod := fyne.KeyModifierShortcutDefault
	item := func(label string, k fyne.KeyName, m fyne.KeyModifier, f func()) *fyne.MenuItem {
		action := f
		f = func() {
			if u.passwordWindow == nil {
				action()
			}
		}
		it := fyne.NewMenuItem(label, f)
		if k != "" {
			it.Shortcut = &desktop.CustomShortcut{KeyName: k, Modifier: m}
			if k == fyne.KeyA && m == mod {
				it.Shortcut = &fyne.ShortcutSelectAll{}
			}
			it.Action = func() {
				window := u.win
				if u.passwordWindow != nil {
					window = u.passwordWindow
				}
				if text, ok := window.Canvas().Focused().(interface {
					fyne.Shortcutable
					SelectedText() string
				}); ok {
					text.TypedShortcut(it.Shortcut)
					return
				}
				f()
			}
			u.win.Canvas().AddShortcut(it.Shortcut, func(fyne.Shortcut) { it.Action() })
		}
		return it
	}
	sep := fyne.NewMenuItemSeparator
	// radio: exclusive-choice submenu (one level, per guidelines)
	radio := func(label string, opts []string, cur string, set func(string)) (*fyne.MenuItem, []*fyne.MenuItem) {
		its := make([]*fyne.MenuItem, len(opts))
		for i, m := range opts {
			its[i] = fyne.NewMenuItem(lang.L(strings.ToUpper(m[:1])+m[1:]), func() { set(m) })
			its[i].Checked = m == cur
		}
		parent := fyne.NewMenuItem(label, nil)
		parent.ChildMenu = fyne.NewMenu("", its...)
		return parent, its
	}
	appearance, themeItems := radio(lang.L("Appearance"), themeModes, u.themeMode, u.setTheme)
	density, densItems := radio(lang.L("Density"), densities, u.density, u.setDensity)
	fontSize, fontItems := radio(lang.L("Font Size"), fontSizes, u.fontSize, u.setFontSize)
	for i, item := range fontItems {
		item.Label = lang.L(fontSizes[i])
	}
	hidden := fyne.NewMenuItem(lang.L("Show Hidden Files"), nil)
	hidden.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyPeriod, Modifier: mod | fyne.KeyModifierShift}
	hidden.Checked = u.showHidden
	hidden.Action = u.toggleHidden
	u.win.Canvas().AddShortcut(hidden.Shortcut, func(fyne.Shortcut) { hidden.Action() })
	u.hiddenItem = hidden
	u.briefItem = item(lang.L("Brief"), fyne.Key1, mod, func() { u.active.setBrief(true); u.save() })
	u.detailedItem = item(lang.L("Detailed"), fyne.Key2, mod, func() { u.active.setBrief(false); u.save() })
	u.themeItems = themeItems
	u.densItems = densItems
	u.fontItems = fontItems
	u.starredItem = fyne.NewMenuItem(lang.L("Starred"), nil)
	u.updateStarredMenu()
	u.compareFilesItem = item(lang.L("Compare Files"), fyne.KeyD, mod|fyne.KeyModifierAlt, func() {
		if !u.compareFilesItem.Disabled {
			u.compare(false)
		}
	})
	u.compareFoldersItem = item(lang.L("Compare Folder Contents"), fyne.KeyD, mod|fyne.KeyModifierAlt|fyne.KeyModifierShift, func() {
		if !u.compareFoldersItem.Disabled {
			u.compare(true)
		}
	})
	u.updateCompareMenu()
	aboutLabel := lang.L("About {{.App}}", map[string]any{"App": appName})
	if runtime.GOOS == "darwin" {
		aboutLabel = lang.L("About") // Fyne moves this to the app menu and replaces the default About panel.
	}

	return fyne.NewMainMenu(
		fyne.NewMenu(lang.L("File"),
			item(lang.L("Open"), fyne.KeyO, mod, func() { u.active.open() }),
			item(lang.L("Edit"), fyne.KeyF4, 0, u.edit),
			item(revealLabel(), fyne.KeyR, mod|fyne.KeyModifierShift, func() { u.openExternal(false) }),
			item(lang.L("Open in Terminal"), fyne.KeyF9, 0, func() { u.openExternal(true) }),
			sep(),
			item(lang.L("New Folder"), fyne.KeyF7, 0, u.mkdir),
			item(lang.L("Find Files…"), fyne.KeyF, mod, func() { u.search() }),
			item(lang.L("Copy…"), fyne.KeyF5, 0, func() { u.copyOrMove(false) }),
			item(lang.L("Move…"), fyne.KeyF6, 0, func() { u.copyOrMove(true) }),
			item(lang.L("Rename…"), fyne.KeyF6, fyne.KeyModifierShift, u.rename),
			item(lang.L("Delete…"), fyne.KeyF8, 0, u.delete),
			sep(), u.compareFilesItem, u.compareFoldersItem,
			sep(),
			item(lang.L("Settings…"), fyne.KeyComma, mod, u.settings), // macOS: Fyne moves it to app menu
		), // Fyne adds Quit to File (Win/Linux) or app menu (macOS)
		fyne.NewMenu(lang.L("Edit"),
			item(lang.L("Select All"), fyne.KeyA, mod, func() { u.active.markAll(true) }),
			item(lang.L("Deselect All"), fyne.KeyA, mod|fyne.KeyModifierShift, func() { u.active.markAll(false) }),
		),
		fyne.NewMenu(lang.L("View"),
			u.briefItem, u.detailedItem,
			sep(), hidden,
			item(lang.L("Refresh"), fyne.KeyR, mod, func() { u.active.refresh() }),
			sep(), appearance, density, fontSize,
		),
		fyne.NewMenu(lang.L("Go"),
			item(lang.L("Enclosing Folder"), fyne.KeyUp, mod, func() { u.active.up() }),
			item(lang.L("Home"), fyne.KeyH, mod|fyne.KeyModifierShift, func() { h, _ := os.UserHomeDir(); u.active.cd(h, "") }),
			u.starredItem,
			item(lang.L("Swap Panes"), fyne.KeyU, mod, u.swap),
			item(lang.L("Other Pane Same Folder"), fyne.KeyE, mod, func() { u.other().navigate(u.active.path, "") }),
		),
		fyne.NewMenu(lang.L("Help"),
			item(aboutLabel, "", 0, u.about),
		),
	)
}

func (u *ui) about() {
	w := u.infoWindow(lang.L("About {{.App}}", map[string]any{"App": appName}))
	icon := canvas.NewImageFromResource(appIcon)
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(80, 80))
	version := lang.L("Version {{.Version}}", map[string]any{"Version": applicationVersion()})
	description := widget.NewLabelWithStyle(lang.L("A keyboard-friendly, two-pane file manager.\nBrowse folders side by side.\nCopy, move, and organize your files\nwithout switching windows."), fyne.TextAlignCenter, fyne.TextStyle{})
	description.Wrapping = fyne.TextWrapWord
	findKey := "Ctrl+F / Alt+F7"
	if runtime.GOOS == "darwin" {
		findKey = "Cmd+F / Alt+F7"
	}
	shortcuts := container.NewGridWithColumns(2)
	for _, shortcut := range [][2]string{
		{"Tab", lang.L("Switch active pane")},
		{"Enter", lang.L("Open folder or file")},
		{"Backspace", lang.L("Parent folder / trim search")},
		{"Space", lang.L("Mark item and show size")},
		{"Insert", lang.L("Mark or unmark item")},
		{"F4", lang.L("Edit file")},
		{"Shift+F4", lang.L("Create new file")},
		{"F5", lang.L("Copy")},
		{"F6", lang.L("Move")},
		{"F2 / Shift+F6", lang.L("Rename selected item")},
		{"F7", lang.L("Create folder")},
		{"F8 / Delete", lang.L("Delete")},
		{findKey, lang.L("Find files")},
		{"F10", lang.L("Quit")},
	} {
		shortcuts.Add(widget.NewLabelWithStyle(shortcut[0], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		shortcuts.Add(widget.NewLabel(shortcut[1]))
	}
	w.SetContent(container.NewVScroll(u.dialogContent(container.NewVBox(
		container.NewCenter(icon),
		widget.NewLabelWithStyle(appName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(version, fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewSeparator(),
		container.New(layout.NewCustomPaddedLayout(8, 8, 20, 20), description),
		widget.NewLabelWithStyle(lang.L("Free and open source.\nLicensed under the MIT license."), fyne.TextAlignCenter, fyne.TextStyle{}),
		windowButton(w, "https://dualcommander.github.io/", func() {
			if err := u.app.OpenURL(&url.URL{Scheme: "https", Host: "dualcommander.github.io", Path: "/"}); err != nil {
				u.showErr(err)
			}
		}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(lang.L("Keyboard shortcuts"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.New(layout.NewCustomPaddedLayout(0, 8, 20, 20), shortcuts),
	))))
	w.Resize(fyne.NewSize(520, 740))
	w.Show()
}

// infoWindow uses native window decorations, including the OS close button.
func (u *ui) infoWindow(title string) fyne.Window {
	for _, w := range u.app.Driver().AllWindows() {
		if w.Title() == title {
			w.RequestFocus()
			return w
		}
	}
	return u.dialogWindow(title)
}

// edit opens the file under the cursor in the configured editor; dirs and ".." ignored.
func (u *ui) edit() {
	if u.active.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	if e := u.active.current(); e.name != "" && e.name != ".." && !e.dir {
		cmd, err := u.editorCommand(filepath.Join(u.active.path, e.name))
		u.startExternal(cmd, err)
	}
}

func (u *ui) editorCommand(path string) (*exec.Cmd, error) {
	command := u.app.Preferences().String(prefEditorCmd)
	if strings.TrimSpace(command) == "" {
		command = editorDefault(runtime.GOOS)
	}
	return native.Command(command, path, u.active.path)
}

func revealLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return lang.L("Reveal in Finder")
	case "windows":
		return lang.L("Show in Explorer")
	}
	return lang.L("Open in File Manager")
}

func (u *ui) externalCommand(terminal bool) (*exec.Cmd, error) {
	if u.active.inArchive() {
		return nil, fmt.Errorf(lang.L("leave archive folder before using terminal or reveal"))
	}
	reveal, term := native.Defaults(runtime.GOOS)
	key, fallback := prefRevealCmd, reveal
	path := u.active.path
	if terminal {
		key, fallback = prefTerminalCmd, term
	} else if s := u.active.selection(); len(s) > 0 {
		path = s[0]
	}
	command := u.app.Preferences().String(key)
	if strings.TrimSpace(command) == "" {
		command = fallback
	}
	return native.Command(command, path, u.active.path)
}

func (u *ui) openExternal(terminal bool) {
	cmd, err := u.externalCommand(terminal)
	if err == nil && terminal {
		native.PrepareTerminal(cmd)
	}
	u.startExternal(cmd, err)
}

func (u *ui) startExternal(cmd *exec.Cmd, err error, acceptedExitCodes ...int) {
	if err == nil {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Start()
		if err == nil {
			go func() {
				if err := cmd.Wait(); err != nil {
					if exit, ok := err.(*exec.ExitError); ok && slices.Contains(acceptedExitCodes, exit.ExitCode()) {
						return
					}
					if detail := strings.TrimSpace(stderr.String()); detail != "" {
						err = fmt.Errorf("%w: %s", err, detail)
					}
					post(func() { u.showErr(err) })
				}
			}() // reap child without blocking UI; surface native handler failures
		}
	}
	u.showErr(err)
}

func (u *ui) setTheme(m string) {
	u.themeMode = m
	applyTheme(u.app, m, u.density)
	for i, it := range u.themeItems {
		it.Checked = themeModes[i] == m
	}
	u.win.MainMenu().Refresh()
	u.save()
}

func (u *ui) setDensity(d string) {
	u.density = d
	for i, it := range u.densItems {
		it.Checked = densities[i] == d
	}
	applyTheme(u.app, u.themeMode, d)
	u.left.setDensity()
	u.right.setDensity()
	u.activate(u.active) // new list widgets need focus + selection; saves
	u.win.MainMenu().Refresh()
}

func (u *ui) setFontSize(size string) {
	if !slices.Contains(fontSizes, size) {
		return
	}
	u.fontSize = size
	u.app.Preferences().SetString(prefFontSize, size)
	for i, it := range u.fontItems {
		it.Checked = fontSizes[i] == size
	}
	applyTheme(u.app, u.themeMode, u.density)
	// Rebuild cached row/column sizes, retaining cursor, marks and view mode.
	u.left.setDensity()
	u.right.setDensity()
	u.activate(u.active)
	u.win.MainMenu().Refresh()
}

func (u *ui) swap() {
	u.left, u.right = u.right, u.left
	u.split.Leading, u.split.Trailing = u.left.box, u.right.box
	u.split.Refresh()
	u.activate(u.active)
}

func (u *ui) fkeyBar() fyne.CanvasObject {
	b := func(l string, f func()) fyne.CanvasObject { return widget.NewButton(l, f) }
	return container.NewGridWithColumns(7,
		b(lang.L("F4 Edit"), u.edit),
		b(lang.L("F5 Copy"), func() { u.copyOrMove(false) }),
		b(lang.L("F6 Move"), func() { u.copyOrMove(true) }),
		b(lang.L("F7 MkDir"), u.mkdir),
		b(lang.L("F8 Delete"), u.delete),
		b(lang.L("F9 Terminal"), func() { u.openExternal(true) }),
		b(lang.L("F10 Quit"), u.quit))
}

// key handles commander keys for pane p; page = entries per PgUp/PgDn.
func (u *ui) key(p *pane, e *fyne.KeyEvent, page int) {
	if u.passwordWindow != nil {
		u.passwordWindow.RequestFocus()
		return
	}
	if p.brief {
		page = p.grid.rows * p.grid.cols
	}
	switch e.Name {
	case desktop.KeyMenu:
		p.keyboardContextMenu()
	case fyne.KeyUp:
		p.moveTo(p.cursor - 1)
	case fyne.KeyDown:
		p.moveTo(p.cursor + 1)
	case fyne.KeyLeft:
		if p.brief {
			p.moveTo(p.cursor - p.grid.rows)
		}
	case fyne.KeyRight:
		if p.brief {
			p.moveTo(p.cursor + p.grid.rows)
		}
	case fyne.KeyTab:
		u.activate(u.other())
	case fyne.KeyReturn, fyne.KeyEnter:
		p.open()
	case fyne.KeyF2:
		if p.current().name == ".." {
			u.win.Canvas().Focus(p.pathBar)
			p.pathBar.TypedShortcut(&fyne.ShortcutSelectAll{})
		} else {
			u.rename()
		}
	case fyne.KeyF4:
		if driver, ok := u.app.Driver().(interface{ CurrentKeyModifiers() fyne.KeyModifier }); ok && driver.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
			u.newFile()
			return
		}
		u.edit()
	case fyne.KeyBackspace:
		if p.searchPrefix != "" && len(p.marked) == 0 {
			runes := []rune(p.searchPrefix)
			p.searchPrefix = string(runes[:len(runes)-1])
			p.matchSearch()
		} else {
			p.up()
		}
	case fyne.KeyEscape:
		p.searchPrefix = ""
		p.updateStatus()
	case fyne.KeySpace:
		p.searchPrefix = ""
		if e := p.current(); e.name != "" && e.name != ".." && (!e.dir || !p.inArchive()) {
			p.flip(p.cursor)
			if p.marked[e.name] {
				if e.dir {
					p.requestDirectorySize()
				} else {
					p.entries[p.cursor].sizeRequested = true
				}
			}
		}
		p.updateStatus()
	case fyne.KeyInsert:
		p.toggleMark()
	case fyne.KeyHome:
		p.moveTo(0)
	case fyne.KeyEnd:
		p.moveTo(len(p.entries) - 1)
	case fyne.KeyPageUp:
		p.moveTo(p.cursor - page)
	case fyne.KeyPageDown:
		p.moveTo(p.cursor + page)
	case fyne.KeyF5:
		u.copyOrMove(false)
	case fyne.KeyF6:
		if driver, ok := u.app.Driver().(interface{ CurrentKeyModifiers() fyne.KeyModifier }); ok && driver.CurrentKeyModifiers()&fyne.KeyModifierShift != 0 {
			u.rename()
			return
		}
		u.copyOrMove(true)
	case fyne.KeyF7:
		u.mkdir()
	case fyne.KeyF8, fyne.KeyDelete:
		u.delete()
	case fyne.KeyF9:
		u.openExternal(true)
	case fyne.KeyF10:
		u.quit()
	}
}

func (u *ui) other() *pane {
	if u.active == u.left {
		return u.right
	}
	return u.left
}

func (u *ui) activate(p *pane) {
	u.active = p
	u.updateViewMenu()
	for _, q := range []*pane{u.left, u.right} {
		if len(q.entries) > 0 {
			q.showCursor()
		}
	}
	u.win.Canvas().Focus(p.focusable())
	u.save()
}

func (u *ui) toggleHidden() {
	if u.passwordWindow != nil {
		return
	}
	u.showHidden = !u.showHidden
	u.left.refresh()
	u.right.refresh()
	u.updateViewMenu()
	u.save()
}

func (u *ui) updateViewMenu() {
	if u.active == nil || u.briefItem == nil || u.win.MainMenu() == nil {
		return
	}
	u.briefItem.Checked = u.active.brief
	u.detailedItem.Checked = !u.active.brief
	u.hiddenItem.Checked = u.showHidden
	u.updateCompareMenu()
	u.win.MainMenu().Refresh()
}

func (u *ui) save() {
	if !u.ready {
		return
	}
	pr := u.app.Preferences()
	pr.SetString(prefLeft, u.left.savedPath())
	pr.SetString(prefRight, u.right.savedPath())
	pr.SetBool(prefLeftBrief, u.left.brief)
	pr.SetBool(prefRightBrief, u.right.brief)
	pr.SetString(prefLeftSort, u.left.sortPref())
	pr.SetString(prefRightSort, u.right.sortPref())
	pr.SetString(prefTheme, u.themeMode)
	pr.SetBool(prefHidden, u.showHidden)
	pr.SetString(prefDensity, u.density)
	side := "left"
	if u.active == u.right {
		side = "right"
	}
	pr.SetString(prefActive, side)
}

// quit persists state incl. window size (only reliable at exit) and closes.
func (u *ui) quit() {
	u.closePanes()
	u.save()
	s := u.win.Canvas().Size()
	u.app.Preferences().SetFloat(prefW, float64(s.Width))
	u.app.Preferences().SetFloat(prefH, float64(s.Height))
	for _, dir := range u.archiveTemps {
		os.RemoveAll(dir)
	}
	u.app.Quit()
}

func (u *ui) closePanes() {
	for _, p := range []*pane{u.left, u.right} {
		if p.watcher != nil {
			w := p.watcher
			p.watcher = nil // invalidate queued refreshes before closing the watcher
			w.Close()
		}
		if p.sizeCancel != nil {
			p.sizeCancel()
		}
		p.iconGeneration++
		p.spaceGeneration++
	}
}

func (p *pane) savedPath() string {
	if p.inArchive() {
		return filepath.Dir(p.archiveRoot)
	}
	return p.path
}

func (u *ui) showErr(err error) {
	if err == nil {
		return
	}
	slog.Error("Operation failed", "error", err)
	if u.win != nil {
		u.errorWindow(err)
	}
}
