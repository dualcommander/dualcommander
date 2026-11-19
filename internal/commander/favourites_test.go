package commander

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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
	tapDialogButton(t, u, "1")
	if u.right.path != u.left.path || u.active != u.right || u.right.favourite.Text != "★" {
		t.Fatal("favourite not opened in requesting pane")
	}
	u.save()
	u = newTrackedUI(t, a)
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
	tapDialogButton(t, u, "1")
	if u.right.path != before || !hasWindow(u.app, "Error") {
		t.Fatal("missing favourite must show error without navigation")
	}
	tapDialogButton(t, u, "OK")
}

func TestStarredKeyboard(t *testing.T) {
	root := fixture(t)
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	a := test.NewTempApp(t)
	u := newTestUI(t, a, src, dst)
	var star, starred *fyne.MenuItem
	for _, menu := range u.win.MainMenu().Items {
		for _, item := range menu.Items {
			switch item.Label {
			case "Star Folder":
				star = item
			case "Starred Folders…":
				starred = item
			case "Starred":
				t.Fatal("Starred submenu remains")
			}
		}
	}
	shortcut := func(item *fyne.MenuItem) *desktop.CustomShortcut {
		s, _ := item.Shortcut.(*desktop.CustomShortcut)
		return s
	}
	mod := fyne.KeyModifierShortcutDefault
	if star == nil || starred == nil || shortcut(star) == nil || *shortcut(star) != (desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: mod}) ||
		shortcut(starred) == nil || *shortcut(starred) != (desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: mod | fyne.KeyModifierShift}) {
		t.Fatal("Go menu missing Star Folder (Mod+D) or Starred Folders… (Mod+Shift+D)")
	}
	star.Action()
	if !slices.Equal(a.Preferences().StringList(prefFavourites), []string{src}) || !star.Checked || u.left.favourite.Text != "★" {
		t.Fatalf("Mod+D did not star active folder: %v %v %q path=%q", a.Preferences().StringList(prefFavourites), star.Checked, u.left.favourite.Text, u.active.path)
	}
	u.activate(u.right)
	if star.Checked {
		t.Fatal("Star Folder checked for unstarred folder")
	}
	star.Action()
	if !slices.Equal(a.Preferences().StringList(prefFavourites), []string{src, dst}) || !star.Checked {
		t.Fatal("Mod+D did not star second folder")
	}
	open := func() fyne.Focusable {
		t.Helper()
		starred.Action()
		top := u.win.Canvas().Overlays().Top()
		if top == nil {
			t.Fatal("starred folders not shown")
		}
		assertUIText(t, top, "1", src, "2", dst)
		focused := u.win.Canvas().Focused()
		if focused == nil || focused.(*favouriteButton).Text != "1" {
			t.Fatal("first starred folder not focused")
		}
		return focused
	}
	open().TypedRune('1')
	if u.right.path != src || u.left.path != src || u.active != u.right || u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("digit did not open starred folder in active pane")
	}
	open().TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if u.right.path != src || u.win.Canvas().Overlays().Top() != nil || u.win.Canvas().Focused() != u.right.focusable() {
		t.Fatal("Escape did not close starred folders and refocus pane")
	}
	focused := open()
	focused.TypedRune('9')
	focused.TypedRune('x')
	if u.right.path != src || u.win.Canvas().Overlays().Top() == nil {
		t.Fatal("unassigned key changed state")
	}
	focused.TypedRune('2')
	if u.right.path != dst {
		t.Fatal("digit 2 did not open second folder")
	}
	star.Action()
	if star.Checked || !slices.Equal(a.Preferences().StringList(prefFavourites), []string{src}) {
		t.Fatal("Mod+D did not unstar")
	}
	u.passwordWindow = u.win
	star.Action()
	u.passwordWindow = nil
	if len(a.Preferences().StringList(prefFavourites)) != 1 {
		t.Fatal("Mod+D ignored modal blocker")
	}
}

func TestStarFolderDisabledInArchive(t *testing.T) {
	root := fixture(t)
	archiveFixture(t, root)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	var star *fyne.MenuItem
	for _, menu := range u.win.MainMenu().Items {
		for _, item := range menu.Items {
			if item.Label == "Star Folder" {
				star = item
			}
		}
	}
	cursorTo(t, u.left, "browse.zip")
	press(u, fyne.KeyReturn)
	waitJobs(t, u)
	u.activate(u.left)
	if !star.Disabled {
		t.Fatal("Star Folder enabled inside archive")
	}
	star.Action()
	if len(u.app.Preferences().StringList(prefFavourites)) != 0 {
		t.Fatal("archive path starred")
	}
}

func TestDiskSpaceText(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	p := u.left
	waitUntil(t, func() bool { return p.space.Text != "Disk usage unavailable" })
	p.setSpace(256*1024, 1024*1024, nil)
	if p.space.Text != "768.0 KB free / 1.0 MB total" {
		t.Fatal("wrong free/total usage", p.space.Text)
	}
	u.app.Preferences().SetBool("showFreeSpace", false)
	p.setSpace(256*1024, 1024*1024, nil)
	if p.space.Text != "256.0 KB used / 1.0 MB total" {
		t.Fatal("wrong used/total usage", p.space.Text)
	}
	for _, used := range []uint64{0, 1024 * 1024} {
		u.app.Preferences().SetBool("showFreeSpace", true)
		p.setSpace(used, 1024*1024, nil)
		if want := fmt.Sprintf("%s free / 1.0 MB total", humanSize(int64(1024*1024-used))); p.space.Text != want {
			t.Fatalf("boundary usage: got %q, want %q", p.space.Text, want)
		}
	}
	p.setSpace(2, 1, nil)
	if p.space.Text != "Disk usage unavailable" {
		t.Fatal("invalid usage presented as free space")
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
		assertUIText(t, u.win.Content(), pathLabel, "★", "768.0 GB free / 1.0 TB total")
		assertPaneRows(t, u.left)
		test.Tap(u.left.favourites)
		assertUIText(t, u.win.Canvas().Overlays().Top(), "1", src, "2", dst, "★")
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
