package commander

import (
	"context"
	"fyne.io/fyne/v2/lang"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// archivePassword is called by jobs only. All window and widget access stays on the UI thread.
func (u *ui) archivePassword(ctx context.Context, name string, retry bool) (string, error) {
	select {
	case u.passwordGate <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-u.passwordGate }()
	type answer struct {
		password string
		ok       bool
	}
	result := make(chan answer, 1)
	var closeWindow func()
	post(func() {
		u.jobs.closeMonitor()
		if ctx.Err() != nil {
			return
		}
		w := u.dialogWindow(lang.L("Archive password"))
		unblock := u.blockParent(w)
		e := newEscEntry()
		e.Password = true
		e.esc = escapeWindow(w)
		message := lang.L("Enter password for {{.Name}}", map[string]any{"Name": filepath.Base(name)})
		if retry {
			message = lang.L("Password incorrect, or encrypted data damaged. Try again.")
		}
		label := widget.NewLabel(message)
		label.Wrapping = fyne.TextWrapWord
		value := answer{}
		w.SetOnClosed(func() {
			e.SetText("")
			unblock()
			result <- value
		})
		closeWindow = w.Close
		submit := func() {
			if e.Text == "" {
				return
			}
			value = answer{e.Text, true}
			w.Close()
		}
		e.OnSubmitted = func(string) { submit() }
		w.SetContent(u.dialogContent(container.NewVBox(label,
			widget.NewForm(widget.NewFormItem(lang.L("Password"), e)),
			dialogButtons(w, 1, windowButton(w, lang.L("Cancel"), w.Close), windowButton(w, lang.L("Unlock"), submit)))))
		w.Resize(fyne.NewSize(500, w.Content().MinSize().Height))
		w.Show()
		w.Canvas().Focus(e)
	})
	select {
	case value := <-result:
		if value.ok {
			return value.password, nil
		}
		return "", context.Canceled
	case <-ctx.Done():
		post(func() {
			if closeWindow != nil {
				closeWindow()
			}
		})
		return "", ctx.Err()
	}
}
