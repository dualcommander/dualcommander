package commander

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestFavouritesUIContent(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	u.setTheme("light")
	waitUntil(t, func() bool { return u.left.space.Text != "Disk usage unavailable" })
	u.left.spaceGeneration++
	u.left.mountButton.SetText("/")
	u.left.setSpace(256*1024*1024*1024, 1024*1024*1024*1024, nil)
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	u.toggleFavourite(src)
	u.toggleFavourite(dst)
	u.win.SetContent(u.left.box)
	u.win.Resize(fyne.NewSize(700, 320))
	for _, brief := range []bool{false, true} {
		u.left.setBrief(brief)
		pathLabel := test.WidgetRenderer(u.left.pathBar).(*pathRenderer).label.Text
		if u.left.pathBar.Text != src || !strings.HasSuffix(pathLabel, string(filepath.Separator)+"src") {
			t.Fatal("favourite path lost full value or current folder label")
		}
		assertUIText(t, u.win.Content(), pathLabel, "★", "256.0 GB used / 1.0 TB total")
		assertPaneRows(t, u.left)
		test.Tap(u.left.favourites)
		assertUIText(t, u.win.Canvas().Overlays().Top(), "src", src, "dst", dst, "★")
		u.win.Canvas().Overlays().Top().Hide()
	}
	u.left.mounts = func() ([]string, error) { return []string{root, dst}, nil }
	test.Tap(u.left.mountButton)
	waitUntil(t, func() bool { return !u.left.mountButton.Disabled() })
	assertUIText(t, u.win.Canvas().Overlays().Top(), root, dst)
	tapDialogButton(t, u, dst)
	pathLabel := test.WidgetRenderer(u.left.pathBar).(*pathRenderer).label.Text
	if u.left.pathBar.Text != dst || !strings.HasSuffix(pathLabel, string(filepath.Separator)+"dst") {
		t.Fatal("mount navigation lost full path or current folder label")
	}
	assertUIText(t, u.win.Content(), pathLabel, "★")
	if u.left.path != dst || u.right.path != root {
		t.Fatal("mount selection navigated wrong pane")
	}
	u.left.mounts = func() ([]string, error) { return nil, nil }
	test.Tap(u.left.mountButton)
	waitUntil(t, func() bool { return !u.left.mountButton.Disabled() })
	assertUIText(t, u.win.Canvas().Overlays().Top(), "No mounted volumes")
	u.win.Canvas().Overlays().Top().Hide()
	u.toggleFavourite(src)
	u.toggleFavourite(dst)
	test.Tap(u.left.favourites)
	assertUIText(t, u.win.Canvas().Overlays().Top(), "No favourites. Star a folder to add it.")
}
