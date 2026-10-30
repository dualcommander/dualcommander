package commander

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2/lang"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/desktop"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// post runs f on the UI thread; tests swap it for a queue drained on the test goroutine.
var post = fyne.Do

// latestProgress bounds pending UI work without dropping completion or errors.
func latestProgress(dispatch func(func()), update fsops.Progress) fsops.Progress {
	report := latestUpdate(dispatch, func(p fsops.CopyProgress) { update(p.Done, p.Total, p.Current) })
	return func(done, total int64, file string) {
		report(fsops.CopyProgress{Done: done, Total: total, Current: file})
	}
}

func latestUpdate[T any](dispatch func(func()), update func(T)) func(T) {
	var mu sync.Mutex
	var pending bool
	var latest T
	return func(value T) {
		mu.Lock()
		latest = value
		if pending {
			mu.Unlock()
			return
		}
		pending = true
		mu.Unlock()
		dispatch(func() {
			mu.Lock()
			value := latest
			pending = false
			mu.Unlock()
			update(value)
		})
	}
}

// ponytail: Cut stays app-local; macOS Copy also exports native file URLs.
type fileClipboard struct {
	paths     []string
	cut, busy bool
}

func (u *ui) captureFiles(cut bool) {
	if u.passwordWindow != nil {
		return
	}
	if u.active.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	u.fileClipboard = &fileClipboard{paths: u.active.selection(), cut: cut}
	if !cut {
		u.showErr(desktop.CopyFiles(u.fileClipboard.paths))
	}
}

func (u *ui) pasteFiles() {
	clip := u.fileClipboard
	if u.passwordWindow != nil || clip == nil || len(clip.paths) == 0 || clip.busy {
		return
	}
	if u.active.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	to := u.active.path
	verb := lang.L("Copy")
	if clip.cut {
		verb = lang.L("Move/Rename")
		clip.busy = true
	}
	title := fmt.Sprintf(lang.L("%s %d item(s) → %s"), verb, len(clip.paths), to)
	u.jobs.runCounted(title, []*pane{u.left, u.right}, func(ctx context.Context, report func(fsops.CopyProgress)) error {
		if !clip.cut {
			return fsops.CopyWithConflicts(ctx, clip.paths, to, report, u.overwriteConflict)
		}
		err := fsops.MoveWithConflicts(ctx, clip.paths, to, func(done, total int64, file string) {
			report(fsops.CopyProgress{Done: done, Total: total, Current: file, FilesTotal: -1})
		}, u.overwriteConflict)
		// Moves can finish only part of a selection; retain skipped/failed sources for retry.
		var remaining []string
		for _, path := range clip.paths {
			if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
				remaining = append(remaining, path)
			}
		}
		post(func() {
			clip.busy = false
			clip.paths = remaining
			if len(remaining) == 0 && u.fileClipboard == clip {
				u.fileClipboard = nil
			}
		})
		return err
	})
}

func (u *ui) copyOrMove(move bool) {
	src, dst := u.active, u.other()
	u.transferSelection(src, dst, src.selection(), move)
}

func (u *ui) transferSelection(src, dst *pane, sel []string, move bool) {
	if src.inArchive() || dst.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only; copy/move involving archive folders is not supported")))
		return
	}
	if len(sel) == 0 {
		return
	}
	verb := lang.L("Copy")
	if move {
		verb = lang.L("Move/Rename")
	}
	target := newEscEntry()
	target.SetText(dst.path)
	if len(sel) == 1 && move && dst.path == src.path {
		target.SetText(sel[0])
	}
	inputDialog(u, fmt.Sprintf(lang.L("%s %d item(s) to"), verb, len(sel)), verb, lang.L("Target"), target, func() {
		if strings.TrimSpace(target.Text) == "" {
			return
		}
		to := target.Text
		if !filepath.IsAbs(to) {
			to = filepath.Join(src.path, to)
		}
		title := fmt.Sprintf(lang.L("%s %d item(s) → %s"), verb, len(sel), to)
		if !move {
			u.jobs.runCounted(title, []*pane{src, dst}, func(ctx context.Context, report func(fsops.CopyProgress)) error {
				return fsops.CopyWithConflicts(ctx, sel, to, report, u.overwriteConflict)
			})
			src.markAll(false)
			return
		}
		u.jobs.run(title, []*pane{src, dst},
			func(ctx context.Context, prog fsops.Progress) error {
				return fsops.MoveWithConflicts(ctx, sel, to, prog, u.overwriteConflict)
			})
		src.markAll(false)
	})
}

// inputDialog: form with one focused entry; Enter submits, focus returns to pane after.
func inputDialog(u *ui, title, confirm, label string, e *escEntry, ok func()) {
	if u.passwordWindow != nil {
		return
	}
	w := u.inputDialogWindow(title)
	w.SetOnClosed(u.blockParent(w))
	submit := func() {
		if e.Validate() == nil {
			w.Close()
			ok()
		}
	}
	e.esc = escapeWindow(w)
	e.OnSubmitted = func(string) { submit() }
	w.SetContent(u.dialogContent(container.NewVBox(
		widget.NewForm(widget.NewFormItem(label, e)),
		dialogButtons(w, 1, windowButton(w, lang.L("Cancel"), w.Close), windowButton(w, confirm, submit)),
	)))
	w.Resize(fyne.NewSize(500, w.Content().MinSize().Height))
	w.Canvas().Focus(e)
	w.Show()
}

func (u *ui) rename() {
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	item := p.current()
	if item.name == "" || item.name == ".." {
		return
	}
	dir := p.path
	src := filepath.Join(dir, item.name)
	name := newEscEntry()
	name.SetText(item.name)
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\\x00") {
			return fmt.Errorf(lang.L("Enter a file name without path separators"))
		}
		if value != item.name {
			if _, err := os.Lstat(filepath.Join(dir, value)); !os.IsNotExist(err) {
				return fmt.Errorf(lang.L("Destination exists or cannot be checked"))
			}
		}
		return nil
	}
	inputDialog(u, lang.L("Rename"), lang.L("Rename"), lang.L("New name"), name, func() {
		if name.Text == item.name {
			return
		}
		// Recheck at submission; never intentionally replace another item.
		if err := name.Validate(); err != nil {
			u.showErr(err)
			return
		}
		if err := os.Rename(src, filepath.Join(dir, name.Text)); err != nil {
			u.showErr(err)
			return
		}
		delete(p.marked, item.name)
		p.cd(dir, name.Text)
		u.other().refresh()
	})
}

// trash is swappable so tests don't fill the real Trash.
var trash = fsops.Trash

func (u *ui) delete() {
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	sel := p.selection()
	if len(sel) == 0 {
		return
	}
	toTrash := u.app.Preferences().BoolWithFallback(prefTrash, true)
	verb, warn, op := lang.L("Delete"), lang.L("\n\nThis cannot be undone."), fsops.Delete
	if toTrash {
		verb, warn, op = lang.L("Move to {{.Trash}}", map[string]any{"Trash": trashName()}), "", trash
	}
	msg := fmt.Sprintf("%s %q?", verb, filepath.Base(sel[0]))
	if len(sel) > 1 {
		msg = fmt.Sprintf(lang.L("%s %d items?\n%s, …"), verb, len(sel), filepath.Base(sel[0]))
	}
	u.confirm(verb, msg+warn, func() {
		u.jobs.run(fmt.Sprintf(lang.L("%s %d item(s)"), verb, len(sel)), []*pane{p, u.other()},
			func(ctx context.Context, prog fsops.Progress) error { return op(ctx, sel, prog) })
		p.markAll(false)
	})
}

func trashName() string {
	switch runtime.GOOS {
	case "darwin":
		return lang.L("Bin")
	case "windows":
		return lang.L("Recycle Bin")
	}
	return lang.L("Trash")
}

// Settings are saved immediately, like macOS settings windows.
func (u *ui) settings() {
	w := u.infoWindow(lang.L("Settings"))
	pr := u.app.Preferences()
	hint := func(text string) fyne.CanvasObject {
		label := widget.NewLabel(text)
		label.Wrapping = fyne.TextWrapWord
		return container.NewBorder(nil, nil, newDialogInfoIcon(), nil, label)
	}
	heading := func(text string) *widget.Label {
		return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	var content *container.ThemeOverride
	choice := func(options []string, selected string, changed func(string)) *dialogSelect {
		labels := make([]string, len(options))
		for i, option := range options {
			labels[i] = lang.L(option)
		}
		s := &dialogSelect{close: w.Close}
		s.Options, s.Selected = labels, lang.L(selected)
		s.ExtendBaseWidget(s)
		s.OnChanged = func(value string) {
			if i := slices.Index(labels, value); i >= 0 {
				changed(options[i])
			}
			content.Theme = dialogTheme{u.app.Settings().Theme()}
			content.Refresh()
			w.Canvas().Focus(s)
		}
		return s
	}
	appearance := widget.NewForm(
		widget.NewFormItem(lang.L("Theme"), choice(themeModes, u.themeMode, u.setTheme)),
		widget.NewFormItem(lang.L("Density"), choice(densities, u.density, u.setDensity)),
		widget.NewFormItem(lang.L("Font size"), choice(fontSizes, u.fontSize, u.setFontSize)),
	)
	languageChoice := &dialogSelect{close: w.Close}
	languageChoice.Options = slices.Clone(languageNames)
	languageChoice.Options[0] = lang.L("System")
	index := slices.Index(languageCodes, pr.String("language"))
	if index < 0 {
		index = 0
	}
	languageChoice.Selected = languageChoice.Options[index]
	languageChoice.ExtendBaseWidget(languageChoice)
	languageChoice.OnChanged = func(value string) {
		if i := slices.Index(languageChoice.Options, value); i >= 0 {
			pr.SetString("language", languageCodes[i])
		}
	}
	languageItem := widget.NewFormItem(lang.L("Language"), languageChoice)
	languageItem.HintText = lang.L("Restart DualCommander to apply language changes.")
	appearance.AppendItem(languageItem)
	viewChoice := func(p *pane) *dialogSelect {
		selected := lang.L("Detailed")
		if p.brief {
			selected = lang.L("Brief")
		}
		return choice([]string{lang.L("Brief"), lang.L("Detailed")}, selected, func(value string) {
			p.setBrief(value == lang.L("Brief"))
			u.save()
		})
	}
	views := widget.NewForm(
		widget.NewFormItem(lang.L("Left pane view"), viewChoice(u.left)),
		widget.NewFormItem(lang.L("Right pane view"), viewChoice(u.right)),
	)
	hidden := &dialogCheck{close: w.Close}
	hidden.Text = lang.L("Show hidden files")
	hidden.Checked = u.showHidden
	hidden.OnChanged = func(b bool) {
		if b != u.showHidden {
			u.setHidden(b)
		}
	}
	hidden.ExtendBaseWidget(hidden)
	freeSpace := &dialogCheck{close: w.Close}
	freeSpace.Text = lang.L("Show free disk space")
	freeSpace.Checked = pr.BoolWithFallback(prefShowFreeSpace, true)
	freeSpace.OnChanged = func(b bool) {
		pr.SetBool(prefShowFreeSpace, b)
		u.left.updateSpace()
		u.right.updateSpace()
	}
	freeSpace.ExtendBaseWidget(freeSpace)
	checkHint := func(c *dialogCheck, text string) {
		label := widget.NewLabel(text)
		label.Wrapping = fyne.TextWrapWord
		icon := container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), newDialogInfoIcon())
		guidance := container.NewBorder(nil, nil, icon, nil, label)
		card := widget.NewCard("", "", container.NewGridWrap(fyne.NewSize(400, guidance.MinSize().Height*2), guidance))
		c.hint = container.New(searchHintLayout{c}, card)
		c.hint.Hide()
	}
	checkHint(freeSpace, lang.L("When unchecked, used disk space is shown instead."))
	c := &dialogCheck{close: w.Close}
	switch runtime.GOOS {
	case "darwin":
		c.Text = lang.L("Move deleted files to Bin")
	case "windows":
		c.Text = lang.L("Move deleted files to Recycle Bin")
	default:
		c.Text = lang.L("Move deleted files to Trash")
	}
	c.OnChanged = func(b bool) { pr.SetBool(prefTrash, b) }
	c.ExtendBaseWidget(c)
	c.Checked = pr.BoolWithFallback(prefTrash, true)
	checkHint(c, lang.L("When unchecked, files bypass the trash and are permanently deleted."))
	browse := &dialogCheck{close: w.Close}
	browse.Text = lang.L("Browse archives in DualCommander")
	browse.OnChanged = func(b bool) { pr.SetBool(prefBrowseArchives, b) }
	browse.ExtendBaseWidget(browse)
	browse.Checked = pr.BoolWithFallback(prefBrowseArchives, true)
	checkHint(browse, lang.L("Browse supported archives like folders (read-only). When unchecked, archives open in the default application."))
	reveal, terminal := desktop.Defaults(runtime.GOOS)
	commandEntry := func(key, fallback string) *escEntry {
		e := newEscEntry()
		e.AlwaysShowValidationError = true // Keep invalid focused input from showing a stale OK marker.
		e.esc = escapeWindow(w)
		e.SetPlaceHolder(fallback)
		e.SetText(pr.String(key))
		e.Validator = func(value string) error {
			if strings.TrimSpace(value) == "" {
				return nil
			}
			_, err := desktop.Command(value, u.active.path, u.active.path)
			if key == prefCompareFilesCmd || key == prefCompareFoldersCmd {
				_, err = desktop.ComparisonCommand(value, u.left.path, u.right.path, u.left.path)
				return err
			}
			if err != nil {
				return fmt.Errorf(lang.L("Use a JSON list with an executable and its arguments"))
			}
			return nil
		}
		e.OnChanged = func(value string) {
			if e.Validate() == nil {
				pr.SetString(key, value)
				u.updateCompareMenu()
				u.win.MainMenu().Refresh()
			}
		}
		return e
	}
	revealItem := widget.NewFormItem(lang.L("Show in file manager"), commandEntry(prefRevealCmd, reveal))
	revealItem.HintText = lang.L("{path} is the selected item; {dir} is the active pane folder.")
	terminalItem := widget.NewFormItem(lang.L("Open folder in terminal"), commandEntry(prefTerminalCmd, terminal))
	terminalItem.HintText = lang.L("{dir} is the active pane folder.")
	editorItem := widget.NewFormItem(lang.L("Edit file (F4)"), commandEntry(prefEditorCmd, editorDefault(runtime.GOOS)))
	editorItem.HintText = lang.L(`Use {path} for the file, e.g. ["code","--wait","{path}"].`)
	filesItem := widget.NewFormItem(lang.L("Compare files"), commandEntry(prefCompareFilesCmd, `["meld","{left}","{right}"]`))
	filesItem.HintText = lang.L("Cursor file in each pane. Use {left} and {right}; blank disables comparison.")
	foldersItem := widget.NewFormItem(lang.L("Compare folder contents"), commandEntry(prefCompareFoldersCmd, `["meld","{left}","{right}"]`))
	foldersItem.HintText = lang.L("Cursor folder in each pane. Configure tool flags for recursive content comparison; blank disables.")
	content = u.dialogContent(container.NewVBox(
		hint(lang.L("Changes are saved automatically. Invalid commands are not saved.")),
		widget.NewSeparator(),
		heading(lang.L("Appearance")),
		appearance,
		widget.NewSeparator(),
		heading(lang.L("File views")),
		views,
		hidden,
		freeSpace,
		widget.NewSeparator(),
		heading(lang.L("File deletion")),
		c,
		widget.NewSeparator(),
		heading(lang.L("Archives")),
		browse,
		widget.NewSeparator(),
		heading(lang.L("External applications")),
		hint(lang.L("Custom commands use JSON argument lists, without a shell.\nLeave a field blank for the OS default; comparison has no default and stays disabled.")),
		widget.NewForm(revealItem, terminalItem, editorItem, filesItem, foldersItem),
	)).(*container.ThemeOverride)
	w.SetContent(container.NewStack(container.NewVScroll(content), freeSpace.hint, c.hint, browse.hint))
	w.Resize(fyne.NewSize(700, min(700, content.MinSize().Height)))
	w.Show()
}

func (u *ui) newFile() {
	if u.passwordWindow != nil {
		return
	}
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	dir := p.path
	name := newEscEntry()
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\:\x00") {
			return fmt.Errorf(lang.L("Enter a file name without path separators"))
		}
		if _, err := os.Lstat(filepath.Join(dir, value)); !os.IsNotExist(err) {
			return fmt.Errorf(lang.L("Destination exists or cannot be checked"))
		}
		return nil
	}
	inputDialog(u, lang.L("New file"), lang.L("Create"), lang.L("File name"), name, func() {
		file, err := os.OpenFile(filepath.Join(dir, name.Text), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			err = file.Close()
		}
		if err != nil {
			u.showErr(err)
			return
		}
		u.showErr(p.cd(dir, name.Text))
		u.activate(p)
	})
}

func (u *ui) mkdir() {
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf(lang.L("archive browsing is read-only")))
		return
	}
	name := newEscEntry()
	inputDialog(u, lang.L("New directory"), lang.L("Create"), lang.L("Name"), name, func() {
		if name.Text != "" {
			dir := filepath.Join(p.path, name.Text)
			u.showErr(os.MkdirAll(dir, 0o755))
			p.cd(p.path, filepath.Base(dir))
		}
	})
}
