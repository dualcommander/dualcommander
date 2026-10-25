package commander

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestBriefCellReusesUnchangedLayout(t *testing.T) {
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), root, root)
	u.left.setBrief(true)
	g := u.left.grid
	r := g.CreateCell().(*row)
	g.UpdateCell(widget.TableCellID{}, r)
	c := r.content.(*fyne.Container)
	before := c.Layout
	g.UpdateCell(widget.TableCellID{}, r)
	if c.Layout != before {
		t.Fatal("unchanged brief cell rebuilt layout during selection refresh")
	}
	g.cell.Width += 20
	g.UpdateCell(widget.TableCellID{}, r)
	if c.Layout.MinSize(c.Objects) != g.cell {
		t.Fatal("brief cell failed to adopt resized grid geometry")
	}
}
