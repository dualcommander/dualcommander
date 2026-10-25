package commander

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	if u.passwordWindow.Canvas() != f.win.Canvas() || u.win.Canvas().Overlays().Top() == nil {
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
	test.Tap(f.regexp)
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
	archiveFixture(t, root)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, closeBy := range []string{"Escape", "Cancel", "native close"} {
		f := u.search()
		test.Tap(findArchiveSearchOption(t, f))
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

func findArchiveSearchOption(t *testing.T, f *finder) *dialogCheck {
	t.Helper()
	for _, object := range test.LaidOutObjects(f.win.Content()) {
		if check, ok := object.(*dialogCheck); ok && check.Text == "Search in archives" {
			return check
		}
	}
	t.Fatal("missing Search in archives option")
	return nil
}

func TestFindArchiveNames(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	defer func() {
		if hasWindow(u.app, "Find Files") {
			f.win.Close()
		}
	}()
	f.glob.SetText("readme")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if !slices.Equal(f.results, []string{filepath.Join(root, "src", "readme.md")}) {
		t.Fatalf("archive search must be opt-in: %v", f.results)
	}
	check := findArchiveSearchOption(t, f)
	f.text.SetText("absent content")
	test.Tap(check)
	if !f.text.Disabled() {
		t.Fatal("archive search must disable content entry")
	}
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	member := filepath.Join(name, "docs", "readme.txt")
	if !slices.Contains(f.results, member) || !slices.Contains(f.results, filepath.Join(root, "src", "readme.md")) || len(f.results) != 2 {
		t.Fatalf("search must ignore disabled content and include archive members: %v", f.results)
	}
	test.Tap(check)
	if f.text.Disabled() || f.text.Text != "absent content" {
		t.Fatal("content search not restored when archive search disabled")
	}
	test.Tap(check)
	test.Tap(f.caseSensitive)
	f.glob.SetText("README")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if len(f.results) != 0 {
		t.Fatal("archive search ignored case sensitivity")
	}
	test.Tap(f.caseSensitive)
	test.Tap(f.globMode)
	f.glob.SetText("*.txt")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if !slices.Contains(f.results, member) {
		t.Fatalf("wildcard failed for archive member: %v", f.results)
	}
	f.list.Select(slices.Index(f.results, member))
	tapDialogButton(t, u, "Go to")
	waitJobs(t, u)
	if u.left.path != filepath.Dir(member) || u.left.current().name != "readme.txt" || u.passwordWindow != nil {
		t.Fatalf("archive result navigation: %s %s", u.left.path, u.left.current().name)
	}
}

func TestFindArchiveFailureKeepsOtherResults(t *testing.T) {
	root := fixture(t)
	name := archiveFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "broken.zip"), []byte("invalid archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	defer func() {
		if hasWindow(u.app, "Find Files") {
			f.win.Close()
		}
	}()
	test.Tap(findArchiveSearchOption(t, f))
	f.glob.SetText("readme")
	f.glob.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	waitUntil(t, func() bool { return f.done })
	if !strings.Contains(f.status.Text, "search incomplete") || !slices.Contains(f.results, filepath.Join(name, "docs", "readme.txt")) || !slices.Contains(f.results, filepath.Join(root, "src", "readme.md")) {
		t.Fatalf("archive failure lost results or stayed silent: %v %s", f.results, f.status.Text)
	}
	check := findArchiveSearchOption(t, f)
	f.win.Canvas().Focus(check)
	check.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if hasWindow(u.app, "Find Files") || u.passwordWindow != nil {
		t.Fatal("Escape from archive option left parent blocked")
	}
}

func TestFindDialogSpacingAndProgress(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	defer f.win.Close()
	outer := f.win.Content().(*fyne.Container).Objects[0].(*container.ThemeOverride).Content.(*fyne.Container)
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
	test.Tap(f.regexp)
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

func TestFindCompactOptionsAndHoverHints(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	defer f.win.Close()
	test.LaidOutObjects(f.win.Content())
	driver := u.app.Driver()
	namePos := driver.AbsolutePositionForObject(f.glob)
	for _, b := range []*searchOption{f.caseSensitive, f.regexp, f.globMode} {
		pos := driver.AbsolutePositionForObject(b)
		if pos.X <= namePos.X || pos.Y != namePos.Y || b.AccessibilityLabel() == b.Text {
			t.Fatalf("option must share filename row and have accessible name: %s, %v vs %v", b.Text, pos, namePos)
		}
		size := b.Size()
		test.MoveMouse(f.win.Canvas(), pos.Add(fyne.NewPos(2, 2)))
		if !b.hint.Visible() || f.win.Canvas().Overlays().Top() != nil {
			t.Fatal("hover hint missing or intercepts clicks")
		}
		test.MoveMouse(f.win.Canvas(), pos.Add(fyne.NewPos(3, 3)))
		if !b.hint.Visible() {
			t.Fatal("hint flickers while pointer stays on option")
		}
		test.Tap(b)
		if !b.Checked || b.hint.Visible() || b.Size() != size {
			t.Fatal("option tap must select without resizing or leaving hint")
		}
		test.MoveMouse(f.win.Canvas(), fyne.NewPos(1, 1))
		if b.hint.Visible() {
			t.Fatal("hint remains after pointer exits")
		}
	}
	if f.regexp.Checked || !f.globMode.Checked || f.mode != "Wildcard" {
		t.Fatal("wildcard and regexp must be exclusive")
	}
	test.Tap(f.globMode)
	if f.mode != "Partial match" {
		t.Fatal("deselecting mode must restore partial match")
	}
}

func TestFindResultsGeometryStable(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.setDensity("compact")
	f := u.search()
	defer f.win.Close()
	// List refresh creates templates outside the dialog's rendering theme scope.
	template := f.list.CreateItem()
	var label *widget.Label
	switch row := template.(type) {
	case *widget.Label:
		label = row
	case *container.ThemeOverride:
		label = row.Content.(*widget.Label)
	}
	if label == nil || label.Theme().Size(theme.SizeNameInnerPadding) != 4 {
		t.Fatal("result template must keep compact padding outside dialog layout")
	}
	for i := 0; i < 60; i++ {
		f.results = append(f.results, filepath.Join(root, "long directory name", "readme.md"))
	}
	f.list.Refresh()
	test.LaidOutObjects(f.win.Content())
	f.list.ScrollToOffset(80)
	type geometry struct {
		pos  fyne.Position
		size fyne.Size
	}
	snapshot := func() []geometry {
		var out []geometry
		for _, o := range test.LaidOutObjects(f.list) {
			if l, ok := o.(*widget.Label); ok && l.Text == f.results[0] {
				out = append(out, geometry{u.app.Driver().AbsolutePositionForObject(l), l.Size()})
			}
		}
		return out
	}
	before := snapshot()
	if len(before) < 3 {
		t.Fatal("need visible result rows")
	}
	offset := f.list.GetScrollOffset()
	for _, row := range before[:3] {
		test.MoveMouse(f.win.Canvas(), row.pos.Add(fyne.NewPos(2, 2)))
		test.TapCanvas(f.win.Canvas(), row.pos.Add(fyne.NewPos(2, 2)))
		if !slices.Equal(before, snapshot()) || f.list.GetScrollOffset() != offset {
			t.Fatal("hover/selection changed result geometry or scroll offset")
		}
	}
}

func TestFindResultsTextGeometryStableWithoutRelayout(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.setDensity("compact")
	f := u.search()
	defer f.win.Close()
	for i := 0; i < 60; i++ {
		f.results = append(f.results, filepath.Join(root, "long directory name", "readme.md"))
	}
	f.list.Refresh()
	objects := laidOutObjects(f.win.Content())
	for _, o := range objects {
		if label, ok := o.(*widget.Label); ok && label.Text == f.results[0] {
			if padding := label.Theme().Size(theme.SizeNameInnerPadding); padding != 4 {
				t.Fatalf("result label lost compact theme: inner padding=%v, want 4", padding)
			}
		}
	}
	type geometry struct {
		pos  fyne.Position
		size fyne.Size
	}
	// Observe cached text positions directly: LaidOutObjects repairs layout and hides jumps.
	snapshot := func() map[*canvas.Text]geometry {
		out := make(map[*canvas.Text]geometry)
		var walk func(fyne.CanvasObject)
		walk = func(o fyne.CanvasObject) {
			switch o := o.(type) {
			case *canvas.Text:
				out[o] = geometry{u.app.Driver().AbsolutePositionForObject(o), o.Size()}
			case fyne.Widget:
				for _, child := range test.WidgetRenderer(o).Objects() {
					walk(child)
				}
			case *fyne.Container:
				for _, child := range o.Objects {
					walk(child)
				}
			}
		}
		walk(f.list)
		return out
	}
	before := snapshot()
	if len(before) < 3 {
		t.Fatal("need visible result text")
	}
	check := func(stage string) {
		t.Helper()
		after := snapshot()
		for text, geometry := range before {
			if got, ok := after[text]; !ok || got != geometry {
				t.Fatalf("%s moved result text: before=%+v after=%+v present=%v", stage, geometry, got, ok)
			}
		}
	}
	var labels []*widget.Label
	for _, o := range objects {
		if label, ok := o.(*widget.Label); ok && label.Text == f.results[0] {
			labels = append(labels, label)
		}
	}
	for _, label := range labels[:3] {
		pos := u.app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(2, 2))
		test.MoveMouse(f.win.Canvas(), pos)
		check("hover")
		test.TapCanvas(f.win.Canvas(), pos)
		check("click")
	}
}

func TestFindEditableFolder(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	f := u.search()
	if f.dir.Text != root {
		t.Fatal("search folder must default to active pane")
	}
	var labeled bool
	for _, o := range test.LaidOutObjects(f.win.Content()) {
		if form, ok := o.(*widget.Form); ok {
			for _, item := range form.Items {
				if item.Text == "In:" && item.Widget == f.dir {
					labeled = true
				}
			}
		}
	}
	if !labeled {
		t.Fatal("search folder needs labeled edit field")
	}
	for _, content := range []string{"", "hello"} {
		f.text.SetText(content)
		f.glob.SetText("readme")
		for _, dir := range []string{filepath.Join(root, "src"), "dst", "src"} {
			f.dir.SetText(dir)
			f.win.Canvas().Focus(f.dir)
			f.dir.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			// Changing input during work must not change its captured root.
			f.dir.SetText("missing")
			waitUntil(t, func() bool { return f.done })
			want := []string{filepath.Join(root, "src", "readme.md")}
			if dir == "dst" {
				want = nil
			}
			if !slices.Equal(f.results, want) || u.left.path != root {
				t.Fatalf("folder %q, content %q: %v", dir, content, f.results)
			}
		}
		for _, dir := range []string{filepath.Join(root, "missing"), filepath.Join(root, "src", "readme.md")} {
			f.dir.SetText(dir)
			f.dir.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			waitUntil(t, func() bool { return f.done })
			if len(f.results) != 0 || !strings.Contains(f.status.Text, "search incomplete") || f.activity.Running() {
				t.Fatalf("invalid folder not reported: %s", f.status.Text)
			}
		}
	}
	f.dir.SetText("")
	f.dir.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if f.status.Text != "Enter a search folder" || f.activity.Running() {
		t.Fatal("empty folder must not start search")
	}
	f.dir.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if hasWindow(u.app, "Find Files") || u.passwordWindow != nil || u.left.path != root {
		t.Fatal("Escape from folder entry must cancel without navigation")
	}
}
