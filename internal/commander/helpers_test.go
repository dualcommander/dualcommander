package commander

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

// ponytail: golden baselines use macOS fonts/icons; add native baselines before comparing other OSes.
func assertNativeRender(t *testing.T, golden string, canvas fyne.Canvas) bool {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Log("native golden comparison requires macOS; workflow assertions still run")
		return true
	}
	return test.AssertRendersToImage(t, golden, canvas)
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
	u := newUI(a)
	u.win.Resize(fyne.NewSize(900, 500))
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

func iconGolden(name string) string {
	if icDir.Name() == "system-folder.png" {
		return strings.TrimSuffix(name, ".png") + "_darwin.png"
	}
	return name
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
