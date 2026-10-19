package commander

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// row wraps a rendered item: click moves cursor (Mod/Shift = mark), double-click opens.
type row struct {
	widget.BaseWidget
	p       *pane
	id      int
	content fyne.CanvasObject
}

func newRowWidget(p *pane, detailed bool) *row {
	r := &row{p: p, content: newRowContent(detailed)}
	r.ExtendBaseWidget(r)
	return r
}

func (r *row) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.content) }

// MouseDown acts at once, like native file managers. Tapped would be delayed ~300ms by
// Fyne's double-tap wait and dropped entirely on quick clicks of two different rows.
func (r *row) MouseDown(e *desktop.MouseEvent) {
	if e.Button != desktop.MouseButtonPrimary {
		return
	}
	r.p.ui.activate(r.p)
	r.p.click(r.id, e.Modifier)
}
func (r *row) MouseUp(*desktop.MouseEvent)   {}
func (r *row) Tapped(*fyne.PointEvent)       {} // absorbs tap so List doesn't reselect
func (r *row) DoubleTapped(*fyne.PointEvent) { r.p.moveTo(r.id); r.p.open() }

// keyList is the detailed view.
type keyList struct {
	widget.List
	p *pane
}

func newKeyList(p *pane) *keyList {
	l := &keyList{p: p}
	l.Length = func() int { return len(p.entries) }
	l.CreateItem = func() fyne.CanvasObject { return newRowWidget(p, true) }
	l.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) { p.fillRow(i, o.(*row)) }
	l.OnSelected = func(i widget.ListItemID) { p.cursor = i; p.updateStatus() }
	l.ExtendBaseWidget(l)
	return l
}

func (l *keyList) AcceptsTab() bool          { return true }
func (l *keyList) TypedKey(e *fyne.KeyEvent) { l.p.ui.key(l.p, e, 20) }
func (l *keyList) TypedRune(r rune)          { l.p.typeSearch(r) }

// keyGrid is the brief view.
type keyGrid struct {
	widget.GridWrap
	p    *pane
	cell fyne.Size
	rows int
	cols int
}

func newKeyGrid(p *pane) *keyGrid {
	g := &keyGrid{p: p, rows: 1, cols: 1}
	g.Length = func() int {
		if len(p.entries) == 0 {
			return 0
		}
		page := (len(p.entries) - 1) / (g.rows * g.cols)
		return page*g.rows*g.cols + min(g.rows, len(p.entries)-page*g.rows*g.cols)*g.cols
	}
	g.CreateItem = func() fyne.CanvasObject {
		r := newRowWidget(p, false)
		if !g.cell.IsZero() {
			r.content = container.NewGridWrap(g.cell, r.content.(*fyne.Container).Objects[0])
		}
		return r
	}
	g.UpdateItem = func(i widget.GridWrapItemID, o fyne.CanvasObject) {
		r := o.(*row)
		c := r.content.(*fyne.Container)
		c.Layout = layout.NewGridWrapLayout(g.cell)
		c.Refresh()
		id := g.entryID(i)
		if id >= len(p.entries) {
			r.id = -1
			r.Hide()
			return
		}
		r.Show()
		p.fillRow(id, r)
	}
	g.OnSelected = func(i widget.GridWrapItemID) {
		if id := g.entryID(i); id < len(p.entries) {
			p.cursor = id
			p.updateStatus()
		} else {
			g.Select(p.cursor)
		}
	}
	g.ExtendBaseWidget(g)
	return g
}

func (g *keyGrid) AcceptsTab() bool          { return true }
func (g *keyGrid) TypedKey(e *fyne.KeyEvent) { g.p.ui.key(g.p, e, g.rows*g.cols) }
func (g *keyGrid) TypedRune(r rune)          { g.p.typeSearch(r) }

// GridWrap keeps native scrolling and row-major slots; entries fill each column first.
func (g *keyGrid) slot(id int) int {
	page, offset := id/(g.rows*g.cols), id%(g.rows*g.cols)
	return page*g.rows*g.cols + offset%g.rows*g.cols + offset/g.rows
}

func (g *keyGrid) entryID(slot int) int {
	page, offset := slot/(g.rows*g.cols), slot%(g.rows*g.cols)
	return page*g.rows*g.cols + offset%g.cols*g.rows + offset/g.cols
}

func (g *keyGrid) Select(id int)      { g.GridWrap.Select(g.slot(id)) }
func (g *keyGrid) Highlight(id int)   { g.GridWrap.Highlight(g.slot(id)) }
func (g *keyGrid) RefreshItem(id int) { g.GridWrap.RefreshItem(g.slot(id)) }

// Expanded cells must not prevent the pane from shrinking again.
func (g *keyGrid) MinSize() fyne.Size { return newRowWidget(g.p, false).MinSize() }

func (g *keyGrid) Resize(size fyne.Size) {
	g.GridWrap.Resize(size)
	g.Refresh()
}

func (g *keyGrid) Refresh() {
	cell := newRowWidget(g.p, false).MinSize()
	pad := g.Theme().Size(theme.SizeNamePadding)
	size := g.Size()
	for _, e := range g.p.entries {
		width := fyne.MeasureText(e.name, theme.TextSize(), fyne.TextStyle{}).Width
		cell.Width = max(cell.Width, width+2*theme.InnerPadding()+theme.IconInlineSize()+pad)
	}
	// ponytail: equal-width columns use longest visible name; names wider than pane truncate.
	cell.Width = min(cell.Width, max(g.MinSize().Width, size.Width))
	rows := max(1, int((size.Height+pad)/(cell.Height+pad)))
	available := max(1, int((size.Width+pad)/(cell.Width+pad)))
	cols := min(available, max(1, (len(g.p.entries)+rows-1)/rows))
	changed := rows != g.rows || cols != g.cols
	g.rows, g.cols = rows, cols
	// Keep native grid layout and scrolling; widen cells instead of adding empty columns.
	cell.Width = max(cell.Width, (size.Width+pad)/float32(cols)-pad-0.001)
	if cell != g.cell {
		g.cell = cell
		g.GridWrap.Refresh()
		// GridWrap caches column count separately from its item size.
		g.GridWrap.Resize(size)
	}
	g.GridWrap.Refresh()
	if changed && g.p.ui.active == g.p && len(g.p.entries) > 0 {
		g.Select(g.p.cursor)
		g.Highlight(g.p.cursor)
	}
}

// cols: visible columns in brief view, 1 in detailed.
func (p *pane) cols() int {
	if p.brief {
		return max(1, p.grid.ColumnCount())
	}
	return 1
}
