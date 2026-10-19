package commander

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestFavouritesWorkflow(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), root)
	test.Tap(u.left.favourite)
	if u.left.favourite.Text != "★" {
		t.Fatal("folder not starred")
	}
	test.Tap(u.right.favourites)
	tapDialogButton(t, u, "src")
	if u.right.path != u.left.path || u.active != u.right || u.right.favourite.Text != "★" {
		t.Fatal("favourite not opened in requesting pane")
	}
	u.save()
	u = newUI(a)
	if u.left.favourite.Text != "★" || u.right.favourite.Text != "★" {
		t.Fatal("favourite lost after restart")
	}
	test.Tap(u.right.favourites)
	tapDialogButton(t, u, "★")
	if len(a.Preferences().StringList(prefFavourites)) != 0 || u.left.favourite.Text != "☆" || u.right.favourite.Text != "☆" {
		t.Fatal("dropdown unstar failed")
	}
	u.win.Canvas().Overlays().Top().Hide()
	u.toggleFavourite(filepath.Join(root, "missing"))
	before := u.right.path
	test.Tap(u.right.favourites)
	tapDialogButton(t, u, "missing")
	if u.right.path != before || !hasWindow(u.app, "Error") {
		t.Fatal("missing favourite must show error without navigation")
	}
	tapDialogButton(t, u, "OK")
}

func TestDiskSpaceText(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	p := u.left
	waitUntil(t, func() bool { return p.space.Text != "Disk usage unavailable" })
	p.setSpace(256*1024, 1024*1024, nil)
	if p.space.Text != "256.0 KB used / 1.0 MB total" {
		t.Fatal("wrong occupied/total usage")
	}
	p.setSpace(0, 0, fmt.Errorf("unavailable"))
	if p.space.Text != "Disk usage unavailable" {
		t.Fatal("disk error presented as valid usage")
	}
}

func TestMountSwitch(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	p.mounts = func() ([]string, error) { return []string{filepath.Join(root, "dst")}, nil }
	test.Tap(p.mountButton)
	waitUntil(t, func() bool { return !p.mountButton.Disabled() })
	tapDialogButton(t, u, filepath.Join(root, "dst"))
	if p.path != filepath.Join(root, "dst") || u.right.path != root || u.active != p {
		t.Fatal("mount did not open in requesting pane")
	}
	waitUntil(t, func() bool { return p.mountButton.Text == filepath.Join(root, "dst") })
	p.mounts = func() ([]string, error) { return nil, fmt.Errorf("mount lookup failed") }
	test.Tap(p.mountButton)
	waitUntil(t, func() bool { return !p.mountButton.Disabled() })
	tapDialogButton(t, u, "OK")
	if p.path != filepath.Join(root, "dst") || p.mountButton.Disabled() {
		t.Fatal("mount error changed pane or disabled selector")
	}
}

func TestCurrentMount(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "disk")
	mounts := []string{root, nested}
	for _, tc := range []struct{ path, want string }{
		{nested, nested},
		{filepath.Join(nested, "folder", "archive.zip", "inside"), nested},
		{filepath.Join(root, "disk-other"), root},
		{root, root},
	} {
		if err := os.MkdirAll(tc.path, 0o700); err != nil {
			t.Fatal(err)
		}
		if got := currentMount(tc.path, mounts); got != tc.want {
			t.Errorf("currentMount(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
	if got := currentMount(root, nil); got != filepath.VolumeName(root)+string(filepath.Separator) {
		t.Fatalf("missing mount table fallback: %q", got)
	}
}

func TestFavouritesRenderGolden(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	u.setTheme("light")
	waitUntil(t, func() bool { return u.left.space.Text != "Disk usage unavailable" })
	u.left.spaceGeneration++
	u.left.mountButton.SetText("/")
	u.left.setSpace(256*1024*1024*1024, 1024*1024*1024*1024, nil)
	u.left.pathBar.SetText("/Users/example/Documents")
	u.left.entries[0].attrs, u.left.entries[0].mod = "drwxr-xr-x", u.left.entries[1].mod
	u.left.entries[0].mod = time.Date(2024, 1, 2, 3, 4, 0, 0, time.Local)
	u.left.list.Refresh()
	u.app.Preferences().SetStringList(prefFavourites, []string{"/Users/example/Documents", "/Users/example/Downloads"})
	u.left.favourite.SetText("★")
	u.win.SetContent(u.left.box)
	u.win.Resize(fyne.NewSize(700, 320))
	for _, state := range []string{"false", "true", "mounts"} {
		if state == "true" {
			u.left.setBrief(true)
			test.Tap(u.left.favourites)
		}
		if state == "mounts" {
			u.win.Canvas().Overlays().Top().Hide()
			u.left.mounts = func() ([]string, error) { return []string{"/", "/Volumes/External"}, nil }
			test.Tap(u.left.mountButton)
			waitUntil(t, func() bool { return !u.left.mountButton.Disabled() })
		}
		golden := iconGolden(fmt.Sprintf("favourites_%s.png", state))
		if !assertNativeRender(t, golden, u.win.Canvas()) && os.Getenv("UPDATE_GOLDEN") != "" {
			if err := os.Rename(filepath.Join("testdata", "failed", golden), filepath.Join("testdata", golden)); err != nil {
				t.Fatal(err)
			}
		}
	}
}
