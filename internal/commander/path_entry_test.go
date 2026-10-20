package commander

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestPathLengthDoesNotWidenPane(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	for _, p := range []*pane{u.left, u.right} {
		e := p.pathBar
		e.SetText("/tmp")
		entryMin, paneMin := e.MinSize(), p.box.MinSize()
		width := p.box.Size().Width
		path := "/Users/" + strings.Repeat("long-parent/", 40) + "current-folder"
		e.SetText(path)
		p.box.Refresh()
		r := test.WidgetRenderer(e).(*pathRenderer)
		for _, editing := range []bool{false, true, false} {
			if editing {
				u.win.Canvas().Focus(e)
			} else {
				u.win.Canvas().Unfocus()
			}
			if e.MinSize() != entryMin || r.MinSize() != entryMin || p.box.MinSize() != paneMin || p.box.Size().Width != width {
				t.Fatalf("path widened pane (editing=%v): entry=%v pane=%v width=%v", editing, e.MinSize(), p.box.MinSize(), p.box.Size().Width)
			}
			if e.Text != path || e.Scroll != fyne.ScrollNone {
				t.Fatal("full editable path or no-scroll setting lost")
			}
			if !editing && (!strings.Contains(r.label.Text, "...") || !strings.HasSuffix(r.label.Text, "current-folder")) {
				t.Fatalf("path not shortened around current folder: %q", r.label.Text)
			}
		}
	}
}

func TestShortPath(t *testing.T) {
	for _, tc := range []struct {
		path  string
		width int
		want  string
	}{
		{"/Users/demo/projects/current", 40, "/Users/demo/projects/current"},
		{"/Users/demo/projects/current", 20, "/Users/.../current"},
		{"/Users/demo/projects/current", 12, "/.../current"},
		{"/Users/demo/projects/current", 11, ".../current"},
		{"/Users/demo/projects/current", 8, "curre..."},
		{`C:\Users\demo\projects\current`, 20, `C:\Users\...\current`},
		{"/home/demo/世界文件夹", 6, "世界文..."},
		{"/", 1, "/"},
	} {
		if got := shortPath(tc.path, func(s string) bool { return utf8.RuneCountInString(s) <= tc.width }); got != tc.want {
			t.Errorf("shortPath(%q, %d) = %q, want %q", tc.path, tc.width, got, tc.want)
		}
	}
}

func TestPathEntryResizeAndEdit(t *testing.T) {
	test.NewTempApp(t)
	e := newPathEntry()
	path := "/Users/demo/projects/very-long-parent/current-folder"
	e.SetText(path)
	w := test.NewWindow(e)
	defer w.Close()
	r := test.WidgetRenderer(e).(*pathRenderer)
	e.Resize(fyne.NewSize(180, e.MinSize().Height))
	if r.label.Text == path || e.Text != path || !r.cover.Visible() || e.Scroll != fyne.ScrollNone || e.Wrapping != fyne.TextWrapOff {
		t.Fatal("path not shortened or full path/scroll settings changed")
	}
	e.Resize(fyne.NewSize(1000, e.MinSize().Height))
	if r.label.Text != path {
		t.Fatal("resize did not restore full path")
	}
	w.Canvas().Focus(e)
	if r.cover.Visible() || e.Text != path {
		t.Fatal("focus did not reveal full editable path")
	}
	e.TypedShortcut(&fyne.ShortcutSelectAll{})
	if e.SelectedText() != path {
		t.Fatal("copy/select uses shortened path")
	}
	e.SetText("/tmp/edited")
	w.Canvas().Unfocus()
	if !r.cover.Visible() || r.label.Text != e.Text {
		t.Fatal("unfocus did not restore path display")
	}
}
