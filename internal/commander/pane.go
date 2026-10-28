package commander

import (
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2/lang"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/archive"
	"github.com/dualcommander/dualcommander/pkg/fsops"
	"github.com/fsnotify/fsnotify"
)

type entry struct {
	name          string
	dir           bool
	size          int64
	mod           time.Time
	attrs         string
	icon          fyne.Resource
	iconRequested bool
	sizeRequested bool
	sizeKnown     bool
	sizeError     string
	sizeFiles     int64
}

// pane is one side of the commander: path bar + list (detailed) or grid (brief).
type pane struct {
	ui              *ui
	path            string
	entries         []entry
	cursor          int
	marked          map[string]bool
	searchPrefix    string
	brief           bool
	viewButton      *widget.Button
	pathBar         *pathEntry
	favourite       *widget.Button
	favourites      *widget.Button
	space           *widget.Label
	mountButton     *widget.Button
	mounts          func() ([]string, error)
	spaceGeneration int
	status          *widget.Label
	statusRow       *fyne.Container
	list            *keyList
	grid            *keyGrid
	body            *fyne.Container
	box             *fyne.Container
	watcher         *fsnotify.Watcher
	sortBy          string // one of sortKeys
	desc            bool
	headers         []*widget.Button
	hdr             fyne.CanvasObject
	archiveRoot     string
	archiveFS       fs.FS
	navigation      int
	iconGeneration  int
	sizeBusy        bool
	sizeCancel      context.CancelFunc
	measureSize     func(context.Context, []string, func(int64, int64)) (int64, int64, error)
}

func newPane(u *ui, path string, brief bool, sortPref string) *pane {
	p := &pane{ui: u, marked: map[string]bool{}, sortBy: "name", measureSize: fsops.SizeWithProgress}
	if by, d, _ := strings.Cut(sortPref, " "); slices.Contains(sortKeys, by) {
		p.sortBy, p.desc = by, d == "desc"
	}
	p.hdr = p.header()
	p.setSortLabels()
	p.pathBar = newPathEntry()
	p.pathBar.OnSubmitted = func(s string) { p.navigate(s, "") }
	p.status = widget.NewLabel("")
	p.status.Truncation = fyne.TextTruncateEllipsis
	p.statusRow = newRowContent(true).(*fyne.Container)
	p.statusRow.Objects[1].Hide()
	p.statusRow.Hide()
	p.list = newKeyList(p)
	p.grid = newKeyGrid(p)
	p.body = container.NewStack()
	p.viewButton = widget.NewButtonWithIcon("", theme.GridIcon(), func() { p.setBrief(!p.brief); u.save() })
	upBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), p.up)
	homeBtn := widget.NewButtonWithIcon("", theme.HomeIcon(), func() { h, _ := os.UserHomeDir(); p.navigate(h, "") })
	p.favourite = widget.NewButton("☆", func() { u.toggleFavourite(p.path) })
	p.favourites = widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), p.showFavourites)
	p.space = widget.NewLabel(lang.L("Disk usage unavailable"))
	p.space.Alignment = fyne.TextAlignTrailing
	p.space.Truncation = fyne.TextTruncateEllipsis
	p.mounts = fsops.Mounts
	p.mountButton = widget.NewButtonWithIcon("/", theme.MenuDropDownIcon(), p.showMounts)
	top := container.NewVBox(container.NewBorder(nil, nil, p.mountButton, nil, p.space), container.NewBorder(nil, nil, container.NewHBox(homeBtn, upBtn),
		container.NewGridWithRows(1, p.favourite, p.favourites, p.viewButton), p.pathBar))
	p.box = container.NewBorder(top, container.NewVBox(widget.NewSeparator(), container.NewStack(p.status, p.statusRow)), nil, nil, p.body)
	p.setBrief(brief)
	if p.cd(path, "") != nil {
		h, _ := os.UserHomeDir()
		p.cd(h, "")
	}
	// startup: cursor on first real entry, not ".."
	if len(p.entries) > 1 && p.entries[0].name == ".." {
		p.moveTo(1)
	}
	return p
}

func (p *pane) setBrief(b bool) {
	p.brief = b
	p.ui.updateViewMenu()
	// Button describes the destination, not the current view.
	p.viewButton.SetIcon(theme.GridIcon())
	if b {
		p.viewButton.SetIcon(theme.ListIcon())
	}
	var view fyne.CanvasObject = p.list
	if b {
		view = p.grid
	}
	p.body.Objects = []fyne.CanvasObject{container.NewBorder(p.hdr, nil, nil, nil, view)}
	p.body.Refresh()
	p.moveTo(p.cursor)
}

// setDensity rebuilds views: rows cache their sizes at creation.
func (p *pane) setDensity() {
	p.list, p.grid = newKeyList(p), newKeyGrid(p)
	p.setBrief(p.brief)
}

func (p *pane) focusable() fyne.Focusable {
	if p.brief {
		return p.grid
	}
	return p.list
}

// cd loads dir; selectName puts the cursor on that entry (e.g. after going up).
func (p *pane) cd(dir, selectName string) error {
	dir, _ = filepath.Abs(dir)
	root, inner := archive.Root(dir)
	var des []fs.DirEntry
	var err error
	if root != "" {
		if root != p.archiveRoot || p.archiveFS == nil {
			p.archiveFS, err = archive.Open(context.Background(), root)
			if err == nil {
				p.archiveRoot = root
			}
		}
		if err == nil {
			des, err = archive.ReadDir(p.archiveFS, inner)
		}
	} else {
		des, err = os.ReadDir(dir)
	}
	if err != nil {
		p.ui.showErr(err)
		return err
	}
	same := dir == p.path
	if !same {
		p.marked = map[string]bool{}
		p.searchPrefix = ""
	}
	es := make([]entry, 0, len(des)+1)
	if filepath.Dir(dir) != dir {
		parent := entry{name: "..", dir: true, attrs: "?"}
		if fi, err := os.Stat(filepath.Dir(dir)); err == nil {
			parent.attrs, parent.mod = fi.Mode().String(), fi.ModTime()
		}
		es = append(es, parent)
	}
	for _, d := range des {
		if !p.ui.showHidden && strings.HasPrefix(d.Name(), ".") {
			continue
		}
		e := entry{name: d.Name(), dir: d.IsDir(), attrs: "?"}
		if fi, err := d.Info(); err == nil {
			e.size, e.mod = fi.Size(), fi.ModTime()
			e.attrs = fi.Mode().String()
		}
		if d.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, d.Name())); err == nil {
				e.dir = st.IsDir()
			}
		}
		es = append(es, e)
	}
	sortEntries(es, p.sortBy, p.desc)
	if p.sizeCancel != nil {
		p.sizeCancel()
	}
	p.path, p.entries = dir, es
	p.iconGeneration++
	if !same {
		p.watch()
	}
	p.watchFiles()
	p.pathBar.SetText(dir)
	p.updateFavourite()
	p.updateSpace()
	cur := 0
	if same {
		cur = p.cursor
	}
	for i, e := range es {
		if e.name == selectName {
			cur = i
		}
	}
	p.list.Refresh()
	p.grid.Refresh()
	p.moveTo(cur)
	// Fyne's table refresh updates cells without invalidating the canvas.
	canvas.Refresh(p.body)
	p.ui.save()
	return nil
}

func (p *pane) refresh() { p.cd(p.path, p.current().name) }

// Sort keys shared by both views.
var sortKeys = []string{"name", "size", "date"}

// sortEntries: ".." first, dirs before files, then by key.
// Dirs have no size, so "size" orders them by name.
func sortEntries(es []entry, by string, desc bool) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if a.name == ".." || b.name == ".." {
			return a.name == ".."
		}
		if a.dir != b.dir {
			return a.dir
		}
		if desc {
			a, b = b, a
		}
		switch {
		case by == "date" && !a.mod.Equal(b.mod):
			return a.mod.Before(b.mod)
		case by == "size" && !a.dir && a.size != b.size:
			return a.size < b.size
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
}

// setSort: clicking the active column flips direction, another column sorts ascending.
func (p *pane) setSort(by string) {
	p.desc = by == p.sortBy && !p.desc
	p.sortBy = by
	p.setSortLabels()
	p.refresh()      // cd keeps cursor on same entry; saves
	p.ui.activate(p) // header button click stole focus; give it back to the list
}

func (p *pane) setSortLabels() {
	for i, b := range p.headers {
		b.SetText(lang.L(strings.ToUpper(sortKeys[i][:1])+sortKeys[i][1:]) + arrow(sortKeys[i] == p.sortBy, p.desc))
	}
}

func (p *pane) sortPref() string {
	if p.desc {
		return p.sortBy + " desc"
	}
	return p.sortBy
}

func arrow(on, desc bool) string {
	switch {
	case !on:
		return ""
	case desc:
		return " ▾"
	}
	return " ▴"
}

// header: clickable Name/Size/Date, laid out like newRowContent's detailed row.
func (p *pane) header() fyne.CanvasObject {
	p.headers = make([]*widget.Button, len(sortKeys))
	for i, k := range sortKeys {
		p.headers[i] = widget.NewButton(lang.L(strings.ToUpper(k[:1])+k[1:])+arrow(k == p.sortBy, p.desc), func() { p.setSort(k) })
		p.headers[i].Importance = widget.LowImportance
	}
	p.headers[0].Alignment = widget.ButtonAlignLeading
	p.headers[1].Alignment = widget.ButtonAlignTrailing
	fixed := func(o fyne.CanvasObject, s string) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(widget.NewLabel(s).MinSize().Width, p.headers[0].MinSize().Height), o)
	}
	right := container.NewHBox(fixed(p.headers[1], "0000.0 MB"), fixed(p.headers[2], "0000-00-00 00:00"), fixed(widget.NewLabel(lang.L("Attributes")), "drwxrwxrwx"))
	return container.NewBorder(nil, nil, nil, right, p.headers[0])
}

// watch refreshes the pane when its dir changes on disk (debounced; bursts = one refresh).
func (p *pane) watch() {
	if p.watcher != nil {
		p.watcher.Close()
		p.watcher = nil
	}
	if p.inArchive() {
		return
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	if w.Add(p.path) != nil {
		w.Close()
		return
	}
	p.watcher = w
	go func() {
		t := time.NewTimer(time.Hour)
		t.Stop()
		defer t.Stop()
		for {
			select {
			case _, ok := <-w.Events:
				if !ok {
					return
				}
				t.Reset(200 * time.Millisecond)
			case <-t.C:
				post(func() {
					if p.watcher == w {
						p.refresh()
					}
				})
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

// kqueue reports directory membership changes, but needs file watches for writes.
func (p *pane) watchFiles() {
	if runtime.GOOS != "darwin" || p.watcher == nil {
		return
	}
	wanted := map[string]bool{p.path: true}
	for _, e := range p.entries {
		if !e.dir && e.name != ".." {
			wanted[filepath.Join(p.path, e.name)] = true
		}
	}
	for _, path := range p.watcher.WatchList() {
		if wanted[path] {
			delete(wanted, path)
		} else {
			p.watcher.Remove(path)
		}
	}
	for path := range wanted {
		p.watcher.Add(path)
	}
}

func (p *pane) up() {
	if parent := filepath.Dir(p.path); parent != p.path {
		p.navigate(parent, filepath.Base(p.path))
	}
}

func (p *pane) current() entry {
	if p.cursor >= 0 && p.cursor < len(p.entries) {
		return p.entries[p.cursor]
	}
	return entry{}
}

func (p *pane) moveTo(i int) {
	if len(p.entries) > 0 {
		p.cursor = max(0, min(i, len(p.entries)-1))
		p.showCursor()
	}
	p.updateStatus()
}

// showCursor: only active pane draws selection; inactive keeps cursor index silently.
func (p *pane) showCursor() {
	if p.ui.active != p {
		p.list.UnselectAll()
		p.grid.UnselectAll()
		return
	}
	if p.brief {
		p.grid.Select(p.cursor)
		// Table.Select already moves its highlight and scrolls into view.
	} else {
		p.list.Select(p.cursor)
		p.list.Highlight(p.cursor)
	}
}

func (p *pane) open() {
	e := p.current()
	switch {
	case e.name == "..":
		p.up()
	case e.dir:
		p.navigate(filepath.Join(p.path, e.name), "")
	case e.name != "":
		name := filepath.Join(p.path, e.name)
		if p.inArchive() {
			_, inner := archive.Root(name)
			f := p.archiveFS
			root := p.archiveRoot
			p.ui.jobs.run(lang.L("Open archive member"), nil, func(ctx context.Context, _ fsops.Progress) error {
				var tmp string
				err := p.ui.archiveRetry(ctx, root, func(password string) error {
					if password != "" {
						var err error
						f, err = archive.Open(ctx, root, password)
						if err != nil {
							return err
						}
					}
					var err error
					tmp, err = archive.Materialize(ctx, f, inner)
					return err
				})
				if err == nil {
					post(func() {
						if p.archiveRoot == root {
							p.archiveFS = f
						}
						p.ui.archiveTemps = append(p.ui.archiveTemps, filepath.Dir(tmp))
						p.openFile(tmp)
					})
				}
				return err
			})
			return
		}
		if archive.Supported(name) && p.ui.app.Preferences().BoolWithFallback(prefBrowseArchives, true) {
			p.navigate(name, "")
			return
		}
		p.openFile(name)
	}
}

func (p *pane) openFile(name string) {
	if u, err := url.Parse(storage.NewFileURI(name).String()); err == nil {
		p.ui.showErr(p.ui.app.OpenURL(u))
	}
}

func (p *pane) inArchive() bool {
	root, _ := archive.Root(p.path)
	return root != ""
}

// navigate indexes archives off the UI thread; stale completions cannot change folders.
func (p *pane) navigate(dir, selected string) {
	dir, _ = filepath.Abs(dir)
	p.navigation++
	version := p.navigation
	root, inner := archive.Root(dir)
	if root == "" {
		p.cd(dir, selected)
		return
	}
	if root == p.archiveRoot && p.archiveFS != nil {
		p.cd(dir, selected)
		return
	}
	p.ui.jobs.run(lang.L("Browse archive"), nil, func(ctx context.Context, _ fsops.Progress) error {
		var f fs.FS
		err := p.ui.archiveRetry(ctx, root, func(password string) error {
			var err error
			f, err = archive.Open(ctx, root, password)
			return err
		})
		if err == nil {
			_, err = archive.ReadDir(f, inner)
		}
		if err == nil {
			post(func() {
				if p.navigation == version {
					p.archiveRoot, p.archiveFS = root, f
					p.cd(dir, selected)
				}
			})
		}
		return err
	})
}

func (u *ui) archiveRetry(ctx context.Context, name string, run func(string) error) error {
	password := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := run(password)
		if !errors.Is(err, archive.ErrPassword) {
			return err
		}
		var promptErr error
		password, promptErr = u.archivePassword(ctx, name, password != "")
		if promptErr != nil {
			return promptErr
		}
	}
}

// flip toggles mark of entry i ("..": never marked).
func (p *pane) flip(i int) {
	if i < 0 || i >= len(p.entries) || p.entries[i].name == ".." {
		return
	}
	n := p.entries[i].name
	if p.marked[n] {
		delete(p.marked, n)
	} else {
		p.marked[n] = true
	}
	p.list.RefreshItem(i)
	p.grid.RefreshItem(i)
}

func (p *pane) toggleMark() {
	p.flip(p.cursor)
	p.moveTo(p.cursor + 1)
}

// click: plain = move cursor, Mod = toggle mark, Shift = mark range from cursor.
func (p *pane) click(i int, m fyne.KeyModifier) {
	switch {
	case m&fyne.KeyModifierShift != 0:
		for k := min(p.cursor, i); k <= max(p.cursor, i) && k < len(p.entries); k++ {
			if n := p.entries[k].name; n != ".." {
				p.marked[n] = true
			}
		}
		p.list.Refresh()
		p.grid.Refresh()
	case m&fyne.KeyModifierShortcutDefault != 0:
		p.flip(i)
	}
	p.moveTo(i)
}

func (p *pane) markAll(on bool) {
	p.marked = map[string]bool{}
	for _, e := range p.entries {
		if on && e.name != ".." {
			p.marked[e.name] = true
		}
	}
	p.list.Refresh()
	p.grid.Refresh()
	p.updateStatus()
}

// selection returns marked paths, or the item under cursor if nothing is marked.
func (p *pane) selection() []string {
	var out []string
	for _, e := range p.entries {
		if p.marked[e.name] {
			out = append(out, filepath.Join(p.path, e.name))
		}
	}
	if e := p.current(); len(out) == 0 && e.name != "" && e.name != ".." {
		out = []string{filepath.Join(p.path, e.name)}
	}
	return out
}

func (p *pane) updateStatus() {
	p.ui.updateCompareMenu()
	if p.ui.active == p {
		p.ui.updateFunctionButtons()
	}
	p.statusRow.Hide()
	p.status.Show()
	if p.searchPrefix != "" {
		p.status.SetText(lang.L("Search: ") + p.searchPrefix)
		return
	}
	var files, dirs, selectedFiles, selectedDirs int
	var totalSize, selectedSize int64
	var folderFiles int64
	var measured, pending bool
	sizeNotice := ""
	// Cursor-only navigation must not scan the directory on every key repeat.
	if len(p.marked) > 0 {
		for _, e := range p.entries {
			if e.name == ".." {
				continue
			}
			if e.dir {
				dirs++
				if e.sizeKnown {
					totalSize += e.size
				}
				if p.marked[e.name] {
					selectedDirs++
					if e.sizeRequested {
						measured = true
						if !e.sizeKnown && e.sizeError == "" {
							selectedSize += e.size
							folderFiles += e.sizeFiles
							pending = true
						}
					}
					if e.sizeKnown {
						selectedSize += e.size
						folderFiles += e.sizeFiles
					} else if e.sizeError != "" {
						sizeNotice = lang.L(" · Folder size unavailable: ") + e.sizeError
					} else if e.sizeRequested && sizeNotice == "" {
						sizeNotice = fmt.Sprintf(lang.L(" · Calculating folder size: %s · %s · %d files…"), e.name, humanSize(e.size), e.sizeFiles)
					}
				}
			} else {
				files++
				totalSize += e.size
				if p.marked[e.name] {
					selectedFiles++
					selectedSize += e.size
					measured = measured || e.sizeRequested
				}
			}
		}
	}
	e := p.current()
	if measured {
		p.status.SetText(fmt.Sprintf(lang.L("%s · %d files"), humanSize(selectedSize), folderFiles+int64(selectedFiles)))
		if strings.HasPrefix(sizeNotice, lang.L(" · Folder size unavailable: ")) {
			p.status.SetText(p.status.Text + sizeNotice)
		}
		if pending {
			p.status.SetText(p.status.Text + lang.L(" · Calculating…"))
		}
		return
	}
	if selectedFiles+selectedDirs > 0 {
		if folderFiles > 0 {
			sizeNotice += fmt.Sprintf(lang.L(" · Folder contents: %d files"), folderFiles)
		}
		// ponytail: totals include only folders measured with Space; refresh invalidates cached sizes.
		if sizeNotice != "" {
			sizeNotice = strings.TrimPrefix(sizeNotice, " · ") + " · "
		}
		p.status.SetText(fmt.Sprintf(lang.L("%sSelected: %s of %s · Files: %d of %d · Folders: %d of %d"),
			sizeNotice, humanSize(selectedSize), humanSize(totalSize), selectedFiles, files, selectedDirs, dirs))
		return
	}
	if e.name == "" {
		p.status.SetText("")
		return
	}
	size := humanSize(e.size)
	if e.dir && !e.sizeKnown {
		size = lang.L("<DIR>")
		if e.sizeError != "" {
			size = lang.L("Folder size unavailable: ") + e.sizeError
		} else if e.sizeRequested {
			size = fmt.Sprintf(lang.L("%s · %d files · Calculating folder size…"), humanSize(e.size), e.sizeFiles)
		}
	}
	if e.dir && e.sizeKnown {
		size += fmt.Sprintf(lang.L(" · %d files"), e.sizeFiles)
	}
	modified := ""
	if !e.mod.IsZero() {
		modified = " · " + e.mod.Format("2006-01-02 15:04")
	}
	p.status.SetText(fmt.Sprintf("%s · %s%s · %s", e.name, size, modified, e.attrs))
	p.fillRow(p.cursor, &row{content: p.statusRow})
	p.status.Hide()
	p.statusRow.Show()
}

func (p *pane) typeSearch(r rune) {
	if r <= ' ' || r == 127 {
		return
	}
	p.searchPrefix += string(r)
	p.matchSearch()
}

func (p *pane) matchSearch() {
	if p.searchPrefix != "" {
		for i, e := range p.entries {
			if e.name != ".." && strings.HasPrefix(strings.ToLower(e.name), strings.ToLower(p.searchPrefix)) {
				p.moveTo(i)
				break
			}
		}
	}
	p.updateStatus()
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	f, i := float64(n)/1024, 0
	for ; f >= 1024 && i < 5; i++ {
		f /= 1024
	}
	return fmt.Sprintf("%.1f %cB", f, "KMGTP"[i])
}

// --- row rendering ---

func newRowContent(detailed bool) fyne.CanvasObject {
	icon := widget.NewIcon(nil)
	name := widget.NewLabel("")
	name.Truncation = fyne.TextTruncateEllipsis
	// fixed: width from sample text, so columns/cells never depend on stale MinSize
	fixed := func(o *widget.Label, sample string) *fyne.Container {
		h := widget.NewLabel(sample).MinSize()
		return container.NewGridWrap(h, o)
	}
	if !detailed {
		// Minimum brief cell width (~20 chars); grid expands for longer names.
		cell := widget.NewLabel("MMMMMMMMMMMMMMMMMM").MinSize().AddWidthHeight(theme.IconInlineSize(), 0)
		return container.NewGridWrap(cell, container.NewBorder(nil, nil, icon, nil, name))
	}
	size := widget.NewLabel("")
	size.Alignment = fyne.TextAlignTrailing
	attrs := widget.NewLabel("")
	attrs.TextStyle.Monospace = true
	right := container.NewHBox(fixed(size, "0000.0 MB"), fixed(widget.NewLabel(""), "0000-00-00 00:00"), fixed(attrs, "drwxrwxrwx"))
	return container.NewBorder(nil, nil, icon, right, name)
}

func (p *pane) fillRow(i int, r *row) {
	if i >= len(p.entries) {
		return
	}
	r.id = i
	e := p.entries[i]
	c := r.content.(*fyne.Container) // Border objects: [center, left, (right)]
	if len(c.Objects) == 1 {         // brief: GridWrap around Border
		c = c.Objects[0].(*fyne.Container)
	}
	name := c.Objects[0].(*widget.Label)
	if c.Objects[1].Visible() {
		c.Objects[1].(*widget.Icon).SetResource(p.entryIcon(i))
	}
	importance := widget.MediumImportance
	if p.marked[e.name] {
		importance = widget.DangerImportance
	}
	// ponytail: Fyne SetText reshapes unchanged text; guard recycled row labels until upstream does.
	setText := func(label *widget.Label, text string) {
		if label.Text != text {
			label.SetText(text)
		}
	}
	if name.Text != e.name || name.TextStyle != (fyne.TextStyle{}) || name.Importance != importance {
		name.TextStyle = fyne.TextStyle{}
		name.Importance = importance
		name.SetText(e.name)
	}
	if len(c.Objects) < 3 {
		return
	}
	right := c.Objects[2].(*fyne.Container)
	size := right.Objects[0].(*fyne.Container).Objects[0].(*widget.Label)
	mod := right.Objects[1].(*fyne.Container).Objects[0].(*widget.Label)
	setText(right.Objects[2].(*fyne.Container).Objects[0].(*widget.Label), e.attrs)
	sizeText := humanSize(e.size)
	if e.dir {
		sizeText = lang.L("<DIR>")
	}
	setText(size, sizeText)
	modified := ""
	if !e.mod.IsZero() {
		modified = e.mod.Format("2006-01-02 15:04")
	}
	setText(mod, modified)
}
