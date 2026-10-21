package commander

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

func (u *ui) copyOrMove(move bool) {
	src, dst := u.active, u.other()
	if src.inArchive() || dst.inArchive() {
		u.showErr(fmt.Errorf("archive browsing is read-only; copy/move involving archive folders is not supported"))
		return
	}
	sel := src.selection()
	if len(sel) == 0 {
		return
	}
	verb := "Copy"
	if move {
		verb = "Move/Rename"
	}
	target := newEscEntry()
	target.SetText(dst.path)
	if len(sel) == 1 && move && dst.path == src.path {
		target.SetText(sel[0])
	}
	inputDialog(u, fmt.Sprintf("%s %d item(s) to", verb, len(sel)), verb, "Target", target, func() {
		if strings.TrimSpace(target.Text) == "" {
			return
		}
		to := target.Text
		if !filepath.IsAbs(to) {
			to = filepath.Join(src.path, to)
		}
		title := fmt.Sprintf("%s %d item(s) → %s", verb, len(sel), to)
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
		dialogButtons(w, 1, windowButton(w, "Cancel", w.Close), windowButton(w, confirm, submit)),
	)))
	w.Resize(fyne.NewSize(500, w.Content().MinSize().Height))
	w.Canvas().Focus(e)
	w.Show()
}

func (u *ui) rename() {
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf("archive browsing is read-only"))
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
			return fmt.Errorf("Enter a file name without path separators")
		}
		if value != item.name {
			if _, err := os.Lstat(filepath.Join(dir, value)); !os.IsNotExist(err) {
				return fmt.Errorf("Destination exists or cannot be checked")
			}
		}
		return nil
	}
	inputDialog(u, "Rename", "Rename", "New name", name, func() {
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
		u.showErr(fmt.Errorf("archive browsing is read-only"))
		return
	}
	sel := p.selection()
	if len(sel) == 0 {
		return
	}
	toTrash := u.app.Preferences().BoolWithFallback(prefTrash, true)
	verb, warn, op := "Delete", "\n\nThis cannot be undone.", fsops.Delete
	if toTrash {
		verb, warn, op = "Move to "+trashName(), "", trash
	}
	msg := fmt.Sprintf("%s %q?", verb, filepath.Base(sel[0]))
	if len(sel) > 1 {
		msg = fmt.Sprintf("%s %d items?\n%s, …", verb, len(sel), filepath.Base(sel[0]))
	}
	u.confirm(verb, msg+warn, func() {
		u.jobs.run(fmt.Sprintf("%s %d item(s)", verb, len(sel)), []*pane{p, u.other()},
			func(ctx context.Context, prog fsops.Progress) error { return op(ctx, sel, prog) })
		p.markAll(false)
	})
}

func trashName() string {
	if runtime.GOOS == "windows" {
		return "Recycle Bin"
	}
	return "Trash"
}

// Settings are saved immediately, like macOS settings windows.
func (u *ui) settings() {
	w := u.infoWindow("Settings")
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
		s := &dialogSelect{close: w.Close}
		s.Options, s.Selected = options, selected
		s.ExtendBaseWidget(s)
		s.OnChanged = func(value string) {
			changed(value)
			content.Theme = dialogTheme{u.app.Settings().Theme()}
			content.Refresh()
			w.Canvas().Focus(s)
		}
		return s
	}
	appearance := widget.NewForm(
		widget.NewFormItem("Theme", choice(themeModes, u.themeMode, u.setTheme)),
		widget.NewFormItem("Density", choice(densities, u.density, u.setDensity)),
		widget.NewFormItem("Font size", choice(fontSizes, u.fontSize, u.setFontSize)),
	)
	viewChoice := func(p *pane) *dialogSelect {
		selected := "Detailed"
		if p.brief {
			selected = "Brief"
		}
		return choice([]string{"Brief", "Detailed"}, selected, func(value string) {
			p.setBrief(value == "Brief")
			u.save()
		})
	}
	views := widget.NewForm(
		widget.NewFormItem("Left pane view", viewChoice(u.left)),
		widget.NewFormItem("Right pane view", viewChoice(u.right)),
	)
	hidden := &dialogCheck{close: w.Close}
	hidden.Text = "Show hidden files"
	hidden.Checked = u.showHidden
	hidden.OnChanged = func(b bool) {
		if b != u.showHidden {
			u.toggleHidden()
		}
	}
	hidden.ExtendBaseWidget(hidden)
	c := &dialogCheck{close: w.Close}
	c.Text = "Move deleted files to " + trashName()
	c.OnChanged = func(b bool) { pr.SetBool(prefTrash, b) }
	c.ExtendBaseWidget(c)
	c.Checked = pr.BoolWithFallback(prefTrash, true)
	browse := &dialogCheck{close: w.Close}
	browse.Text = "Browse archives in DualCommander"
	browse.OnChanged = func(b bool) { pr.SetBool(prefBrowseArchives, b) }
	browse.ExtendBaseWidget(browse)
	browse.Checked = pr.BoolWithFallback(prefBrowseArchives, true)
	reveal, terminal := desktop.Defaults(runtime.GOOS)
	commandEntry := func(key, fallback string) *escEntry {
		e := newEscEntry()
		e.esc = escapeWindow(w)
		e.SetPlaceHolder(fallback)
		e.SetText(pr.String(key))
		e.Validator = func(value string) error {
			if strings.TrimSpace(value) == "" {
				return nil
			}
			_, err := desktop.Command(value, u.active.path, u.active.path)
			if err != nil {
				return fmt.Errorf("Use a JSON list with an executable and its arguments")
			}
			return nil
		}
		e.OnChanged = func(value string) {
			if e.Validate() == nil {
				pr.SetString(key, value)
			}
		}
		return e
	}
	revealItem := widget.NewFormItem("Show in file manager", commandEntry(prefRevealCmd, reveal))
	revealItem.HintText = "{path} is the selected item; {dir} is the active pane folder."
	terminalItem := widget.NewFormItem("Open folder in terminal", commandEntry(prefTerminalCmd, terminal))
	terminalItem.HintText = "{dir} is the active pane folder."
	content = u.dialogContent(container.NewVBox(
		hint("Changes are saved automatically. Invalid commands are not saved."),
		widget.NewSeparator(),
		heading("Appearance"),
		appearance,
		widget.NewSeparator(),
		heading("File views"),
		views,
		hidden,
		widget.NewSeparator(),
		heading("File deletion"),
		c,
		widget.NewSeparator(),
		heading("Archives"),
		browse,
		widget.NewSeparator(),
		heading("External applications"),
		hint("Custom commands use JSON argument lists, without a shell.\nLeave a field blank to use the OS default shown below."),
		widget.NewForm(revealItem, terminalItem),
	)).(*container.ThemeOverride)
	w.SetContent(container.NewVScroll(content))
	w.Resize(fyne.NewSize(700, min(700, content.MinSize().Height)))
	w.Show()
}

func (u *ui) newFile() {
	if u.passwordWindow != nil {
		return
	}
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf("archive browsing is read-only"))
		return
	}
	dir := p.path
	name := newEscEntry()
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\:\x00") {
			return fmt.Errorf("Enter a file name without path separators")
		}
		if _, err := os.Lstat(filepath.Join(dir, value)); !os.IsNotExist(err) {
			return fmt.Errorf("Destination exists or cannot be checked")
		}
		return nil
	}
	inputDialog(u, "New file", "Create", "File name", name, func() {
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
		u.showErr(fmt.Errorf("archive browsing is read-only"))
		return
	}
	name := newEscEntry()
	inputDialog(u, "New directory", "Create", "Name", name, func() {
		if name.Text != "" {
			dir := filepath.Join(p.path, name.Text)
			u.showErr(os.MkdirAll(dir, 0o755))
			p.cd(p.path, filepath.Base(dir))
		}
	})
}
