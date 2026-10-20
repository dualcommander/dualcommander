package commander

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// pathEntry retains the full editable path behind a compact, unfocused label.
type pathEntry struct {
	widget.Entry
	editing bool
}

func newPathEntry() *pathEntry {
	e := &pathEntry{}
	e.Wrapping, e.Scroll = fyne.TextWrapOff, fyne.ScrollNone
	e.ExtendBaseWidget(e)
	return e
}

func (e *pathEntry) FocusGained() { e.editing = true; e.Entry.FocusGained() }
func (e *pathEntry) FocusLost()   { e.editing = false; e.Entry.FocusLost(); e.Refresh() }

func (e *pathEntry) CreateRenderer() fyne.WidgetRenderer {
	label := widget.NewLabel("")
	background := canvas.NewRectangle(e.Theme().Color(theme.ColorNameInputBackground, fyne.CurrentApp().Settings().ThemeVariant()))
	return &pathRenderer{WidgetRenderer: e.Entry.CreateRenderer(), entry: e, label: label,
		background: background, cover: container.NewStack(background, label)}
}

type pathRenderer struct {
	fyne.WidgetRenderer
	entry      *pathEntry
	label      *widget.Label
	background *canvas.Rectangle
	cover      *fyne.Container
}

func (r *pathRenderer) MinSize() fyne.Size {
	size := r.WidgetRenderer.MinSize()
	th := r.entry.Theme()
	size.Width = fyne.MeasureText("...", th.Size(theme.SizeNameText), fyne.TextStyle{}).Width + 2*th.Size(theme.SizeNameInnerPadding)
	return size
}

func (r *pathRenderer) Objects() []fyne.CanvasObject {
	return append(r.WidgetRenderer.Objects(), r.cover)
}

func (r *pathRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	r.cover.Resize(size)
	r.update()
}

func (r *pathRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	r.update()
}

func (r *pathRenderer) update() {
	if r.entry.editing {
		r.cover.Hide()
		return
	}
	r.background.FillColor = r.entry.Theme().Color(theme.ColorNameInputBackground, fyne.CurrentApp().Settings().ThemeVariant())
	r.background.Refresh()
	width := r.entry.Size().Width - 2*r.entry.Theme().Size(theme.SizeNameInnerPadding)
	r.label.SetText(shortPath(r.entry.Text, func(s string) bool {
		return fyne.MeasureText(s, r.entry.Theme().Size(theme.SizeNameText), fyne.TextStyle{}).Width <= width
	}))
	r.cover.Show()
}

// shortPath drops middle directories before shortening the current folder name.
func shortPath(path string, fits func(string) bool) string {
	if fits(path) {
		return path
	}
	separator := "/"
	if strings.Contains(path, `\`) {
		separator = `\`
	}
	parts := strings.Split(strings.TrimRight(path, separator), separator)
	for keep := len(parts) - 2; keep >= 1; keep-- {
		candidate := strings.Join(parts[:keep], separator) + separator + "..." + separator + parts[len(parts)-1]
		if fits(candidate) {
			return candidate
		}
	}
	name := parts[len(parts)-1]
	if fits("..." + separator + name) {
		return "..." + separator + name
	}
	for runes := []rune(name); len(runes) > 0; runes = runes[:len(runes)-1] {
		if candidate := string(runes) + "..."; fits(candidate) {
			return candidate
		}
	}
	return "..."
}
