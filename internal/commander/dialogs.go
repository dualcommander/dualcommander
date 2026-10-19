package commander

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// dialogWindow keeps native decorations; closing never confirms an operation.
func (u *ui) dialogWindow(title string) fyne.Window {
	w := u.app.NewWindow(title)
	w.SetIcon(appIcon)
	w.CenterOnScreen()
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	w.SetOnClosed(func() { u.win.RequestFocus(); u.activate(u.active) })
	return w
}

func escapeWindow(w fyne.Window) func(*fyne.KeyEvent) bool {
	return func(e *fyne.KeyEvent) bool {
		if e.Name != fyne.KeyEscape {
			return false
		}
		w.Close()
		return true
	}
}

// Fyne delivers keys to the focused widget, not its canvas.
type dialogButton struct {
	widget.Button
	close func()
}

func windowButton(w fyne.Window, text string, tapped func()) *dialogButton {
	b := &dialogButton{close: w.Close}
	b.Text, b.OnTapped = text, tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *dialogButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		b.close()
	} else {
		b.Button.TypedKey(e)
	}
}

type dialogCheck struct {
	widget.Check
	close func()
}

type dialogList struct {
	widget.List
	close func()
}

func (l *dialogList) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		l.close()
	} else {
		l.List.TypedKey(e)
	}
}

func (c *dialogCheck) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		c.close()
	} else {
		c.Check.TypedKey(e)
	}
}

func (u *ui) confirm(title, message string, ok func()) {
	w := u.dialogWindow(title)
	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewBorder(nil,
		container.NewHBox(windowButton(w, "No", w.Close), windowButton(w, "Yes", func() { w.Close(); ok() })),
		nil, nil, label)))
	w.Resize(fyne.NewSize(500, 180))
	w.Show()
}

func (u *ui) errorWindow(err error) {
	w := u.dialogWindow("Error")
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewBorder(nil,
		windowButton(w, "OK", w.Close), nil, nil, label)))
	w.Resize(fyne.NewSize(500, 180))
	w.Show()
}
