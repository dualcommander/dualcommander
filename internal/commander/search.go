package commander

import (
	"bufio"
	"context"
	"fmt"
	"fyne.io/fyne/v2/lang"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
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
	text, glob, dir  *escEntry
	mode             string
	caseSensitive    *searchOption
	regexp, globMode *searchOption
	win              fyne.Window
	list             *dialogList
	status           *widget.Label
	activity         *widget.ProgressBarInfinite
	results          []string
	done             bool
	cancel           context.CancelFunc
	ctx              context.Context
	workerDone       chan struct{}
}

// ponytail: Fyne has no tooltip widget; in-content hints never intercept pointer events.
type searchOption struct {
	dialogButton
	Checked bool
	hint    *fyne.Container
	label   string
}

func (b *searchOption) AccessibilityLabel() string { return b.label }

func (b *searchOption) SetChecked(checked bool) {
	b.Checked = checked
	b.Importance = widget.LowImportance
	if checked {
		b.Importance = widget.HighImportance
	}
	b.Refresh()
}

func (b *searchOption) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e)
	b.hint.Show()
	b.hint.Refresh()
}

func (b *searchOption) MouseOut() { b.Button.MouseOut(); b.hint.Hide() }

type searchHintLayout struct{ anchor *searchOption }

func (l searchHintLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }
func (l searchHintLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(l.anchor)
	for _, o := range objects {
		s := o.MinSize()
		o.Resize(s)
		o.Move(fyne.NewPos(max(0, min(pos.X, size.Width-s.Width)), pos.Y+l.anchor.Size().Height+4))
	}
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
		u.showErr(fmt.Errorf(lang.L("Find Files is not supported inside archives")))
		return nil
	}
	_, rgErr := exec.LookPath("rg")
	f := &finder{text: newEscEntry(), glob: newEscEntry(), dir: newEscEntry(), status: widget.NewLabel(""), cancel: func() {}}
	f.dir.SetText(p.path)
	f.activity = widget.NewProgressBarInfinite()
	f.activity.MinSize() // Fyne starts animation when first creating the renderer; initialize before stopping.
	f.activity.Stop()
	f.activity.Hide()
	f.text.SetPlaceHolder(lang.L("Text in file (empty = names only)"))
	f.glob.SetPlaceHolder(lang.L("File name, e.g. readme"))
	w := u.dialogWindow(lang.L("Find Files"))
	f.mode = "Partial match"
	option := func(text, label, hint string) *searchOption {
		b := &searchOption{label: label}
		b.Text, b.close, b.Importance = text, w.Close, widget.LowImportance
		b.ExtendBaseWidget(b)
		b.hint = container.New(searchHintLayout{b}, widget.NewCard("", "", widget.NewLabel(hint)))
		b.hint.Hide()
		b.OnTapped = func() { b.hint.Hide(); b.SetChecked(!b.Checked) }
		return b
	}
	f.caseSensitive = option("Aa", lang.L("Match case"), lang.L("Match case: distinguish uppercase and lowercase file names."))
	f.regexp = option(".*", lang.L("Regular expression"), lang.L("Regular expression: match file names using Go regexp syntax."))
	f.globMode = option("*", lang.L("Wildcard"), lang.L("Wildcard: match whole file names with * and ?."))
	setMode := func(b, other *searchOption, mode string) {
		b.hint.Hide()
		b.SetChecked(!b.Checked)
		other.SetChecked(false)
		f.mode = "Partial match"
		if b.Checked {
			f.mode = mode
		}
	}
	f.regexp.OnTapped = func() { setMode(f.regexp, f.globMode, "Regular expression") }
	f.globMode.OnTapped = func() { setMode(f.globMode, f.regexp, "Wildcard") }
	selected := -1
	goTo := windowButton(w, lang.L("Go to"), func() {
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
	resultsTheme := searchResultsTheme{dialogTheme{u.app.Settings().Theme()}}
	f.list = &dialogList{close: w.Close}
	f.list.Length = func() int { return len(f.results) }
	f.list.CreateItem = func() fyne.CanvasObject {
		// A blank label has no text row; selection refresh would reset cached row height.
		l := widget.NewLabel(" ")
		l.Truncation = fyne.TextTruncateEllipsis
		// Virtualized rows are created outside the dialog's rendering theme scope.
		return container.NewThemeOverride(l, resultsTheme)
	}
	f.list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) {
		o.(*container.ThemeOverride).Content.(*widget.Label).SetText(f.results[i])
	}
	f.list.ExtendBaseWidget(f.list)
	f.list.HideSeparators = true
	f.list.activate = goTo.OnTapped
	start := func(string) {
		if f.ctx != nil && !f.done {
			return
		}
		match, err := nameMatcher(f.glob.Text, f.mode, f.caseSensitive.Checked)
		if err != nil {
			f.status.SetText(lang.L("Invalid pattern: ") + err.Error())
			return
		}
		root := f.dir.Text
		if root == "" {
			f.status.SetText(lang.L("Enter a search folder"))
			return
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(p.path, root)
		}
		root = filepath.Clean(root)
		f.cancel()
		ctx, cancel := context.WithCancel(context.Background())
		f.ctx = ctx
		f.cancel, f.results, f.done = cancel, nil, false
		selected = -1
		goTo.Disable()
		f.list.UnselectAll()
		f.list.Refresh()
		args := searchCmd(runtime.GOOS, rgErr == nil, root, f.text.Text, "")
		content := f.text.Text != ""
		finished := make(chan struct{})
		f.workerDone = finished
		f.status.SetText(lang.L("Searching…"))
		// ponytail: recursive search has no known total; avoid a second traversal just for percentages.
		f.activity.Show()
		f.activity.Start()
		go func() {
			defer close(finished)
			var results []string
			info, err := os.Stat(root)
			if err == nil && !info.IsDir() {
				err = fmt.Errorf(lang.L("not a folder: %s"), root)
			}
			if err == nil && content {
				err = runSearch(ctx, args, func(batch []string) {
					for _, path := range batch {
						if match(filepath.Base(path)) {
							results = append(results, path)
						}
					}
				})
			} else if err == nil {
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
				f.status.SetText(fmt.Sprintf(lang.L("%d found"), len(f.results)))
				if err != nil {
					f.status.SetText(fmt.Sprintf(lang.L("%d found; search incomplete: %v"), len(f.results), err))
				}
			})
		}()
	}
	f.text.OnSubmitted, f.glob.OnSubmitted = start, start
	f.dir.OnSubmitted = start
	f.list.OnSelected = func(i widget.ListItemID) {
		selected = i
		goTo.Enable()
	}
	nameRow := container.NewBorder(nil, nil, nil, container.NewHBox(f.caseSensitive, f.regexp, f.globMode), f.glob)
	form := widget.NewForm(widget.NewFormItem(lang.L("In:"), f.dir), widget.NewFormItem(lang.L("File name"), nameRow), widget.NewFormItem(lang.L("Containing text"), f.text))
	search := windowButton(w, lang.L("Search"), func() { start("") })
	buttons := dialogButtons(w, 0, search, goTo, windowButton(w, lang.L("Cancel"), w.Close))
	footer := container.NewVBox(container.NewBorder(nil, nil, nil, buttons, f.status), container.New(operationTrackLayout{}, f.activity))
	f.win = w
	// Keep result spacing compact without changing shared dialog or pane density.
	results := container.NewThemeOverride(f.list, resultsTheme)
	w.SetContent(container.NewStack(u.dialogContent(container.NewBorder(form, footer, nil, nil, results)),
		f.caseSensitive.hint, f.regexp.hint, f.globMode.hint))
	unblock := u.blockParent(w)
	w.SetOnClosed(func() { f.cancel(); f.activity.Stop(); f.activity.Hide(); unblock() })
	esc := escapeWindow(w)
	f.text.esc, f.glob.esc = esc, esc // focused entry swallows keys otherwise
	f.dir.esc = esc
	w.Resize(fyne.NewSize(700, 500))
	w.CenterOnScreen()
	w.Show()
	w.Canvas().Focus(f.text)
	return f
}

type searchResultsTheme struct{ fyne.Theme }

func (t searchResultsTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding || name == theme.SizeNameInnerPadding {
		return 4
	}
	return t.Theme.Size(name)
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
