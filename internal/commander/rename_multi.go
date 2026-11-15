package commander

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (u *ui) renameMulti() {
	if u.passwordWindow != nil {
		return
	}
	p := u.active
	if p.inArchive() {
		u.showErr(errors.New(lang.L("archive browsing is read-only")))
		return
	}
	selected := p.selection()
	if len(selected) == 0 {
		return
	}
	selectedNames := make(map[string]bool, len(selected))
	for _, path := range selected {
		selectedNames[filepath.Base(path)] = true
	}
	entries := slices.Clone(p.entries)
	dir := p.path
	w := u.dialogWindow(lang.L("Multi-Rename"))
	closed, generation := false, 0
	var cancelDelay func()
	unblock := u.blockParent(w)
	bar := widget.NewProgressBar()
	bar.TextFormatter = func() string { return "" }
	track := container.New(operationTrackLayout{}, bar)
	track.Hide()
	w.SetOnClosed(func() {
		closed = true
		generation++
		if cancelDelay != nil {
			cancelDelay()
		}
		track.Hide()
		unblock()
	})
	from, to := newEscEntry(), newEscEntry()
	from.SetPlaceHolder(lang.L("Match template"))
	to.SetPlaceHolder(lang.L("Name template"))
	from.SetText("{name}.{ext}")
	to.SetText("{{.name}}.{{.ext}}")
	for _, e := range []*escEntry{from, to} {
		e.esc = escapeWindow(w)
		e.AlwaysShowValidationError = true
	}
	from.Validator = func(value string) error { _, _, err := renameTemplate(value, ""); return err }
	to.Validator = func(value string) error { _, _, err := renameTemplate(from.Text, value); return err }
	var oldNames, newNames, captureRows []string
	files, folders := 0, 0
	for _, e := range entries {
		if selectedNames[e.name] {
			if e.dir {
				folders++
			} else {
				files++
			}
		}
	}
	status := widget.NewLabel("")
	status.Importance = widget.DangerImportance
	showError := func(text string) {
		status.SetText(text)
		if text == "" {
			status.Hide()
		} else {
			status.Show()
		}
	}
	counts := widget.NewLabel(fmt.Sprintf(lang.L("%d files · %d folders"), files, folders))
	status.Wrapping = fyne.TextWrapWord
	apply := windowButton(w, lang.L("Rename"), nil)
	rowTheme := dialogTheme{u.app.Settings().Theme()}
	list := &dialogList{close: w.Close}
	list.Length = func() int { return len(oldNames) }
	list.CreateItem = func() fyne.CanvasObject {
		// Keep a text line during initial measurement so selection cannot change row height.
		row := container.NewGridWithColumns(2, widget.NewLabel(" "), widget.NewLabel(" "))
		// Virtualized rows are created outside the dialog's rendering theme scope.
		return container.NewThemeOverride(row, rowTheme)
	}
	list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) {
		box := o.(*container.ThemeOverride).Content.(*fyne.Container)
		box.Objects[0].(*widget.Label).SetText(oldNames[i])
		text := newNames[i]
		if captureRows[i] != "" {
			text += " (" + captureRows[i] + ")"
		}
		box.Objects[1].(*widget.Label).SetText(text)
	}
	list.ExtendBaseWidget(list)
	invalidate := func() {
		generation++
		oldNames, newNames, captureRows = nil, nil, nil
		if cancelDelay != nil {
			cancelDelay()
			cancelDelay = nil
		}
		apply.Disable()
		track.Hide()
		list.UnselectAll()
		list.Refresh()
		showError("")
		from.Validate()
		to.Validate()
	}
	preview := func() {
		if err := errors.Join(from.Validate(), to.Validate()); err != nil {
			status.SetText(err.Error())
			return
		}
		rule, replacement, _ := renameTemplate(from.Text, to.Text)
		bar.SetValue(0)
		track.Show()
		current := generation
		var next, files, folders, index int
		existing := make(map[string]bool, len(entries))
		for _, e := range entries {
			existing[e.name] = true
		}
		var scan func()
		scan = func() {
			if closed || generation != current {
				return
			}
			end := min(next+256, len(entries))
			for _, e := range entries[next:end] {
				if !selectedNames[e.name] {
					continue
				}
				groups := rule.FindStringSubmatch(e.name)
				if groups == nil {
					oldNames = append(oldNames, e.name)
					newNames = append(newNames, e.name)
					captureRows = append(captureRows, "")
					continue
				}
				index++
				valuesMap := map[string]any{"Index": index}
				for i, key := range rule.SubexpNames()[1:] {
					valuesMap[key] = groups[i+1]
				}
				var rendered strings.Builder
				if err := replacement.Execute(&rendered, valuesMap); err != nil {
					showError(err.Error())
					track.Hide()
					return
				}
				name := rendered.String()
				oldNames = append(oldNames, e.name)
				newNames = append(newNames, name)
				captureRows = append(captureRows, "")
				if name == e.name {
					captureRows[len(captureRows)-1] = lang.L("Unchanged")
					continue
				}
				if e.dir {
					folders++
				} else {
					files++
				}
			}
			next = end
			bar.SetValue(float64(next) / float64(len(entries)))
			if next < len(entries) {
				post(scan)
				return
			}
			track.Hide()
			errs := fsops.CheckRenames(oldNames, newNames, func(name string) bool { return existing[name] })
			valid := files+folders > 0
			for i, err := range errs {
				if err != nil {
					valid = false
					captureRows[i] = err.Error()
					showError(oldNames[i] + ": " + err.Error())
				}
			}
			list.Refresh()
			if valid {
				apply.Enable()
			}
		}
		post(scan)
	}
	after := u.renameAfter
	if after == nil {
		after = func(delay time.Duration, f func()) func() {
			timer := time.AfterFunc(delay, func() { post(f) })
			return func() { timer.Stop() }
		}
	}
	schedule := func() {
		invalidate()
		if err := from.Validate(); err != nil {
			showError(lang.L("Match template") + ": " + err.Error())
			return
		}
		if err := to.Validate(); err != nil {
			showError(lang.L("Name template") + ": " + err.Error())
			return
		}
		current := generation
		cancelDelay = after(250*time.Millisecond, func() {
			if !closed && generation == current {
				preview()
			}
		})
	}
	for _, e := range []*escEntry{from, to} {
		e.OnChanged = func(string) { schedule() }
	}
	apply.OnTapped = func() {
		if apply.Disabled() {
			return
		}
		old, names := slices.Clone(oldNames), slices.Clone(newNames)
		w.Close()
		u.jobs.run(lang.L("Multi-Rename"), []*pane{p, u.other()}, func(ctx context.Context, progress fsops.Progress) error {
			return fsops.Rename(ctx, dir, old, names, progress)
		})
	}
	examples := lang.L("Capture text with {name}; reuse it as {{.name}}. Match the whole name, case-sensitive. {} is an anonymous capture: first {{.C0}}, then {{.C1}}.") + "\n\n" +
		"{name}.jpg  →  {{.name}}.png\nphoto.jpg  →  photo.png\n\n" +
		"{name}  →  backup_{{.name}}\nnotes.txt  →  backup_notes.txt\n\n" +
		"{first}-{last}.{ext}  →  {{.last}}-{{.first}}.{{.ext}}\none-two.jpg  →  two-one.jpg\n\n" +
		"{name}.{ext}  →  {{kebab .name}}.{{lower .ext}}\nCaaZaa.JPG  →  caa-zaa.jpg\n\n" +
		"{name}.{ext}  →  {{.name}}_{{index 3}}.{{.ext}}\nphoto.jpg  →  photo_001.jpg\n\n" +
		"{name}.jpeg  →  {{index 2}}.jpeg\n01.jpeg, 02.jpeg, …\n{{index}} / {{.Index}} / {{printf \"%02d\" .Index}}\n\n" +
		lang.L("Index starts at 1 for matching selected items in pane order. Padding: %01d to %09d.") + "\n\n" +
		lang.L("Helpers: upper, lower, snake, kebab, camel, pascal. Each takes one capture. No loops, conditions, nested calls, or pipelines.")
	help := newSearchOption(w, "", lang.L("Multi-Rename"), examples)
	help.Text = ""
	help.Icon = newDialogInfoIcon().Resource
	help.OnTapped = func() {
		if help.hint.Visible() {
			help.hint.Hide()
		} else {
			help.hint.Show()
		}
	}
	manual := &dialogHyperlink{close: w.Close}
	manual.Text = lang.L("User manual")
	manual.URL = &url.URL{Scheme: "https", Host: "dualcommander.github.io", Path: "/manual.html", Fragment: "multi-rename"}
	manual.OnTapped = func() { u.showErr(u.app.OpenURL(manual.URL)) }
	manual.ExtendBaseWidget(manual)
	guidance := container.NewHBox(help, container.NewThemeOverride(manual, dialogHyperlinkTheme{rowTheme}))
	form := widget.NewForm(
		widget.NewFormItem(lang.L("Match template"), from),
		widget.NewFormItem(lang.L("Name template"), to),
	)
	header := container.NewGridWithColumns(2, widget.NewLabel(lang.L("Original")), widget.NewLabel(lang.L("New name")))
	w.SetContent(container.NewStack(u.dialogContent(container.NewBorder(container.NewVBox(guidance, form, status, header), container.NewVBox(counts, track, dialogButtons(w, 1, windowButton(w, lang.L("Cancel"), w.Close), apply)), nil, nil, list)), help.hint))
	schedule()
	w.Resize(fyne.NewSize(700, 600))
	w.Canvas().Focus(from)
	w.Show()
}
