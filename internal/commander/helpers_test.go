package commander

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// laidOutObjects inspects live renderers; recreating Table renderers drops cached cells.
func laidOutObjects(o fyne.CanvasObject) []fyne.CanvasObject {
	objects := []fyne.CanvasObject{o}
	var children []fyne.CanvasObject
	switch c := o.(type) {
	case fyne.Widget:
		r := test.WidgetRenderer(c)
		r.Layout(c.Size())
		children = r.Objects()
	case *fyne.Container:
		if c.Layout != nil {
			c.Layout.Layout(c.Objects, c.Size())
		}
		children = c.Objects
	}
	for _, child := range children {
		objects = append(objects, laidOutObjects(child)...)
	}
	return objects
}

// assertUIText checks widget content without depending on native fonts or pixels.
func assertUIText(t *testing.T, content fyne.CanvasObject, want ...string) {
	t.Helper()
	var got []string
	for _, o := range laidOutObjects(content) {
		if !o.Visible() {
			continue
		}
		switch w := o.(type) {
		case *widget.Label:
			got = append(got, w.Text)
		case *widget.Button:
			got = append(got, w.Text)
		case *dialogButton:
			got = append(got, w.Text)
		case *widget.Entry:
			got = append(got, w.Text)
		}
	}
	for _, text := range want {
		if !slices.Contains(got, text) {
			t.Errorf("missing UI text %q; got %q", text, got)
		}
	}
}

func assertPaneRows(t *testing.T, p *pane) {
	t.Helper()
	count := 0
	for _, o := range laidOutObjects(p.body) {
		r, ok := o.(*row)
		if !ok || !r.Visible() || r.id < 0 {
			continue
		}
		count++
		e := p.entries[r.id]
		assertUIText(t, r, e.name)
		if !p.brief {
			size := humanSize(e.size)
			if e.dir {
				size = "<DIR>"
			}
			modified := ""
			if !e.mod.IsZero() {
				modified = e.mod.Format("2006-01-02 15:04")
			}
			assertUIText(t, r, size, e.attrs, modified)
		}
		var name *widget.Label
		var icon *widget.Icon
		for _, child := range laidOutObjects(r) {
			if l, ok := child.(*widget.Label); ok && l.Text == e.name {
				name = l
			}
			if i, ok := child.(*widget.Icon); ok {
				icon = i
			}
		}
		if icon == nil || icon.Resource == nil {
			t.Errorf("row %q missing icon", e.name)
		}
		if name != nil && (name.Importance == widget.DangerImportance) != p.marked[e.name] {
			t.Errorf("row %q mark styling differs from selection", e.name)
		}
	}
	if count == 0 {
		t.Fatal("pane has no laid-out rows")
	}
}

// fixture: tmp tree with fixed mtimes so renders are deterministic.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	stamp := time.Date(2024, 1, 2, 3, 4, 0, 0, time.Local)
	for _, d := range []string{"src", "dst", "src/docs", "src/photos"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	for name, body := range map[string]string{
		"src/readme.md": "hello", "src/song.mp3": "la", "src/app.zip": "zz", "src/.secret": "x", "src/photos/a.png": "png",
	} {
		os.WriteFile(filepath.Join(root, name), []byte(body), 0o644)
	}
	for _, p := range []string{"src/readme.md", "src/song.mp3", "src/app.zip", "src/docs", "src/photos", "src/photos/a.png"} {
		os.Chtimes(filepath.Join(root, p), stamp, stamp)
	}
	return root
}

func newTestUI(t *testing.T, a fyne.App, left, right string) *ui {
	t.Helper()
	a.Preferences().SetString(prefLeft, left)
	a.Preferences().SetString(prefRight, right)
	initIcons()
	u := newTrackedUI(t, a)
	u.win.Resize(fyne.NewSize(900, 500))
	return u
}

func newTrackedUI(t *testing.T, a fyne.App) *ui {
	t.Helper()
	u := newUI(a)
	t.Cleanup(func() {
		for _, w := range slices.Clone(a.Driver().AllWindows()) {
			w.SetCloseIntercept(nil)
			w.Close()
		}
		u.closePanes()
	})
	return u
}

func windowByTitle(t *testing.T, a fyne.App, title string) fyne.Window {
	t.Helper()
	var found fyne.Window
	for _, w := range a.Driver().AllWindows() {
		if w.Title() == title {
			if found != nil {
				t.Fatalf("duplicate window %q", title)
			}
			found = w
		}
	}
	if found == nil {
		t.Fatalf("missing window %q", title)
	}
	return found
}

// set once: background goroutines (watchers) from earlier tests read post concurrently.
// trash stub: removes but records, so tests never touch the user's real Trash.
func init() {
	post = func(f func()) { uiQueue <- f }
	trash = func(ctx context.Context, ps []string, prog fsops.Progress) error {
		trashed.Store(true)
		return fsops.Delete(ctx, ps, prog)
	}
}

var trashed atomic.Bool

// uiQueue stands in for the UI thread (test driver runs fyne.Do inline = races).
var uiQueue = make(chan func(), 4096)

// waitUntil runs queued UI updates on the test goroutine until cond holds.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for !cond() {
		select {
		case f := <-uiQueue:
			f()
		case <-deadline:
			t.Fatal("timed out waiting")
		}
	}
}

func waitJobs(t *testing.T, u *ui) {
	t.Helper()
	waitUntil(t, func() bool { return len(u.jobs.box.Objects) == 0 })
}

func names(p *pane) (out []string) {
	for _, e := range p.entries {
		out = append(out, e.name)
	}
	return
}

func press(u *ui, k fyne.KeyName) { u.key(u.active, &fyne.KeyEvent{Name: k}, 20) }

func cursorTo(t *testing.T, p *pane, name string) {
	t.Helper()
	i := slices.Index(names(p), name)
	if i < 0 {
		t.Fatalf("%q not in %v", name, names(p))
	}
	p.moveTo(i)
}

func hasWindow(a fyne.App, title string) bool {
	for _, w := range a.Driver().AllWindows() {
		if w.Title() == title {
			return true
		}
	}
	return false
}

// tapDialogButton checks native windows before popup menus.
func tapDialogButton(t *testing.T, u *ui, label string) {
	t.Helper()
	windows := u.app.Driver().AllWindows()
	for i := len(windows) - 1; i >= 0; i-- {
		if windows[i] == u.win {
			continue
		}
		for _, o := range test.LaidOutObjects(windows[i].Content()) {
			if b, ok := o.(*dialogButton); ok && b.Text == label {
				test.Tap(b)
				return
			}
		}
	}
	top := u.win.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("no dialog shown")
	}
	for _, o := range test.LaidOutObjects(top) {
		if b, ok := o.(*widget.Button); ok && b.Text == label {
			test.Tap(b)
			return
		}
	}
	t.Fatalf("button %q not found", label)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
