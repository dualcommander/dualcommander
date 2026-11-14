package commander

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type renameDelay struct {
	pending   func()
	cancelled []func()
	delay     time.Duration
}

func fakeRenameDelay(u *ui) *renameDelay {
	c := &renameDelay{}
	u.renameAfter = func(delay time.Duration, f func()) func() {
		c.delay = delay
		c.pending = f
		return func() { c.cancelled = append(c.cancelled, f); c.pending = nil }
	}
	return c
}

func (c *renameDelay) fire() {
	if c.pending != nil {
		f := c.pending
		c.pending = nil
		f()
	}
}

func TestMultiRenameDefaultsAndExtensionlessPreview(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	if err := os.WriteFile(filepath.Join(src, "LICENSE"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	u.left.marked["LICENSE"], u.left.marked["readme.md"] = true, true
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	if got := findEntry(t, w, "Match template").Text; got != "{name}.{ext}" {
		t.Errorf("match default = %q", got)
	}
	if got := findEntry(t, w, "Name template").Text; got != "{{.name}}.{{.ext}}" {
		t.Errorf("name default = %q", got)
	}
	findEntry(t, w, "Match template").SetText("{name}.{ext}")
	findEntry(t, w, "Name template").SetText("{{.name}}.{{.ext}}")
	clock.fire()
	drainUI()
	for _, o := range test.LaidOutObjects(w.Content()) {
		if list, ok := o.(*dialogList); ok {
			if list.Length() != 2 {
				t.Fatal("selected rows missing")
			}
			row := list.CreateItem()
			for i := 0; i < list.Length(); i++ {
				list.UpdateItem(i, row)
				cells := row.(*container.ThemeOverride).Content.(*fyne.Container).Objects
				if cells[0].(*widget.Label).Text == "LICENSE" && cells[1].(*widget.Label).Text != "LICENSE" {
					t.Errorf("extensionless result = %q", cells[1].(*widget.Label).Text)
				}
			}
		}
	}
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("identity template enabled rename")
	}
	w.Close()
}

func TestMultiRenameAutomaticPreviewAction(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{name}")
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("Rename enabled before automatic preview")
	}
	output := findEntry(t, w, "Name template")
	output.SetText("new_{{.name}}")
	output.SetText("latest_{{.name}}")
	if clock.delay != 250*time.Millisecond {
		t.Errorf("preview debounce = %v, want 250ms", clock.delay)
	}
	for _, stale := range clock.cancelled {
		stale()
	}
	drainUI()
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("stale timer enabled Rename")
	}
	clock.fire()
	var progress *widget.ProgressBar
	var track *fyne.Container
	for _, o := range test.LaidOutObjects(w.Content()) {
		if b, ok := o.(*widget.ProgressBar); ok {
			progress = b
		}
		if c, ok := o.(*fyne.Container); ok {
			if _, ok := c.Layout.(operationTrackLayout); ok {
				track = c
			}
		}
	}
	if progress == nil || track == nil || !track.Visible() || !findButton(t, w, "Rename").Disabled() {
		t.Fatal("preview progress missing or Rename enabled during work")
	}
	drainUI()
	if track.Visible() || progress.Value != 1 {
		t.Fatal("completed progress not hidden")
	}
	if findButton(t, w, "Rename").Disabled() {
		t.Fatal("automatic preview missing")
	}
	output.SetText("{{")
	if clock.pending != nil || !findButton(t, w, "Rename").Disabled() {
		t.Fatal("invalid input scheduled preview")
	}
	output.SetText("again_{{.name}}")
	w.Close()
	for _, stale := range clock.cancelled {
		stale()
	}
	drainUI()
	if track.Visible() || !findButton(t, w, "Rename").Disabled() {
		t.Fatal("closed dialog resumed preview")
	}
}

func TestMultiRenamePreviewTextGeometryStable(t *testing.T) {
	for _, density := range densities {
		t.Run(density, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.setDensity(density)
			u.left.marked["readme.md"], u.left.marked["song.mp3"] = true, true
			clock := fakeRenameDelay(u)
			u.renameMulti()
			w := windowByTitle(t, u.app, "Multi-Rename")
			defer w.Close()
			var list *dialogList
			var outputEntry *escEntry
			for _, o := range laidOutObjects(w.Content()) {
				if l, ok := o.(*dialogList); ok {
					list = l
				}
				if e, ok := o.(*escEntry); ok && e.PlaceHolder == "Name template" {
					outputEntry = e
				}
			}
			if list == nil || outputEntry == nil {
				t.Fatal("preview list or name entry missing")
			}
			for _, o := range laidOutObjects(list.CreateItem()) {
				if label, ok := o.(*widget.Label); ok && label.Theme().Size(theme.SizeNameInnerPadding) != 8 {
					t.Fatalf("preview template lost dialog padding: got %v, want 8", label.Theme().Size(theme.SizeNameInnerPadding))
				}
			}
			type geometry struct {
				pos  fyne.Position
				size fyne.Size
			}
			// Read cached positions only; layout helpers conceal selection-time jumps.
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
				walk(list)
				return out
			}
			for _, output := range []string{"{{.name}}.{{.ext}}", "new_{{.name}}.{{.ext}}"} {
				outputEntry.SetText(output)
				clock.fire()
				drainUI()
				laidOutObjects(w.Content())
				before := snapshot()
				if len(before) < 4 {
					t.Fatal("need both columns of two preview rows")
				}
				for i := 0; i < list.Length(); i++ {
					list.Select(i)
					after := snapshot()
					for text, want := range before {
						if got, ok := after[text]; !ok || got != want {
							t.Fatalf("selection moved preview text %q: before=%+v after=%+v present=%v", text.Text, want, got, ok)
						}
					}
				}
			}
		})
	}
}

func TestMultiRenamePreviewRowHeightStable(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.setDensity("compact")
	u.left.marked["readme.md"], u.left.marked["song.mp3"] = true, true
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	defer w.Close()
	findEntry(t, w, "Name template").SetText("new_{{.name}}.{{.ext}}")
	clock.fire()
	drainUI()
	for _, o := range test.LaidOutObjects(w.Content()) {
		list, ok := o.(*dialogList)
		if !ok {
			continue
		}
		row := list.CreateItem()
		for _, child := range test.LaidOutObjects(row) {
			if label, ok := child.(*widget.Label); ok && label.Text == "" {
				t.Error("preview template has no text line for initial row measurement")
			}
		}
		before := row.MinSize().Height
		for i := 0; i < list.Length(); i++ {
			list.UpdateItem(i, row)
			if after := row.MinSize().Height; after != before {
				t.Fatalf("populating preview changed row height: %v to %v", before, after)
			}
		}
		return
	}
	t.Fatal("preview list missing")
}

// findEntry locates a dialog entry by placeholder; avoids depending on translated labels.
func findEntry(t *testing.T, w fyne.Window, placeholder string) *escEntry {
	t.Helper()
	for _, o := range test.LaidOutObjects(w.Content()) {
		if e, ok := o.(*escEntry); ok && e.PlaceHolder == placeholder {
			return e
		}
	}
	t.Fatalf("missing entry with placeholder %q", placeholder)
	return nil
}

// findButton locates a dialog button by its text.
func findButton(t *testing.T, w fyne.Window, text string) *dialogButton {
	t.Helper()
	for _, o := range test.LaidOutObjects(w.Content()) {
		if b, ok := o.(*dialogButton); ok && b.Text == text {
			return b
		}
	}
	t.Fatalf("missing button %q", text)
	return nil
}

func TestMultiRenameInlineErrors(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	from, to := findEntry(t, w, "Match template"), findEntry(t, w, "Name template")
	for _, tc := range []struct{ match, output, field string }{
		{"{", "{{.name}}", "Match template"},
		{"{name}", "{{", "Name template"},
		{"{name}", "{{.missing}}", "Name template"},
	} {
		from.SetText(tc.match)
		to.SetText(tc.output)
		_, _, err := renameTemplate(tc.match, tc.output)
		assertUIText(t, w.Content(), tc.field+": "+err.Error())
		if !findButton(t, w, "Rename").Disabled() || clock.pending != nil {
			t.Fatal("invalid template allowed preview or rename")
		}
	}
	from.SetText("{name}")
	to.SetText("new_{{.name}}")
	clock.fire()
	drainUI()
	if findButton(t, w, "Rename").Disabled() {
		t.Fatal("corrected template stayed invalid")
	}
	w.Close()
}

func TestMultiRenameHelpAndSkippedRows(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.left.marked["readme.md"], u.left.marked["song.mp3"] = true, true
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{title}.md")
	findEntry(t, w, "Name template").SetText("{{.title}}.md")
	clock.fire()
	drainUI()
	var hints, manuals int
	for _, o := range test.LaidOutObjects(w.Content()) {
		if b, ok := o.(*dialogButton); ok && b.Text == "User manual" {
			manuals++
			if b.Importance != widget.LowImportance {
				t.Fatal("manual link styling invalid")
			}
			position := u.app.Driver().AbsolutePositionForObject
			if position(b).Y >= position(findEntry(t, w, "Match template")).Y {
				t.Fatal("manual link not above form")
			}
		}
		if h, ok := o.(*searchOption); ok {
			hints++
			position := u.app.Driver().AbsolutePositionForObject
			if position(h).Y >= position(findEntry(t, w, "Match template")).Y {
				t.Error("help not above form")
			}
			if h.hint.Visible() {
				t.Fatal("hint visible before interaction")
			}
			h.MouseIn(nil)
			if !h.hint.Visible() {
				t.Fatal("hover hint missing")
			}
			icons := 0
			for _, child := range test.LaidOutObjects(h.hint) {
				if _, ok := child.(*dialogInfoIcon); ok {
					icons++
				}
			}
			if icons != 1 || h.Icon == nil || h.Text != "" {
				t.Fatal("hint does not match Settings info styling")
			}
			h.MouseOut()
		}
		if l, ok := o.(*dialogList); ok {
			if l.Length() != 2 {
				t.Fatal("selected skipped rows missing")
			}
			row := l.CreateItem()
			l.UpdateItem(0, row)
			assertUIText(t, row, "readme.md (Unchanged)")
			l.UpdateItem(1, row)
			assertUIText(t, row, "song.mp3")
		}
	}
	if hints != 1 || manuals != 1 {
		t.Fatalf("help controls = %d, manual links = %d", hints, manuals)
	}
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("unchanged names enabled Apply")
	}
	w.Close()
}

func TestMultiRenameSelectedOnly(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"] = true
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	assertUIText(t, w.Content(), "1 files · 0 folders")
	findEntry(t, w, "Match template").SetText("{title}.{ext}")
	findEntry(t, w, "Name template").SetText("{{upper .title}}.txt")
	clock.fire()
	drainUI()
	if findButton(t, w, "Rename").Disabled() {
		t.Fatal("selected item template rejected")
	}
	assertUIText(t, w.Content(), "1 files · 0 folders")
	findButton(t, w, "Rename").Tapped(nil)
	waitJobs(t, u)
	for _, name := range []string{"README.txt", "song.mp3", "photos"} {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMultiRenameNumberedPreview(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	for _, name := range []string{"a.jpeg", "b.jpeg"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	for _, name := range []string{"a.jpeg", "b.jpeg", "readme.md", "photos"} {
		u.left.marked[name] = true
	}
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{name}.jpeg")
	output := findEntry(t, w, "Name template")
	output.SetText(`{{printf "%02d" .Index}}.jpeg`)
	for run := 0; run < 2; run++ {
		if run == 1 {
			output.SetText(`x{{.Index}}`)
			output.SetText(`{{printf "%02d" .Index}}.jpeg`)
		}
		clock.fire()
		drainUI()
		if findButton(t, w, "Rename").Disabled() {
			t.Fatal("numbering preview rejected")
		}
		for _, o := range test.LaidOutObjects(w.Content()) {
			if l, ok := o.(*dialogList); ok {
				for i := 0; i < l.Length(); i++ {
					row := l.CreateItem()
					l.UpdateItem(i, row)
					if i == 1 {
						assertUIText(t, row, "a.jpeg", "01.jpeg")
					}
					if i == 2 {
						assertUIText(t, row, "b.jpeg", "02.jpeg")
					}
				}
			}
		}
	}
	findButton(t, w, "Rename").Tapped(nil)
	waitJobs(t, u)
	for _, name := range []string{"01.jpeg", "02.jpeg", "readme.md", "photos"} {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMultiRenameIndexFunction(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	for _, name := range []string{"LICENSE", "a.tar.gz"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(src, "album.v1"), 0700); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	for _, name := range []string{"album.v1", "LICENSE", "a.tar.gz"} {
		u.left.marked[name] = true
	}
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{name}")
	output := findEntry(t, w, "Name template")
	for _, o := range test.LaidOutObjects(w.Content()) {
		if _, ok := o.(*dialogCheck); ok {
			t.Error("counter checkbox remains; numbering belongs in template")
		}
	}
	output.SetText("{{.name}}_{{index 3}}")
	clock.fire()
	drainUI()
	if findButton(t, w, "Rename").Disabled() {
		t.Fatal("index preview rejected")
	}
	output.SetText("{{.name}}")
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("template edit kept stale preview")
	}
	clock.fire()
	drainUI()
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("identity rename enabled after disabling index")
	}
	output.SetText("{{.name}}_{{index 3}}")
	clock.fire()
	drainUI()
	findButton(t, w, "Rename").Tapped(nil)
	waitJobs(t, u)
	for old, name := range map[string]string{"album.v1": "album.v1_001", "a.tar.gz": "a.tar.gz_002", "LICENSE": "LICENSE_003"} {
		if exists(filepath.Join(src, old)) || !exists(filepath.Join(src, name)) || !slices.Contains(names(u.left), name) {
			t.Fatalf("rename %q to %q failed", old, name)
		}
	}
	if !exists(filepath.Join(src, "readme.md")) {
		t.Fatal("unselected file renamed")
	}
}

func TestMultiRenameIndexCollision(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	if err := os.WriteFile(filepath.Join(src, "readme_001.md"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Name template").SetText("{{.name}}_{{index 3}}.{{.ext}}")
	clock.fire()
	drainUI()
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("index collision accepted")
	}
	w.Close()
	data, err := os.ReadFile(filepath.Join(src, "readme_001.md"))
	if err != nil || string(data) != "keep" || !exists(filepath.Join(src, "readme.md")) {
		t.Fatalf("collision changed files: %q, %v", data, err)
	}
}

func TestMultiRenameStalePreviewAndFolders(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	if err := os.WriteFile(filepath.Join(src, "photos", "nested.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	cursorTo(t, u.left, "photos")
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{name}")

	output := findEntry(t, w, "Name template")
	output.SetText("new_{{.name}}")
	clock.fire()
	output.SetText("pictures")
	drainUI()
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("stale preview enabled Rename")
	}
	clock.fire()
	drainUI()
	assertUIText(t, w.Content(), "0 files · 1 folders")
	findButton(t, w, "Rename").Tapped(nil)
	waitJobs(t, u)
	if _, err := os.Stat(filepath.Join(src, "pictures", "nested.txt")); err != nil {
		t.Fatal(err)
	}
	clock = fakeRenameDelay(u)
	u.renameMulti()
	w = windowByTitle(t, u.app, "Multi-Rename")
	findEntry(t, w, "Match template").SetText("{}.txt")
	findEntry(t, w, "Name template").SetText("new_{{.C0}}")
	clock.fire()
	drainUI()
	assertUIText(t, w.Content(), "0 files · 1 folders")
	if !findButton(t, w, "Rename").Disabled() {
		t.Fatal("recursive match accepted")
	}
	w.Close()
}

func drainUI() {
	for len(uiQueue) > 0 {
		(<-uiQueue)()
	}
}

func TestMultiRenameDryRun(t *testing.T) {
	root := fixture(t)
	src := filepath.Join(root, "src")
	for _, name := range []string{"one-two.jpg", "three-four.jpg"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	u := newTestUI(t, test.NewTempApp(t), src, filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	u.left.marked["one-two.jpg"] = true
	u.left.marked["three-four.jpg"] = true
	clock := fakeRenameDelay(u)
	u.renameMulti()
	w := windowByTitle(t, u.app, "Multi-Rename")

	findEntry(t, w, "Match template").SetText("{}-{}.jpg")
	output := findEntry(t, w, "Name template")
	output.SetText("{{.C1}}-{{.C0}}.png")
	action := findButton(t, w, "Rename")
	assertUIText(t, w.Content(), "2 files · 0 folders")
	for _, o := range test.LaidOutObjects(w.Content()) {
		if b, ok := o.(*dialogButton); ok && b != action && b.Text != "Cancel" && b.Text != "User manual" {
			t.Fatal("separate Apply button shown before dry run")
		}
	}
	clock.fire()
	drainUI()
	assertUIText(t, w.Content(), "2 files · 0 folders", "Original", "New name")
	var list *dialogList
	for _, o := range test.LaidOutObjects(w.Content()) {
		if l, ok := o.(*dialogList); ok {
			list = l
		}
	}
	if list == nil || list.Length() != 2 {
		t.Fatal("preview missing matched rows")
	}
	row := list.CreateItem()
	list.UpdateItem(0, row)
	assertUIText(t, row, "one-two.jpg", "two-one.png")
	list.Select(0)
	if _, err := os.Stat(filepath.Join(src, "one-two.jpg")); err != nil {
		t.Fatal("dry run changed disk")
	}
	output.SetText("{{.C0}}-{{.C1}}.png")
	if action.Text != "Rename" || !action.Disabled() || list.Length() != 0 {
		t.Fatal("edit kept reviewed preview")
	}
	clock.fire()
	drainUI()
	findButton(t, w, "Rename").Tapped(nil)
	waitJobs(t, u)
	for _, name := range []string{"one-two.png", "three-four.png", "readme.md"} {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Contains(names(u.left), "one-two.png") || u.left.marked["readme.md"] {
		t.Fatal("pane not refreshed or marks changed")
	}
}

func TestMultiRenameValidation(t *testing.T) {
	for _, tc := range []struct{ from, to string }{{"{}", "same"}, {"{}", "../bad"}, {"{}", "{{.C1}}"}, {"{", "{{.C0}}"}} {
		t.Run(tc.from+tc.to, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			u.left.marked["readme.md"] = true
			u.left.marked["song.mp3"] = true
			clock := fakeRenameDelay(u)
			u.renameMulti()
			w := windowByTitle(t, u.app, "Multi-Rename")
			findEntry(t, w, "Match template").SetText(tc.from)
			findEntry(t, w, "Name template").SetText(tc.to)
			clock.fire()
			drainUI()
			for _, o := range test.LaidOutObjects(w.Content()) {
				if b, ok := o.(*dialogButton); ok && b.Text != "Cancel" && b.Text != "User manual" && !b.Disabled() {
					t.Fatal("invalid batch accepted")
				}
			}
			w.Close()
		})
	}
}

func TestMultiRenameCancelControls(t *testing.T) {
	for _, target := range []string{"canvas", "native", "Cancel", "Match template", "Name template", "list", "Rename", "pending", "User manual"} {
		t.Run(target, func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			cursorTo(t, u.left, "readme.md")
			u.left.marked["readme.md"] = true
			u.left.marked["song.mp3"] = true
			clock := fakeRenameDelay(u)
			u.renameMulti()
			w := windowByTitle(t, u.app, "Multi-Rename")
			findEntry(t, w, "Match template").SetText("{name}")
			findEntry(t, w, "Name template").SetText("new_{{.name}}")
			clock.fire()
			drainUI()
			if target == "pending" {
				findEntry(t, w, "Name template").SetText("other_{{.name}}")
			}
			press(u, fyne.KeyF7)
			if hasWindow(u.app, "New Folder") {
				t.Fatal("parent input not blocked")
			}
			switch target {
			case "native", "pending":
				w.Close()
			case "Cancel":
				findButton(t, w, "Cancel").Tapped(nil)
			case "canvas":
				w.Canvas().Unfocus()
				w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape})
			default:
				var control fyne.Focusable
				for _, o := range test.LaidOutObjects(w.Content()) {
					switch c := o.(type) {
					case *escEntry:
						if c.PlaceHolder == target {
							control = c
						}
					case *searchOption:
						if c.Text == target {
							control = c
						}
					case *dialogList:
						if target == "list" {
							control = c
						}
					case *dialogButton:
						if c.Text == target {
							control = c
						}
					}
				}
				if control == nil {
					t.Fatal("control missing")
				}
				w.Canvas().Focus(control)
				control.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			}
			if hasWindow(u.app, "Multi-Rename") || u.passwordWindow != nil {
				t.Fatal("modal not released")
			}
			if _, err := os.Stat(filepath.Join(root, "src", "readme.md")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
