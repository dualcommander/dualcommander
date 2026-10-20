package commander

import (
	"context"
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
		if ctx.Err() != nil {
			return
		}
		w := u.dialogWindow("Archive password")
		u.passwordWindow = w
		e := newEscEntry()
		e.Password = true
		e.esc = escapeWindow(w)
		message := "Enter password for " + filepath.Base(name)
		if retry {
			message = "Password incorrect, or encrypted data damaged. Try again."
		}
		label := widget.NewLabel(message)
		label.Wrapping = fyne.TextWrapWord
		// Fyne has no decorated modal-window API. Its modal overlay blocks parent input;
		// password controls live in a separate OS-decorated window, not an in-canvas dialog.
		block := widget.NewModalPopUp(widget.NewLabel("Waiting for archive password…"), u.win.Canvas())
		block.Show()
		u.win.Canvas().Unfocus()
		menu := u.win.MainMenu()
		var disabled []*fyne.MenuItem
		var disable func(*fyne.Menu)
		disable = func(menu *fyne.Menu) {
			for _, item := range menu.Items {
				if item.ChildMenu != nil {
					disable(item.ChildMenu)
				}
				if !item.Disabled {
					item.Disabled = true
					disabled = append(disabled, item)
				}
			}
		}
		for _, child := range menu.Items {
			disable(child)
		}
		menu.Refresh()
		value := answer{}
		w.SetOnClosed(func() {
			e.SetText("")
			block.Hide()
			for _, item := range disabled {
				item.Disabled = false
			}
			menu.Refresh()
			u.passwordWindow = nil
			u.win.RequestFocus()
			u.activate(u.active)
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
		w.SetContent(container.NewPadded(container.NewVBox(label,
			widget.NewForm(widget.NewFormItem("Password", e)),
			container.NewHBox(windowButton(w, "Cancel", w.Close), windowButton(w, "Unlock", submit)))))
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
