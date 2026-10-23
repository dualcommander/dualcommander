package commander

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2/lang"
	"path/filepath"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

type operation struct {
	title, file           string
	done, total           int64
	filesDone, filesTotal int64
	counted               bool
	cancel                context.CancelFunc
	cancelling            bool
}

type jobs struct {
	ui       *ui
	box      *fyne.Container
	active   []*operation
	button   *widget.Button
	bar      *widget.ProgressBar
	activity *widget.ProgressBarInfinite
	monitor  fyne.Window
	list     *dialogList
}

// Keep native progress animation, but separate thin track from accessible click target.
type operationStripLayout struct{}

// Keep progress slim even when dialog rows have room to expand.
type operationTrackLayout struct{}

func (operationTrackLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 3)
}

func (operationTrackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, min(float32(3), size.Height)))
}

func (operationStripLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return objects[2].MinSize().AddWidthHeight(0, 3)
}

func (operationStripLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	height := min(float32(3), size.Height)
	for _, bar := range objects[:2] {
		bar.Move(fyne.NewPos(0, size.Height-height))
		bar.Resize(fyne.NewSize(size.Width, height))
	}
	objects[2].Move(fyne.NewPos(0, 0))
	objects[2].Resize(fyne.NewSize(size.Width, size.Height-height))
}

func (j *jobs) run(title string, refresh []*pane, fn func(context.Context, fsops.Progress) error) {
	j.runCounted(title, refresh, func(ctx context.Context, report func(fsops.CopyProgress)) error {
		return fn(ctx, func(done, total int64, file string) {
			report(fsops.CopyProgress{Done: done, Total: total, Current: file, FilesTotal: -1})
		})
	})
}

func (j *jobs) runCounted(title string, refresh []*pane, fn func(context.Context, func(fsops.CopyProgress)) error) {
	ctx, cancel := context.WithCancel(context.Background())
	op := &operation{title: title, cancel: cancel}
	j.active = append(j.active, op)
	j.update()
	go func() {
		err := fn(ctx, latestUpdate(post, func(p fsops.CopyProgress) {
			if !slices.Contains(j.active, op) {
				return
			}
			op.done, op.total, op.file = p.Done, p.Total, p.Current
			op.filesDone, op.filesTotal, op.counted = p.FilesDone, p.FilesTotal, p.FilesTotal >= 0
			if op.counted && op.total == 0 {
				op.done, op.total = p.FilesDone, p.FilesTotal
			}
			j.update()
		}))
		cancelled := ctx.Err() != nil
		cancel()
		post(func() {
			j.active = slices.DeleteFunc(j.active, func(o *operation) bool { return o == op })
			j.update()
			for _, p := range refresh {
				p.refresh()
			}
			if err != nil && !cancelled && !errors.Is(err, context.Canceled) {
				j.closeMonitor()
				j.ui.showErr(fmt.Errorf("%s: %w", title, err))
			}
		})
	}()
}

func (j *jobs) update() {
	if j.list != nil {
		j.list.Refresh()
	}
	if len(j.active) == 0 {
		if j.activity != nil {
			j.activity.Stop()
		}
		j.box.RemoveAll()
		j.box.Hide()
		return
	}
	if j.button == nil {
		j.button = widget.NewButton("", j.show)
		j.button.Importance = widget.LowImportance
		j.bar = widget.NewProgressBar()
		j.bar.TextFormatter = func() string { return "" }
		j.activity = widget.NewProgressBarInfinite()
		j.activity.Stop()
	}
	text := fmt.Sprintf(lang.L("%d operations · Show details"), len(j.active))
	// ponytail: progress callbacks have no unit; mixed tasks are not safely summable.
	// Add explicit work units before aggregating multiple task percentages.
	known := len(j.active) == 1 && j.active[0].total > 0
	if known {
		op := j.active[0]
		value := min(1.0, max(0.0, float64(op.done)/float64(op.total)))
		text = fmt.Sprintf(lang.L("1 operation · %.0f%% · Show details"), value*100)
		if op.counted {
			text = fmt.Sprintf(lang.L("1 operation · %.0f%% · %d/%d files · Show details"), value*100, op.filesDone, op.filesTotal)
		}
		j.bar.SetValue(value)
		j.activity.Stop()
		j.activity.Hide()
		j.bar.Show()
	} else {
		j.bar.Hide()
		j.activity.Show()
		j.activity.Start()
	}
	j.button.SetText(text)
	if len(j.box.Objects) == 0 {
		j.box.Add(container.New(operationStripLayout{}, j.bar, j.activity, j.button))
	}
	j.box.Show()
}

func (j *jobs) closeMonitor() {
	if j.monitor != nil {
		j.monitor.Close()
	}
}

func (j *jobs) show() {
	if j.monitor != nil {
		j.monitor.RequestFocus()
		return
	}
	if j.ui.passwordWindow != nil || len(j.active) == 0 {
		return
	}
	w := j.ui.dialogWindow(lang.L("Running Operations"))
	j.monitor = w
	unblock := j.ui.blockParent(w)
	l := &dialogList{close: w.Close}
	l.Length = func() int { return len(j.active) }
	l.CreateItem = func() fyne.CanvasObject {
		title, file := widget.NewLabel(lang.L("Operation")), widget.NewLabel(lang.L("Current file"))
		title.Truncation, file.Truncation = fyne.TextTruncateEllipsis, fyne.TextTruncateEllipsis
		bar := widget.NewProgressBar()
		bar.TextFormatter = func() string { return "" }
		return container.NewBorder(nil, nil, nil, windowButton(w, lang.L("Cancel"), nil),
			container.NewVBox(title, file, container.New(operationTrackLayout{}, bar)))
	}
	l.UpdateItem = func(i widget.ListItemID, obj fyne.CanvasObject) {
		op := j.active[i]
		row := obj.(*fyne.Container)
		fields := row.Objects[0].(*fyne.Container).Objects
		fields[0].(*widget.Label).SetText(op.title)
		file := lang.L("Discovering work…")
		if op.file != "" {
			file = filepath.Base(op.file)
		}
		if op.total <= 0 {
			file += lang.L(" · Working…")
		} else {
			file += fmt.Sprintf(" · %.0f%%", min(1.0, max(0.0, float64(op.done)/float64(op.total)))*100)
		}
		if op.counted {
			file += fmt.Sprintf(lang.L(" · %d/%d files"), op.filesDone, op.filesTotal)
		}
		fields[1].(*widget.Label).SetText(file)
		bar := fields[2].(*fyne.Container).Objects[0].(*widget.ProgressBar)
		bar.SetValue(0)
		bar.Hide()
		if op.total > 0 {
			bar.SetValue(float64(op.done) / float64(op.total))
			bar.Show()
		}
		stop := row.Objects[1].(*dialogButton)
		stop.OnTapped = func() { op.cancelling = true; op.cancel(); stop.Disable() }
		if op.cancelling {
			stop.Disable()
		} else {
			stop.Enable()
		}
	}
	l.ExtendBaseWidget(l)
	j.list = l
	w.SetOnClosed(func() { j.monitor, j.list = nil, nil; unblock() })
	close := windowButton(w, lang.L("Close"), w.Close)
	w.SetContent(j.ui.dialogContent(container.NewBorder(nil, dialogButtons(w, 0, close), nil, nil, l)))
	w.Resize(fyne.NewSize(600, 360))
	w.Show()
	w.Canvas().Focus(close)
}
