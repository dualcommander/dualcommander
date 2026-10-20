package commander

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestStatusDetailsMatchFullView(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), root)
	p := u.left
	for _, brief := range []bool{false, true} {
		p.refresh()
		p.markAll(false)
		p.setBrief(brief)
		for _, name := range []string{"readme.md", "docs"} {
			cursorTo(t, p, name)
			if !p.statusRow.Visible() || p.status.Visible() {
				t.Fatal("normal details must use column row, not message label")
			}
			r := newRowWidget(p, true)
			p.fillRow(p.cursor, r)
			if p.statusRow.Objects[1].Visible() || !r.content.(*fyne.Container).Objects[1].Visible() {
				t.Fatal("status icon must stay hidden; file-list icon must stay visible")
			}
			for _, width := range []float32{500, 900} {
				size := fyne.NewSize(width, r.MinSize().Height)
				p.statusRow.Resize(size)
				r.content.Resize(size)
				var got, want []*widget.Label
				for _, pair := range []struct {
					object fyne.CanvasObject
					labels *[]*widget.Label
				}{{p.statusRow, &got}, {r.content, &want}} {
					for _, o := range laidOutObjects(pair.object) {
						if label, ok := o.(*widget.Label); ok {
							*pair.labels = append(*pair.labels, label)
						}
					}
				}
				if len(got) != 4 || len(want) != 4 {
					t.Fatalf("expected name/size/date/attributes: %d / %d", len(got), len(want))
				}
				for i := range got {
					if got[i].Text != want[i].Text || got[i].Alignment != want[i].Alignment || got[i].TextStyle != want[i].TextStyle || (i > 0 && (got[i].Size() != want[i].Size() || got[i].Position() != want[i].Position())) {
						t.Fatalf("column %d differs from full view at width %v", i, width)
					}
				}
				if got[0].Position().X != 0 || got[0].Size().Width <= want[0].Size().Width {
					t.Fatal("status filename must reclaim hidden icon space")
				}
				actualColumns := p.statusRow.Objects[2].(*fyne.Container)
				expectedColumns := r.content.(*fyne.Container).Objects[2].(*fyne.Container)
				if actualColumns.Position() != expectedColumns.Position() {
					t.Fatal("metadata columns must start at full-view offset")
				}
				for i := range actualColumns.Objects {
					if actualColumns.Objects[i].Position() != expectedColumns.Objects[i].Position() {
						t.Fatalf("column %d offset differs from full view", i)
					}
				}
				if got[0].Truncation != fyne.TextTruncateEllipsis || got[1].Alignment != fyne.TextAlignTrailing {
					t.Fatal("name must truncate; size must align right")
				}
			}
		}
		p.typeSearch('d')
		if p.statusRow.Visible() || !p.status.Visible() {
			t.Fatal("search must switch back to message label")
		}
		press(u, fyne.KeySpace)
		if p.statusRow.Visible() || !p.status.Visible() {
			t.Fatal("folder scan must preserve progress message")
		}
		waitUntil(t, func() bool { return !p.sizeBusy })
	}
}
