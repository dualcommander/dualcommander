package commander

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestThemeSwitchPreservesPaneState(t *testing.T) {
	root := fixture(t)
	dir := filepath.Join(root, "src")
	right := filepath.Join(dir, "photos")
	a := test.NewTempApp(t)
	u := newTestUI(t, a, dir, right)
	u.left.setBrief(true)
	u.right.setBrief(false)
	cursorTo(t, u.left, "readme.md")
	u.left.toggleMark()
	cursorTo(t, u.right, "a.png")
	leftCursor := u.left.current().name // toggleMark advances the cursor
	for _, mode := range []string{"light", "dark"} {
		u.setTheme(mode)
		if u.themeMode != mode || a.Preferences().String(prefTheme) != mode {
			t.Fatalf("theme not applied or saved: %s", mode)
		}
		if !u.left.brief || u.right.brief {
			t.Fatal("theme switch must preserve pane view modes")
		}
		if u.left.path != dir || u.right.path != right {
			t.Fatal("theme switch must preserve pane folders")
		}
		if u.left.current().name != leftCursor || !u.left.marked["readme.md"] || u.right.current().name != "a.png" {
			t.Fatal("theme switch must preserve pane cursors and marked file")
		}
	}
}
