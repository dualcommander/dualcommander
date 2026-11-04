package commander

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/archive"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

func (u *ui) pack() {
	u.packSelection(u.active.selection())
}

func (u *ui) packSelection(selection []string) {
	if u.passwordWindow != nil {
		return
	}
	src, dst := u.active, u.other()
	if src.inArchive() || dst.inArchive() {
		u.showErr(errors.New(lang.L("archive browsing is read-only")))
		return
	}
	if len(selection) == 0 {
		return
	}
	dir := dst.path
	w := u.dialogWindow(lang.L("Create Archive"))
	name, password, confirm := newEscEntry(), newEscEntry(), newEscEntry()
	password.Password, confirm.Password = true, true
	for _, entry := range []*escEntry{name, password, confirm} {
		entry.esc = escapeWindow(w)
	}
	w.SetOnClosed(func() { password.SetText(""); confirm.SetText("") })
	format := &dialogSelect{close: w.Close}
	format.Options = []string{".zip", ".tar", ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"}
	format.ExtendBaseWidget(format)
	format.OnChanged = func(ext string) {
		for _, old := range format.Options {
			if strings.HasSuffix(strings.ToLower(name.Text), old) {
				name.SetText(name.Text[:len(name.Text)-len(old)] + ext)
				break
			}
		}
		if ext == ".zip" {
			password.Enable()
			confirm.Enable()
		} else {
			password.SetText("")
			confirm.SetText("")
			password.Disable()
			confirm.Disable()
		}
	}
	name.SetText(filepath.Base(selection[0]) + ".zip")
	format.SetSelected(".zip")
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "/\\:\x00") || !strings.HasSuffix(strings.ToLower(value), format.Selected) || len(value) <= len(format.Selected) {
			return errors.New(lang.L("Enter a file name with the selected archive extension"))
		}
		return nil
	}
	confirm.Validator = func(value string) error {
		if value != password.Text {
			return errors.New(lang.L("Passwords do not match"))
		}
		return nil
	}
	submit := func() {
		if name.Validate() != nil || confirm.Validate() != nil {
			return
		}
		file, secret := filepath.Join(dir, name.Text), password.Text
		w.Close()
		u.jobs.run(lang.L("Create Archive"), []*pane{src, dst}, func(ctx context.Context, progress fsops.Progress) error {
			return archive.Pack(ctx, selection, file, progress, secret)
		})
	}
	for _, entry := range []*escEntry{name, password, confirm} {
		entry.OnSubmitted = func(string) { submit() }
	}
	note := widget.NewLabel(lang.L("Password protection: ZIP only (AES-256). File names remain visible.\nSome archive tools cannot open encrypted ZIP files. Existing files are never replaced."))
	note.Wrapping = fyne.TextWrapWord
	w.SetContent(u.dialogContent(container.NewVBox(
		widget.NewLabel(lang.L("Create in: ")+dir),
		widget.NewForm(widget.NewFormItem(lang.L("Archive name"), name), widget.NewFormItem(lang.L("Format"), format),
			widget.NewFormItem(lang.L("Password (optional)"), password), widget.NewFormItem(lang.L("Confirm password"), confirm)),
		note,
		dialogButtons(w, 1, windowButton(w, lang.L("Cancel"), w.Close), windowButton(w, lang.L("Create"), submit)),
	)))
	w.Resize(fyne.NewSize(560, w.Content().MinSize().Height))
	w.Show()
	w.Canvas().Focus(name)
}
