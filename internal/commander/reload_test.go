package commander

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

type refreshTrackingCanvas struct {
	fyne.Canvas
	refreshed []fyne.CanvasObject
}

func (c *refreshTrackingCanvas) Refresh(o fyne.CanvasObject) {
	c.refreshed = append(c.refreshed, o)
	c.Canvas.Refresh(o)
}

type refreshTrackingWindow struct {
	fyne.Window
	canvas *refreshTrackingCanvas
}

func (w *refreshTrackingWindow) Canvas() fyne.Canvas { return w.canvas }

func TestForegroundRepaintsWithoutNavigation(t *testing.T) {
	for _, brief := range []bool{false, true} {
		t.Run(fmt.Sprint("brief=", brief), func(t *testing.T) {
			root := fixture(t)
			a := test.NewTempApp(t)
			u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.left.setBrief(brief)
			cursorTo(t, u.left, "readme.md")
			u.left.marked["readme.md"] = true
			generation := u.left.iconGeneration
			focus := u.win.Canvas().Focused()
			c := &refreshTrackingCanvas{Canvas: u.win.Canvas()}
			u.win = &refreshTrackingWindow{Window: u.win, canvas: c}
			foreground := a.Lifecycle().(interface{ OnEnteredForeground() func() }).OnEnteredForeground()
			if foreground == nil {
				t.Fatal("foreground return has no repaint handler")
			}
			foreground()
			if len(c.refreshed) != 0 {
				t.Fatal("foreground repaint must be queued on UI dispatcher")
			}
			waitUntil(t, func() bool { return slices.Contains(c.refreshed, u.win.Content()) })
			if u.left.current().name != "readme.md" || !u.left.marked["readme.md"] ||
				u.left.iconGeneration != generation || u.win.Canvas().Focused() != focus {
				t.Fatal("foreground repaint changed navigation, marks, listing, or focus")
			}
		})
	}
}

func TestTestUICleanupClosesEveryInstance(t *testing.T) {
	root := fixture(t)
	var instances []*ui
	var cancelled []context.Context
	t.Run("session", func(t *testing.T) {
		a := test.NewTempApp(t)
		instances = append(instances, newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst")))
		instances = append(instances, newTrackedUI(t, a)) // restart on same preferences
		for _, u := range instances {
			for _, p := range []*pane{u.left, u.right} {
				if p.watcher == nil {
					t.Fatal("watcher not started")
				}
				ctx, cancel := context.WithCancel(context.Background())
				p.sizeCancel = cancel
				cancelled = append(cancelled, ctx)
			}
		}
	})
	for _, u := range instances {
		for _, p := range []*pane{u.left, u.right} {
			if p.watcher != nil {
				t.Fatal("test cleanup leaked watcher")
			}
		}
	}
	for _, ctx := range cancelled {
		if ctx.Err() != context.Canceled {
			t.Fatal("test cleanup left size worker running")
		}
	}
	// Pending worker callbacks must not recreate closed watchers or refresh old panes.
	for {
		select {
		case f := <-uiQueue:
			f()
		default:
			for _, u := range instances {
				if u.left.watcher != nil || u.right.watcher != nil {
					t.Fatal("queued refresh revived watcher")
				}
			}
			return
		}
	}
}

func TestReloadActionActivePane(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	// No watcher: shortcut itself must reload, not a delayed filesystem event.
	u.left.watcher.Close()
	u.left.watcher = nil
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"] = true
	if err := os.WriteFile(filepath.Join(u.left.path, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	rightGeneration := u.right.iconGeneration
	for _, menu := range u.win.MainMenu().Items {
		for _, item := range menu.Items {
			if item.Label == "Refresh" {
				if item.Shortcut == nil {
					t.Fatal("refresh shortcut missing")
				}
				item.Action()
			}
		}
	}
	if !slices.Contains(names(u.left), "new.txt") || u.right.iconGeneration != rightGeneration {
		t.Fatal("Ctrl+R did not reload only active pane")
	}
	if u.left.current().name != "readme.md" || !u.left.marked["readme.md"] {
		t.Fatal("reload lost cursor or marks")
	}
}

func TestNativeWatchFileWriteAndRename(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	p := u.left
	cursorTo(t, p, "readme.md")
	p.marked["readme.md"] = true
	path := filepath.Join(p.path, "readme.md")
	if err := os.WriteFile(path, []byte("changed content"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return p.current().size == int64(len("changed content")) })
	if !p.marked["readme.md"] {
		t.Fatal("auto-reload lost mark")
	}
	if err := os.Rename(path, filepath.Join(p.path, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		return slices.Contains(names(p), "renamed.md") && !slices.Contains(names(p), "readme.md")
	})
	p.cd(filepath.Join(root, "dst"), "")
	path = filepath.Join(p.path, "created.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return slices.Contains(names(p), "created.txt") })
	cursorTo(t, p, "created.txt")
	if err := os.WriteFile(path, []byte("longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return p.current().size == 6 })
}

// Large folders: the watcher must track the directory only. Per-file kqueue watches
// cost one descriptor each and were added synchronously on every cd.
func TestWatcherTracksDirectoryOnlyInLargeFolder(t *testing.T) {
	root := fixture(t)
	big := filepath.Join(root, "big")
	if err := os.Mkdir(big, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 2000 {
		if err := os.WriteFile(filepath.Join(big, fmt.Sprintf("f%04d.txt", i)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), big)
	p := u.right
	if p.watcher == nil {
		t.Fatal("watcher not started")
	}
	if got := p.watcher.WatchList(); !slices.Equal(got, []string{p.path}) {
		t.Fatalf("watch list = %d paths, want only directory", len(got))
	}
}
