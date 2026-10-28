package commander

import (
	"fyne.io/fyne/v2/lang"
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	native "github.com/dualcommander/dualcommander/pkg/desktop"
)

type dialogTheme struct{ fyne.Theme }

// Fyne Icon skips refreshing unchanged resources, leaving cached tint after theme switches.
type dialogInfoIcon struct{ widget.Icon }

func newDialogInfoIcon() *dialogInfoIcon {
	i := &dialogInfoIcon{}
	i.ExtendBaseWidget(i)
	i.Refresh()
	return i
}

func (i *dialogInfoIcon) Refresh() {
	i.Resource = theme.NewColoredResource(theme.InfoIcon(), theme.ColorNameForeground)
	i.Icon.Refresh()
}

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
	if modal, ok := w.(*modalWindow); ok {
		w = modal.Window
	}
	previous := u.passwordWindow
	u.passwordWindow = w
	u.blockNativeParent(true)
	block := &busyBlocker{}
	block.ExtendBaseWidget(block)
	u.win.Canvas().Overlays().Add(block)
	u.win.Canvas().Unfocus()
	menu := u.win.MainMenu()
	var disabled []*fyne.MenuItem
	var disable func(*fyne.Menu)
	disable = func(menu *fyne.Menu) {
		for _, item := range menu.Items {
			// Native menus must still dispatch text selection to the active dialog.
			if _, ok := item.Shortcut.(*fyne.ShortcutSelectAll); ok && w != u.win {
				continue
			}
			if item.ChildMenu != nil {
				disable(item.ChildMenu)
			}
			if !item.Disabled {
				item.Disabled = true
				disabled = append(disabled, item)
			}
		}
	}
	if menu != nil {
		for _, child := range menu.Items {
			disable(child)
		}
		menu.Refresh()
	}
	closed := false
	return func() {
		if closed {
			return
		}
		closed = true
		u.win.Canvas().Overlays().Remove(block)
		for _, item := range disabled {
			item.Disabled = false
		}
		if menu != nil {
			menu.Refresh()
		}
		u.passwordWindow = previous
		if previous != nil {
			previous.RequestFocus()
		} else {
			u.blockNativeParent(false)
			u.win.RequestFocus()
			if u.active != nil {
				u.activate(u.active)
			}
		}
	}
}

func (u *ui) blockNativeParent(blocked bool) {
	if window, ok := u.win.(driver.NativeWindow); ok {
		window.RunNative(func(context any) { native.SetWindowBlocked(context, blocked) })
	}
}

// dialogWindow keeps native decorations; closing never confirms an operation.

// ponytail: cache one input window after first use; Fyne cannot prepare native windows invisibly.
type inputWindow struct {
	fyne.Window
	closed func()
	open   bool
}

func (w *inputWindow) Close() {
	if !w.open {
		return
	}
	w.open = false
	w.Hide()
	w.SetTitle("")
	w.Canvas().Unfocus()
	w.SetContent(container.NewWithoutLayout())
	w.Canvas().SetOnTypedKey(nil)
	closed := w.closed
	w.closed = nil
	if closed != nil {
		closed()
	}
}

func (w *inputWindow) SetOnClosed(closed func()) { w.closed = closed }

func (u *ui) inputDialogWindow(title string) fyne.Window {
	if u.inputWindow == nil {
		native := u.dialogWindow(title)
		w := &inputWindow{Window: native}
		u.inputWindow = w
		native.SetCloseIntercept(w.Close)
		native.SetOnClosed(func() {
			u.inputWindow = nil
			w.Close()
		})
	}
	w := u.inputWindow
	w.open = true
	w.SetTitle(title)
	w.CenterOnScreen()
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { escapeWindow(w)(e) })
	return w
}

func (u *ui) dialogWindow(title string) fyne.Window {
	w := &modalWindow{Window: u.app.NewWindow(title), ui: u}
	w.Window.SetOnClosed(func() {
		if w.unblock != nil {
			w.unblock()
			w.unblock = nil
		}
		if w.closed != nil {
			w.closed()
		}
	})
	w.CenterOnScreen()
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape {
			w.Close()
		}
	})
	return w
}

// modalWindow applies native relationships after Show creates the native handle.
// Existing workflow-specific cancellation callbacks remain independent of blocking.
type modalWindow struct {
	fyne.Window
	ui      *ui
	unblock func()
	closed  func()
}

func (w *modalWindow) SetOnClosed(closed func()) { w.closed = closed }

func (w *modalWindow) Hide() {
	w.Window.Hide()
	if w.unblock != nil {
		w.unblock()
		w.unblock = nil
	}
}

func (w *modalWindow) Show() {
	if w.ui.passwordWindow == nil {
		w.unblock = w.ui.blockParent(w)
	}
	w.Window.Show()
	parent, parentOK := w.ui.win.(driver.NativeWindow)
	child, childOK := w.Window.(driver.NativeWindow)
	if parentOK && childOK {
		parent.RunNative(func(parentContext any) {
			child.RunNative(func(childContext any) { native.SetModalParent(parentContext, childContext) })
		})
	}
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
	hint  *fyne.Container
}

type dialogSelect struct {
	widget.Select
	close func()
}

// ponytail: in-content hover hints reuse search layout, avoiding input-blocking popups.
func (c *dialogCheck) MouseIn(e *desktop.MouseEvent) {
	c.Check.MouseIn(e)
	if c.hint != nil {
		c.hint.Show()
		c.hint.Refresh()
	}
}

func (c *dialogCheck) hideHint() {
	if c.hint != nil {
		c.hint.Hide()
	}
}

func (c *dialogCheck) MouseOut() { c.Check.MouseOut(); c.hideHint() }
func (c *dialogCheck) FocusGained() {
	c.Check.FocusGained()
	if c.hint != nil {
		c.hint.Show()
		c.hint.Refresh()
	}
}
func (c *dialogCheck) FocusLost()                { c.Check.FocusLost(); c.hideHint() }
func (c *dialogCheck) Tapped(e *fyne.PointEvent) { c.Check.Tapped(e); c.hideHint() }
func (c *dialogCheck) TypedRune(r rune)          { c.Check.TypedRune(r); c.hideHint() }

// ponytail: retain Fyne's select rendering; replace its animated, duplicate-value popup only.
func (s *dialogSelect) Tapped(*fyne.PointEvent) {
	if s.Disabled() {
		return
	}
	driver := fyne.CurrentApp().Driver()
	c := driver.CanvasForObject(s)
	if c == nil {
		return
	}
	c.Focus(s)
	var items []*fyne.MenuItem
	for _, option := range s.Options {
		if option == s.Selected {
			continue
		}
		items = append(items, fyne.NewMenuItem(option, func() { s.SetSelected(option) }))
	}
	if len(items) == 0 {
		return
	}
	popup := widget.NewPopUpMenu(fyne.NewMenu("", items...), c)
	popup.ShowAtPosition(driver.AbsolutePositionForObject(s).Add(fyne.NewPos(0, s.Size().Height)))
	popup.Resize(fyne.NewSize(s.Size().Width, popup.MinSize().Height))
}

func (s *dialogSelect) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		s.close()
	} else if e.Name == fyne.KeySpace || e.Name == fyne.KeyUp || e.Name == fyne.KeyDown || e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		s.Tapped(nil)
	} else {
		s.Select.TypedKey(e)
	}
}

type dialogList struct {
	widget.List
	close    func()
	activate func()
}

func (l *dialogList) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		l.close()
	} else if l.activate != nil && (e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter) {
		l.activate()
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
	w.SetOnClosed(u.blockParent(w))
	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord
	cancel := windowButton(w, lang.L("Cancel"), w.Close)
	w.SetContent(u.dialogContent(container.New(layout.NewCustomPaddedVBoxLayout(12), label,
		dialogButtons(w, 0, cancel, windowButton(w, title, func() { w.Close(); ok() })),
	)))
	w.Resize(fyne.NewSize(500, w.Content().MinSize().Height))
	w.Canvas().Focus(cancel)
	w.Show()
}

func (u *ui) errorWindow(err error) {
	w := u.dialogWindow(lang.L("Error"))
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapWord
	ok := windowButton(w, lang.L("OK"), w.Close)
	w.SetContent(u.dialogContent(container.NewBorder(nil,
		dialogButtons(w, 0, ok), nil, nil, label)))
	w.Resize(fyne.NewSize(500, 180))
	w.Show()
	w.Canvas().Focus(ok)
}
