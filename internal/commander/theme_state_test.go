package commander

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestThemeSwitchPreservesPaneState(t *testing.T) {
	root := fixture(t)
	dir := filepath.Join(root, "src")
	u := newTestUI(t, test.NewTempApp(t), dir, dir)
	u.left.setBrief(true)
	u.right.setBrief(false)
	cursorTo(t, u.left, "readme.md")
	u.left.toggleMark()
	cursorTo(t, u.right, "app.zip")
	leftCursor := u.left.current().name
	for _, mode := range []string{"light", "dark"} {
		u.setTheme(mode)
		if !u.left.brief || u.right.brief {
			t.Fatal("theme switch must preserve pane view modes")
		}
		if u.left.current().name != leftCursor || !u.left.marked["readme.md"] || u.right.current().name != "app.zip" {
			t.Fatal("theme switch must preserve pane cursors and marked file")
		}
	}
}
