package commander

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
)

func TestOutboundFileDrag(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"], u.left.marked["photos"] = true, true
	var source *row
	for _, object := range laidOutObjects(u.left.body) {
		if r, ok := object.(*row); ok && r.id == u.left.cursor {
			source = r
			break
		}
	}
	if source == nil {
		t.Fatal("missing source row")
	}
	calls := 0
	u.startFileDrag = func(_ fyne.Window, paths []string) bool {
		calls++
		if len(paths) != 2 {
			t.Fatalf("want marked files, got %v", paths)
		}
		return true
	}
	source.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(10, 10)}})
	if calls != 0 {
		t.Fatal("internal drag started native session")
	}
	source.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(-10, 10)}})
	source.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: fyne.NewPos(-20, 10)}})
	source.DragEnd()
	if calls != 1 {
		t.Fatalf("want one native session, got %d", calls)
	}
	if u.passwordWindow != nil || !exists(filepath.Join(root, "src", "readme.md")) || exists(filepath.Join(root, "dst", "readme.md")) {
		t.Fatal("native drag also performed internal transfer")
	}
}

func TestPaneDropCopy(t *testing.T) {
	for _, brief := range []bool{false, true} {
		t.Run(map[bool]string{false: "detailed", true: "brief"}[brief], func(t *testing.T) {
			root := fixture(t)
			u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
			u.left.setBrief(brief)
			cursorTo(t, u.left, "readme.md")
			u.left.marked["readme.md"], u.left.marked["photos"] = true, true
			var source *row
			for _, object := range laidOutObjects(u.left.body) {
				if r, ok := object.(*row); ok && r.id == u.left.cursor {
					source = r
					break
				}
			}
			if source == nil {
				t.Fatal("missing source row")
			}
			drag, ok := any(source).(fyne.Draggable)
			if !ok {
				t.Fatal("file row does not support dragging")
			}
			position := u.app.Driver().AbsolutePositionForObject(u.right.body).Add(fyne.NewPos(20, 20))
			drag.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: position}})
			drag.DragEnd()
			tapDialogButton(t, u, "Copy")
			waitJobs(t, u)
			for _, name := range []string{"readme.md", "photos/a.png"} {
				if !exists(filepath.Join(root, "dst", name)) || !exists(filepath.Join(root, "src", name)) {
					t.Fatalf("copy missing or source removed: %s", name)
				}
			}
		})
	}
}

func TestExternalDropCopy(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	laidOutObjects(u.win.Content())
	handler, ok := any(u).(interface {
		dropURIs(fyne.Position, []fyne.URI)
	})
	if !ok {
		t.Fatal("external file drops unsupported")
	}
	position := u.app.Driver().AbsolutePositionForObject(u.right.body).Add(fyne.NewPos(20, 20))
	for _, uris := range [][]fyne.URI{nil, {nil}, {storage.NewURI("https://example.com/file")}} {
		handler.dropURIs(position, uris)
		if u.passwordWindow != nil {
			t.Fatal("invalid drop opened dialog")
		}
	}
	handler.dropURIs(fyne.NewPos(-1, -1), []fyne.URI{storage.NewFileURI(filepath.Join(root, "src", "readme.md"))})
	if u.passwordWindow != nil {
		t.Fatal("drop outside panes opened dialog")
	}
	handler.dropURIs(position, []fyne.URI{storage.NewFileURI(filepath.Join(root, "src", "readme.md"))})
	blocked := u.passwordWindow
	handler.dropURIs(position, []fyne.URI{storage.NewFileURI(filepath.Join(root, "src", "song.mp3"))})
	if u.passwordWindow != blocked {
		t.Fatal("drop replaced modal dialog")
	}
	tapDialogButton(t, u, "Cancel")
	if exists(filepath.Join(root, "dst", "readme.md")) {
		t.Fatal("cancel copied file")
	}
	laidOutObjects(u.win.Content())
	position = u.split.Position().Add(u.right.box.Position()).Add(fyne.NewPos(20, 100))
	handler.dropURIs(position, []fyne.URI{storage.NewFileURI(filepath.Join(root, "src", "readme.md"))})
	tapDialogButton(t, u, "Copy")
	waitJobs(t, u)
	if !exists(filepath.Join(root, "dst", "readme.md")) {
		t.Fatal("external copy missing")
	}
}
