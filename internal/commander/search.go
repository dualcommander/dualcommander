package commander

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// searchCmd picks native tooling: ripgrep if installed (fastest), else OS tools.
// text == "" means name-only search by glob.
func searchCmd(goos string, rg bool, dir, text, glob string) []string {
	if glob == "" {
		glob = "*"
	}
	switch {
	case rg && text == "":
		return []string{"rg", "--files", "-uu", "--iglob", glob, dir}
	case rg:
		return []string{"rg", "-l", "-i", "-F", "-uu", "--iglob", glob, "-e", text, dir}
	case goos == "windows" && text == "":
		return []string{"where", "/r", dir, glob}
	case goos == "windows": // ponytail: findstr output uses OEM codepage; non-ASCII paths may garble. Install rg.
		return []string{"findstr", "/s", "/m", "/i", "/l", "/c:" + text, filepath.Join(dir, glob)}
	case text == "":
		return []string{"find", dir, "-iname", glob}
	default:
		return []string{"grep", "-rlIiF", "--include=" + glob, "-e", text, dir}
	}
}

// runSearch streams matching paths to emit in batches; returns when tool exits or ctx ends.
func runSearch(ctx context.Context, args []string, emit func([]string)) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // no shell: args can't inject
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(out)
	var batch []string
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			batch = append(batch, l)
		}
		if len(batch) >= 50 {
			emit(batch)
			batch = nil
		}
	}
	if len(batch) > 0 {
		emit(batch)
	}
	cmd.Wait() // exit 1 = "no match" for grep/findstr; stderr (permission noise) dropped
	return nil
}

// finder is the Find Files dialog state (exposed for tests).
type finder struct {
	text, glob    *escEntry
	mode          *dialogSelect
	caseSensitive *dialogCheck
	win           fyne.Window
	list          *dialogList
	status        *widget.Label
	activity      *widget.ProgressBarInfinite
	results       []string
	done          bool
	cancel        context.CancelFunc
	ctx           context.Context
	workerDone    chan struct{}
}

func nameMatcher(pattern, mode string, sensitive bool) (func(string) bool, error) {
	if mode == "Wildcard" && pattern == "" {
		pattern = "*"
	}
	if mode == "Regular expression" {
		if !sensitive {
			pattern = "(?i:" + pattern + ")"
		}
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return r.MatchString, nil
	}
	if !sensitive {
		pattern = strings.ToLower(pattern)
	}
	if mode == "Wildcard" {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return nil, err
		}
	}
	return func(name string) bool {
		if !sensitive {
			name = strings.ToLower(name)
		}
		if mode == "Wildcard" {
			ok, _ := filepath.Match(pattern, name)
			return ok
		}
		return strings.Contains(name, pattern)
	}, nil
}

func (u *ui) search() *finder {
	if u.passwordWindow != nil {
		return nil
	}
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf("Find Files is not supported inside archives"))
		return nil
	}
	_, rgErr := exec.LookPath("rg")
	f := &finder{text: newEscEntry(), glob: newEscEntry(), status: widget.NewLabel(""), cancel: func() {}}
	f.activity = widget.NewProgressBarInfinite()
	f.activity.MinSize() // Fyne starts animation when first creating the renderer; initialize before stopping.
	f.activity.Stop()
	f.activity.Hide()
	f.text.SetPlaceHolder("Text in file (empty = names only)")
	f.glob.SetPlaceHolder("File name, e.g. readme")
	w := u.dialogWindow("Find Files")
	f.mode = &dialogSelect{close: w.Close}
	f.mode.Options, f.mode.Selected = []string{"Partial match", "Regular expression", "Wildcard"}, "Partial match"
	f.mode.ExtendBaseWidget(f.mode)
	f.caseSensitive = &dialogCheck{close: w.Close}
	f.caseSensitive.Text = "Case sensitive (file names)"
	f.caseSensitive.ExtendBaseWidget(f.caseSensitive)
	selected := -1
	goTo := windowButton(w, "Go to", func() {
		if selected < 0 || selected >= len(f.results) {
			return
		}
		r := f.results[selected]
		if err := p.cd(filepath.Dir(r), filepath.Base(r)); err != nil {
			f.status.SetText(err.Error())
			return
		}
		w.Close()
		if strings.HasPrefix(filepath.Base(r), ".") && !u.showHidden {
			u.toggleHidden()
			p.cd(filepath.Dir(r), filepath.Base(r))
		}
		u.activate(p)
	})
	goTo.Disable()
	f.list = &dialogList{close: w.Close}
	f.list.Length = func() int { return len(f.results) }
	f.list.CreateItem = func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	f.list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(f.results[i]) }
	f.list.ExtendBaseWidget(f.list)
	f.list.activate = goTo.OnTapped
	start := func(string) {
		if f.ctx != nil && !f.done {
			return
		}
		match, err := nameMatcher(f.glob.Text, f.mode.Selected, f.caseSensitive.Checked)
		if err != nil {
			f.status.SetText("Invalid pattern: " + err.Error())
			return
		}
		f.cancel()
		ctx, cancel := context.WithCancel(context.Background())
		f.ctx = ctx
		f.cancel, f.results, f.done = cancel, nil, false
		selected = -1
		goTo.Disable()
		f.list.UnselectAll()
		f.list.Refresh()
		args := searchCmd(runtime.GOOS, rgErr == nil, p.path, f.text.Text, "")
		content := f.text.Text != ""
		root := p.path
		finished := make(chan struct{})
		f.workerDone = finished
		f.status.SetText("Searching…")
		// ponytail: recursive search has no known total; avoid a second traversal just for percentages.
		f.activity.Show()
		f.activity.Start()
		go func() {
			defer close(finished)
			var results []string
			var err error
			if content {
				err = runSearch(ctx, args, func(batch []string) {
					for _, path := range batch {
						if match(filepath.Base(path)) {
							results = append(results, path)
						}
					}
				})
			} else {
				err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					if err != nil {
						return err
					}
					if path != root && match(d.Name()) {
						results = append(results, path)
					}
					return nil
				})
			}
			post(func() {
				if ctx.Err() != nil {
					return
				}
				f.results = results
				f.list.Refresh()
				f.done = true
				f.activity.Stop()
				f.activity.Hide()
				f.status.SetText(fmt.Sprintf("%d found", len(f.results)))
				if err != nil {
					f.status.SetText(fmt.Sprintf("%d found; search incomplete: %v", len(f.results), err))
				}
			})
		}()
	}
	f.text.OnSubmitted, f.glob.OnSubmitted = start, start
	f.list.OnSelected = func(i widget.ListItemID) {
		selected = i
		goTo.Enable()
	}
	form := container.New(layout.NewCustomPaddedVBoxLayout(12), widget.NewLabel("In: "+p.path),
		widget.NewForm(widget.NewFormItem("File name", f.glob), widget.NewFormItem("Match", f.mode), widget.NewFormItem("Containing text", f.text)), f.caseSensitive,
		dialogButtons(w, 0, windowButton(w, "Search", func() { start("") })),
		container.NewVBox(f.status, container.New(operationTrackLayout{}, f.activity)))
	f.win = w
	w.SetContent(u.dialogContent(container.NewBorder(form, container.NewCenter(container.NewHBox(goTo, windowButton(w, "Cancel", w.Close))), nil, nil, f.list)))
	unblock := u.blockParent(w)
	w.SetOnClosed(func() { f.cancel(); f.activity.Stop(); f.activity.Hide(); unblock() })
	esc := escapeWindow(w)
	f.text.esc, f.glob.esc = esc, esc // focused entry swallows keys otherwise
	w.Resize(fyne.NewSize(700, 500))
	w.CenterOnScreen()
	w.Show()
	w.Canvas().Focus(f.text)
	return f
}

// escEntry: Entry that lets Esc close its window (Fyne Entry ignores Esc, and a focused
// widget's keys never reach the canvas handler).
type escEntry struct {
	widget.Entry
	esc func(*fyne.KeyEvent) bool
}

func newEscEntry() *escEntry {
	e := &escEntry{}
	e.ExtendBaseWidget(e)
	return e
}

func (e *escEntry) TypedKey(k *fyne.KeyEvent) {
	if e.esc == nil || !e.esc(k) {
		e.Entry.TypedKey(k)
	}
}
