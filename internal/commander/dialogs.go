package commander

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type dialogTheme struct{ fyne.Theme }

func (t dialogTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 8
	}
	return t.Theme.Size(name)
}

// Dialog spacing stays comfortable independently of file-list density.
func (u *ui) dialogContent(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(
		container.New(layout.NewCustomPaddedLayout(16, 16, 16, 16), content),
		dialogTheme{u.app.Settings().Theme()})
}

// ponytail: Fyne has no wait cursor; use a static hourglass over blocked content.
type busyBlocker struct{ widget.BaseWidget }

func (b *busyBlocker) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
func (b *busyBlocker) Cursor() desktop.Cursor { return b }
func (b *busyBlocker) Image() (image.Image, int, int) {
	im := image.NewNRGBA(image.Rect(0, 0, 24, 24))
	for y := 2; y < 22; y++ {
		half := max(1, 11-y, y-12)
		for x := 11 - half; x <= 12+half; x++ {
			c := color.NRGBA{255, 255, 255, 255}
			if x == 11-half || x == 12+half || y <= 3 || y >= 20 {
				c = color.NRGBA{0, 0, 0, 255}
			}
			im.SetNRGBA(x, y, c)
		}
	}
	return im, 12, 12
}
func (b *busyBlocker) Tapped(*fyne.PointEvent)          {}
func (b *busyBlocker) TappedSecondary(*fyne.PointEvent) {}
func (b *busyBlocker) DoubleTapped(*fyne.PointEvent)    {}
func (b *busyBlocker) MouseDown(*desktop.MouseEvent)    {}
func (b *busyBlocker) MouseUp(*desktop.MouseEvent)      {}
func (b *busyBlocker) MouseIn(*desktop.MouseEvent)      {}
func (b *busyBlocker) MouseMoved(*desktop.MouseEvent)   {}
func (b *busyBlocker) MouseOut()                        {}
func (b *busyBlocker) Scrolled(*fyne.ScrollEvent)       {}

// blockParent also covers native panels while their worker waits for AppKit.
func (u *ui) blockParent(w fyne.Window) func() {
	u.passwordWindow = w
	block := &busyBlocker{}
	block.ExtendBaseWidget(block)
	u.win.Canvas().Overlays().Add(block)
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
	return func() {
		u.win.Canvas().Overlays().Remove(block)
		for _, item := range disabled {
			item.Disabled = false
		}
		menu.Refresh()
		u.passwordWindow = nil
		u.win.RequestFocus()
		u.activate(u.active)
	}
}

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

// Default remains visible while a form field has focus; Tab selects other buttons.
func dialogButtons(w fyne.Window, selected int, buttons ...*dialogButton) fyne.CanvasObject {
	primary := buttons[selected]
	primary.Importance = widget.HighImportance
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if !escapeWindow(w)(e) && (e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter) {
			primary.TypedKey(e)
		}
	})
	objects := make([]fyne.CanvasObject, len(buttons))
	for i, b := range buttons {
		objects[i] = b
	}
	return container.NewCenter(container.NewHBox(objects...))
}

func (b *dialogButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		b.close()
	} else if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		b.Tapped(nil)
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
	no := windowButton(w, "No", w.Close)
	w.SetContent(u.dialogContent(container.NewBorder(nil,
		dialogButtons(w, 0, no, windowButton(w, "Yes", func() { w.Close(); ok() })),
		nil, nil, label)))
	w.Resize(fyne.NewSize(500, 180))
	w.Show()
	w.Canvas().Focus(no)
}

func (u *ui) errorWindow(err error) {
	w := u.dialogWindow("Error")
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapWord
	ok := windowButton(w, "OK", w.Close)
	w.SetContent(u.dialogContent(container.NewBorder(nil,
		dialogButtons(w, 0, ok), nil, nil, label)))
	w.Resize(fyne.NewSize(500, 180))
	w.Show()
	w.Canvas().Focus(ok)
}
