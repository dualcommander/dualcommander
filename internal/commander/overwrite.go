package commander

import (
	"context"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// Only the worker waits; shared gate serializes prompts from concurrent jobs.
func (u *ui) overwriteConflict(ctx context.Context, conflict fsops.Conflict) (fsops.ConflictAction, error) {
	select {
	case u.passwordGate <- struct{}{}:
	case <-ctx.Done():
		return fsops.Skip, ctx.Err()
	}
	defer func() { <-u.passwordGate }()
	type answer struct {
		action fsops.ConflictAction
		err    error
	}
	result := make(chan answer, 1)
	var closeWindow func()
	post(func() {
		if ctx.Err() != nil {
			return
		}
		u.jobs.closeMonitor()
		w := u.dialogWindow("Target already exists")
		unblock := u.blockParent(w)
		value := answer{err: context.Canceled}
		w.SetOnClosed(func() { unblock(); result <- value })
		closeWindow = w.Close
		details := func(title, path string, info os.FileInfo) fyne.CanvasObject {
			kind := "File"
			if info.IsDir() {
				kind = "Folder"
			}
			if info.Mode()&os.ModeSymlink != 0 {
				kind = "Symbolic link"
			}
			text := fmt.Sprintf("%s\nName: %s\nPath: %s\nType: %s\nSize: %s (%d bytes)\nModified: %s\nAttributes: %s", title, info.Name(), path, kind, humanSize(info.Size()), info.Size(), info.ModTime().Format("2006-01-02 15:04:05 MST"), info.Mode())
			label := widget.NewLabel(text)
			label.Wrapping = fyne.TextWrapBreak
			return label
		}
		message := "Choose what to do with this conflict. All applies only to this operation."
		if conflict.SourceInfo.IsDir() && conflict.TargetInfo.IsDir() {
			message += " Overwrite merges folders; conflicting children are checked individually."
		}
		label := widget.NewLabel(message)
		label.Wrapping = fyne.TextWrapWord
		var buttons []*dialogButton
		buttons = append(buttons, windowButton(w, "Cancel", w.Close))
		for _, option := range []struct {
			label  string
			action fsops.ConflictAction
		}{
			{"Skip", fsops.Skip}, {"Skip All", fsops.SkipAll}, {"Overwrite", fsops.Overwrite}, {"Overwrite All", fsops.OverwriteAll},
		} {
			buttons = append(buttons, windowButton(w, option.label, func() {
				value = answer{action: option.action}
				w.Close()
			}))
		}
		body := container.NewVScroll(container.NewVBox(label,
			details("Source", conflict.Source, conflict.SourceInfo),
			details("Target", conflict.Target, conflict.TargetInfo)))
		w.SetContent(u.dialogContent(container.NewBorder(nil, dialogButtons(w, 1, buttons...), nil, nil, body)))
		w.Resize(fyne.NewSize(720, 480))
		w.Canvas().Focus(buttons[1]) // safe default: Enter skips, never overwrites silently
		w.Show()
	})
	select {
	case value := <-result:
		return value.action, value.err
	case <-ctx.Done():
		post(func() {
			if closeWindow != nil {
				closeWindow()
			}
		})
		return fsops.Skip, ctx.Err()
	}
}
