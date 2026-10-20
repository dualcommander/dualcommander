package commander

import (
	"context"
	"errors"
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

// jobs runs file operations in background goroutines and lists them with progress + cancel.
type jobs struct {
	ui  *ui
	box *fyne.Container
}

// latestProgress bounds pending UI work without dropping completion or errors.
func latestProgress(dispatch func(func()), update fsops.Progress) fsops.Progress {
	var mu sync.Mutex
	var pending bool
	var done, total int64
	var file string
	return func(d, t int64, f string) {
		mu.Lock()
		done, total, file = d, t, f
		if pending {
			mu.Unlock()
			return
		}
		pending = true
		mu.Unlock()
		dispatch(func() {
			mu.Lock()
			d, t, f := done, total, file
			pending = false
			mu.Unlock()
			update(d, t, f)
		})
	}
}

func (j *jobs) run(title string, refresh []*pane, fn func(ctx context.Context, prog fsops.Progress) error) {
	ctx, cancel := context.WithCancel(context.Background())
	bar := widget.NewProgressBar()
	label := widget.NewLabel(title)
	label.Truncation = fyne.TextTruncateEllipsis
	var item *fyne.Container
	stop := widget.NewButton("Cancel", cancel)
	item = container.NewBorder(nil, nil, nil, stop, container.NewVBox(label, bar))
	j.box.Add(item)
	go func() {
		err := fn(ctx, latestProgress(post, func(done, total int64, cur string) {
			if total > 0 {
				bar.SetValue(float64(done) / float64(total))
			}
			label.SetText(title + " — " + filepath.Base(cur))
		}))
		cancelled := ctx.Err() != nil
		cancel()
		post(func() {
			j.box.Remove(item)
			for _, p := range refresh {
				p.refresh()
			}
			if err != nil && !cancelled && !errors.Is(err, context.Canceled) {
				j.ui.showErr(fmt.Errorf("%s: %w", title, err))
			}
		})
	}()
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
		u.jobs.run(fmt.Sprintf("%s %d item(s) → %s", verb, len(sel), to), []*pane{src, dst},
			func(ctx context.Context, prog fsops.Progress) error {
				if move {
					return fsops.Move(ctx, sel, to, prog)
				}
				return fsops.Copy(ctx, sel, to, prog)
			})
		src.markAll(false)
	})
}

// inputDialog: form with one focused entry; Enter submits, focus returns to pane after.
func inputDialog(u *ui, title, confirm, label string, e *escEntry, ok func()) {
	w := u.dialogWindow(title)
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
	w.Show()
	w.Canvas().Focus(e)
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
	msg := fmt.Sprintf("%s %d item(s)?\n%s", verb, len(sel), filepath.Base(sel[0]))
	if len(sel) > 1 {
		msg += ", …"
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

// settings: one page (guidelines). Saved immediately, like macOS settings windows.
func (u *ui) settings() {
	w := u.infoWindow("Settings")
	pr := u.app.Preferences()
	notice := widget.NewLabel("When turned off, deleted files cannot be restored from " + trashName() + ".")
	notice.Wrapping = fyne.TextWrapWord
	c := &dialogCheck{close: w.Close}
	c.Text = "Move deleted files to " + trashName()
	c.OnChanged = func(b bool) { pr.SetBool(prefTrash, b) }
	c.ExtendBaseWidget(c)
	c.Checked = pr.BoolWithFallback(prefTrash, true)
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
	revealItem := widget.NewFormItem("Reveal command", commandEntry(prefRevealCmd, reveal))
	revealItem.HintText = "{path} is the selected item; {dir} is the active pane folder."
	terminalItem := widget.NewFormItem("Terminal command", commandEntry(prefTerminalCmd, terminal))
	terminalItem.HintText = "{dir} is the active pane folder."
	help := widget.NewLabel("Custom commands use JSON argument lists, without a shell.\nLeave a field blank to use the OS default shown below.")
	help.Wrapping = fyne.TextWrapWord
	content := u.dialogContent(container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Changes are saved automatically. Invalid commands are not saved."),
		widget.NewSeparator(),
		c, notice,
		help,
		widget.NewForm(revealItem, terminalItem),
	))
	w.SetContent(content)
	w.Resize(fyne.NewSize(700, 340))
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
