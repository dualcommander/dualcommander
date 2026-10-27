package commander

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type paneSplit struct {
	container.Split
}

func newPaneSplit(left, right fyne.CanvasObject) *paneSplit {
	s := &paneSplit{Split: container.Split{Horizontal: true, Offset: 0.5, Leading: left, Trailing: right}}
	s.BaseWidget.ExtendBaseWidget(s)
	return s
}

func (s *paneSplit) CreateRenderer() fyne.WidgetRenderer {
	r := s.Split.CreateRenderer()
	// ponytail: transparent input layer until Fyne exposes divider callbacks; leave native renderer untouched.
	for _, o := range r.Objects() {
		if d, ok := o.(fyne.Draggable); ok {
			input := &paneDivider{Draggable: d, Hoverable: o.(desktop.Hoverable), Cursorable: o.(desktop.Cursorable), split: s}
			input.ExtendBaseWidget(input)
			return &paneSplitRenderer{WidgetRenderer: r, native: o, input: input}
		}
	}
	return r
}

type paneSplitRenderer struct {
	fyne.WidgetRenderer
	native fyne.CanvasObject
	input  *paneDivider
}

func (r *paneSplitRenderer) Objects() []fyne.CanvasObject {
	return append(append([]fyne.CanvasObject{}, r.WidgetRenderer.Objects()...), r.input)
}

func (r *paneSplitRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	r.input.Move(r.native.Position())
	r.input.Resize(r.native.Size())
}

func (r *paneSplitRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	r.Layout(r.input.split.Size())
}

type paneDivider struct {
	widget.BaseWidget
	fyne.Draggable
	desktop.Hoverable
	desktop.Cursorable
	split    *paneSplit
	tooltips [2]*widget.PopUp
	overlay  *fyne.Container
}

func (d *paneDivider) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}

func (d *paneDivider) Tapped(*fyne.PointEvent)       {}
func (d *paneDivider) DoubleTapped(*fyne.PointEvent) { d.split.SetOffset(0.5) }

func (d *paneDivider) Dragged(e *fyne.DragEvent) {
	d.Draggable.Dragged(e)
	left, right := d.split.Leading.Size().Width, d.split.Trailing.Size().Width
	if left+right <= 0 {
		return
	}
	percent := int(float64(left/(left+right))*100 + 0.5)
	driver := fyne.CurrentApp().Driver()
	canvas := driver.CanvasForObject(d.split)
	if d.overlay == nil {
		d.overlay = container.NewWithoutLayout()
		for i := range d.tooltips {
			d.tooltips[i] = widget.NewPopUp(widget.NewLabel(""), canvas)
			d.overlay.Add(d.tooltips[i])
		}
	}
	for i, pane := range []fyne.CanvasObject{d.split.Leading, d.split.Trailing} {
		tooltip := d.tooltips[i]
		value := percent
		if i == 1 {
			value = 100 - percent
		}
		tooltip.Content.(*widget.Label).SetText(fmt.Sprintf("%d%%", value))
		size := tooltip.MinSize()
		tooltip.Resize(size)
		pos := driver.AbsolutePositionForObject(pane)
		tooltip.Move(pos.Add(fyne.NewPos((pane.Size().Width-size.Width)/2, (pane.Size().Height-size.Height)/2)))
	}
	if canvas.Overlays().Top() != d.overlay {
		canvas.Overlays().Add(d.overlay)
	}
}

func (d *paneDivider) DragEnd() {
	d.Draggable.DragEnd()
	if d.overlay != nil {
		d.tooltips[0].Canvas.Overlays().Remove(d.overlay)
	}
}
