package commander

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestViewButtonDestination(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, root, root)
	check := func(brief bool) {
		t.Helper()
		icon := theme.GridIcon()
		if brief {
			icon = theme.ListIcon()
		}
		if u.left.brief != brief || u.left.viewButton.Text != "" || u.left.viewButton.Icon.Name() != icon.Name() {
			t.Fatalf("wrong destination button for brief=%v", brief)
		}
	}
	check(false)
	test.Tap(u.left.viewButton)
	check(true)
	u = newTrackedUI(t, a)
	check(true)
	test.Tap(u.left.viewButton)
	check(false)
	// Menu/shortcut and density changes share setBrief.
	u.left.setBrief(true)
	check(true)
	u.left.setDensity()
	check(true)
	if u.right.brief || u.right.viewButton.Text != "" || u.right.viewButton.Icon.Name() != theme.GridIcon().Name() {
		t.Fatal("toggle changed other pane")
	}
}
