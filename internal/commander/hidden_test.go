package commander

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestHiddenToggleRepaintsScrolledPanes(t *testing.T) {
	root := fixture(t)
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf(".hidden-%03d", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, brief := range []bool{false, true} {
		t.Run(fmt.Sprint(brief), func(t *testing.T) {
			u := newTestUI(t, test.NewTempApp(t), root, root)
			for _, p := range []*pane{u.left, u.right} {
				p.setBrief(brief)
			}
			u.toggleHidden()
			for _, p := range []*pane{u.left, u.right} {
				u.activate(p)
				p.moveTo(len(p.entries) - 1)
			}
			u.hiddenItem.Action()
			for _, p := range []*pane{u.left, u.right} {
				// Inspect existing renderer objects without layout, refresh, or pointer events.
				var rows []*row
				var visit func(fyne.CanvasObject)
				visit = func(o fyne.CanvasObject) {
					if !o.Visible() {
						return
					}
					if r, ok := o.(*row); ok {
						rows = append(rows, r)
						return
					}
					if w, ok := o.(fyne.Widget); ok {
						for _, child := range test.WidgetRenderer(w).Objects() {
							visit(child)
						}
					}
					if c, ok := o.(*fyne.Container); ok {
						for _, child := range c.Objects {
							visit(child)
						}
					}
				}
				visit(p.body)
				if len(rows) == 0 {
					t.Fatal("toggle left pane empty until next input")
				}
				for _, r := range rows {
					if r.id < 0 || r.id >= len(p.entries) {
						t.Fatalf("stale row %d after toggle", r.id)
					}
				}
			}
		})
	}
}
