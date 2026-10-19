package commander

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	text, glob *escEntry
	win        fyne.Window
	list       *dialogList
	status     *widget.Label
	results    []string
	done       bool
	cancel     context.CancelFunc
}

func (u *ui) search() *finder {
	p := u.active
	if p.inArchive() {
		u.showErr(fmt.Errorf("Find Files is not supported inside archives"))
		return nil
	}
	_, rgErr := exec.LookPath("rg")
	f := &finder{text: newEscEntry(), glob: newEscEntry(), status: widget.NewLabel(""), cancel: func() {}}
	f.text.SetPlaceHolder("Text in file (empty = names only)")
	f.glob.SetPlaceHolder("File name pattern, e.g. *.go")
	w := u.dialogWindow("Find Files")
	f.list = &dialogList{close: w.Close}
	f.list.Length = func() int { return len(f.results) }
	f.list.CreateItem = func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	f.list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(f.results[i]) }
	f.list.ExtendBaseWidget(f.list)
	start := func(string) {
		f.cancel()
		ctx, cancel := context.WithCancel(context.Background())
		f.cancel, f.results, f.done = cancel, nil, false
		f.list.Refresh()
		args := searchCmd(runtime.GOOS, rgErr == nil, p.path, f.text.Text, f.glob.Text)
		f.status.SetText("Searching with " + args[0] + "…")
		go func() {
			err := runSearch(ctx, args, func(b []string) {
				post(func() { f.results = append(f.results, b...); f.list.Refresh() })
			})
			post(func() {
				f.done = true
				f.status.SetText(fmt.Sprintf("%d found (%s)", len(f.results), args[0]))
				u.showErr(err)
			})
		}()
	}
	f.text.OnSubmitted, f.glob.OnSubmitted = start, start
	f.list.OnSelected = func(i widget.ListItemID) {
		r := f.results[i]
		if !filepath.IsAbs(r) {
			r = filepath.Join(p.path, r)
		}
		w.Close()
		p.cd(filepath.Dir(r), filepath.Base(r))
		u.activate(p)
	}
	form := container.NewVBox(widget.NewLabel("In: "+p.path), f.text, f.glob,
		windowButton(w, "Search", func() { start("") }), f.status)
	// Own OS window: native title bar with close button, movable, doesn't block panes.
	f.win = w
	w.SetContent(container.NewBorder(form, nil, nil, nil, f.list))
	w.SetOnClosed(func() { f.cancel(); u.win.RequestFocus(); u.activate(u.active) })
	esc := escapeWindow(w)
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { esc(e) }) // nothing focused
	f.text.esc, f.glob.esc = esc, esc                           // focused entry swallows keys otherwise
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
