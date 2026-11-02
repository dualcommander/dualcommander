package commander

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNewUIInitializesIcons(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	a.Preferences().SetString(prefLeft, root)
	a.Preferences().SetString(prefRight, root)
	icDir = nil
	u := newTrackedUI(t, a)
	if icDir == nil || iconFor(entry{name: "folder", dir: true}) == nil {
		t.Fatal("window startup did not initialize folder icons")
	}
	if u.left.entryIcon(0) == nil {
		t.Fatal("window startup did not initialize parent icon")
	}
}
