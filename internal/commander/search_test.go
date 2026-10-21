package commander

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func TestFileNameSearchModes(t *testing.T) {
	for _, tc := range []struct {
		pattern, mode, name string
		sensitive, want     bool
	}{
		{"READ", "Partial match", "readme.md", false, true},
		{"READ", "Partial match", "readme.md", true, false},
		{"read", "Partial match", "readme.md", true, true},
		{".", "Partial match", "readme", false, false},
		{`^READ.*\.md$`, "Regular expression", "readme.md", false, true},
		{`^READ.*\.md$`, "Regular expression", "readme.md", true, false},
		{`\.md$`, "Regular expression", "readme.md.bak", false, false},
		{"*.MD", "Wildcard", "readme.md", false, true},
		{"*.MD", "Wildcard", "readme.md", true, false},
		{"", "Wildcard", "readme.md", false, true},
	} {
		match, err := nameMatcher(tc.pattern, tc.mode, tc.sensitive)
		if err != nil || match(tc.name) != tc.want {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	for _, mode := range []string{"Regular expression", "Wildcard"} {
		if _, err := nameMatcher("[", mode, false); err == nil {
			t.Fatalf("invalid %s accepted", mode)
		}
	}
}

func TestFindFileNameWorkflow(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	if u.passwordWindow != f.win || u.win.Canvas().Overlays().Top() == nil {
		t.Fatal("parent not blocked")
	}
	f.glob.SetText("READ")
	f.caseSensitive.SetChecked(true)
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if len(f.results) != 0 {
		t.Fatal("case sensitive search matched lowercase")
	}
	f.caseSensitive.SetChecked(false)
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if !slices.Equal(f.results, []string{filepath.Join(root, "src", "readme.md")}) {
		t.Fatalf("results: %v", f.results)
	}
	f.list.Select(0)
	if u.left.path != root || !hasWindow(u.app, "Find Files") {
		t.Fatal("selection navigated before confirmation")
	}
	f.mode.SetSelected("Regular expression")
	f.glob.SetText("[")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if f.status.Text == "Searching…" {
		t.Fatal("invalid regex started worker")
	}
	f.glob.SetText(`^a\.png$`)
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if !slices.Equal(f.results, []string{filepath.Join(root, "src", "photos", "a.png")}) {
		t.Fatalf("regex results: %v", f.results)
	}
	f.list.Select(0)
	tapDialogButton(t, u, "Go to")
	if u.left.path != filepath.Join(root, "src", "photos") || u.left.current().name != "a.png" || u.passwordWindow != nil || u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("navigation or parent restoration failed")
	}
}

func TestFindCancelRunning(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, closeBy := range []string{"Escape", "Cancel", "native close"} {
		f := u.search()
		f.glob.SetText("read")
		f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		switch closeBy {
		case "Escape":
			f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		case "Cancel":
			tapDialogButton(t, u, "Cancel")
		default:
			f.win.Close()
		}
		if f.ctx.Err() == nil {
			t.Fatal("worker context not cancelled")
		}
		if f.activity.Running() || f.activity.Visible() {
			t.Fatal("cancel left search progress active")
		}
		select {
		case <-f.workerDone:
		case <-time.After(5 * time.Second):
			t.Fatal("cancelled worker did not exit")
		}
		if u.left.path != root || u.passwordWindow != nil || hasWindow(u.app, "Find Files") {
			t.Fatal("cancel navigated or left parent blocked")
		}
	}
}

func TestFindDialogSpacingAndProgress(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	defer f.win.Close()
	outer := f.win.Content().(*container.ThemeOverride).Content.(*fyne.Container)
	body := outer.Objects[0].(*fyne.Container)
	laidOutObjects(f.win.Content())
	if body.Position() != fyne.NewPos(16, 16) || outer.Size().Width-body.Size().Width != 32 || outer.Size().Height-body.Size().Height != 32 {
		t.Fatal("Find Files needs shared 16px outer margins")
	}
	var track *fyne.Container
	for _, obj := range laidOutObjects(f.win.Content()) {
		if c, ok := obj.(*fyne.Container); ok {
			if _, ok := c.Layout.(operationTrackLayout); ok {
				track = c
			}
		}
	}
	if track == nil || len(track.Objects) != 1 || track.Objects[0] != f.activity {
		t.Fatal("search must reuse background-operation progress track")
	}
	if f.activity.Running() || f.activity.Visible() {
		t.Fatal("idle search progress active")
	}
	f.mode.SetSelected("Regular expression")
	f.glob.SetText("[")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if f.activity.Running() || f.activity.Visible() {
		t.Fatal("invalid pattern started progress")
	}
	for _, fail := range []bool{false, true} {
		if fail {
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
		}
		f.glob.SetText("read")
		f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		if !f.activity.Running() || !f.activity.Visible() {
			t.Fatal("running search needs indeterminate progress")
		}
		laidOutObjects(f.win.Content())
		if f.activity.Size().Height != 3 || f.activity.Size().Width != track.Size().Width {
			t.Fatal("search progress must use full-width 3px track")
		}
		waitUntil(t, func() bool { return f.done })
		if f.activity.Running() || f.activity.Visible() {
			t.Fatal("completion/error left progress active")
		}
	}
}

func TestFindHiddenResult(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, ".hidden"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	f.glob.SetText(".hidden")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if len(f.results) != 1 {
		t.Fatalf("hidden results: %v", f.results)
	}
	f.list.Select(0)
	f.list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if !u.showHidden || u.left.current().name != ".hidden" {
		t.Fatal("hidden result not revealed and selected")
	}
}
