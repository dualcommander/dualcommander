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
	if e.Button != desktop.MouseButtonPrimary || r.id < 0 || r.id >= len(r.p.entries) || r.p.ui.passwordWindow != nil {
		return
	}
	r.p.click(r.id, e.Modifier)
	// Select before activation: restoring the old cursor can scroll and recycle this row.
	r.p.ui.activate(r.p)
}
func (r *row) MouseUp(*desktop.MouseEvent)   {}
func (r *row) Tapped(*fyne.PointEvent)       {} // absorbs tap so List doesn't reselect
func (r *row) DoubleTapped(*fyne.PointEvent) { r.p.moveTo(r.id); r.p.open() }

func (r *row) TappedSecondary(e *fyne.PointEvent) {
	if r.p.ui.passwordWindow != nil || r.id < 0 || r.id >= len(r.p.entries) {
		return
	}
	r.p.ui.activate(r.p)
	r.p.moveTo(r.id)
	r.p.showContextMenu(e.AbsolutePosition)
}

// keyList is the detailed view.
type keyList struct {
	widget.List
	p         *pane
	rowHeight float32
}

func newKeyList(p *pane) *keyList {
	l := &keyList{p: p, rowHeight: newRowWidget(p, true).MinSize().Height}
	l.Length = func() int { return len(p.entries) }
	l.CreateItem = func() fyne.CanvasObject { return newRowWidget(p, true) }
	l.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) { p.fillRow(i, o.(*row)) }
	l.OnSelected = func(i widget.ListItemID) {
		if p.cursor != i {
			p.cursor = i
			p.updateStatus()
		}
	}
	l.ExtendBaseWidget(l)
	return l
}

func (l *keyList) AcceptsTab() bool          { return true }
func (l *keyList) TypedKey(e *fyne.KeyEvent) { l.p.ui.key(l.p, e, 20) }
func (l *keyList) TypedRune(r rune)          { l.p.typeSearch(r) }

// Rows handle item clicks; view background only restores the pane's cursor.
func (l *keyList) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary && l.p.ui.passwordWindow == nil {
		l.p.ui.activate(l.p)
	}
}
func (l *keyList) MouseUp(*desktop.MouseEvent) {}
func (l *keyList) Tapped(*fyne.PointEvent) {
	l.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
}

// keyGrid fills columns top-to-bottom and scrolls horizontally.
type keyGrid struct {
	widget.Table
	p            *pane
	cell         fyne.Size
	rows, cols   int
	scroll       *container.Scroll
	markersReady bool
}

func newKeyGrid(p *pane) *keyGrid {
	g := &keyGrid{p: p, rows: 1, cols: 1}
	g.Length = func() (int, int) {
		return min(g.rows, len(p.entries)), (len(p.entries) + g.rows - 1) / g.rows
	}
	g.CreateCell = func() fyne.CanvasObject {
		r := newRowWidget(p, false)
		if !g.cell.IsZero() {
			r.content = container.NewGridWrap(g.cell, r.content.(*fyne.Container).Objects[0])
		}
		return r
	}
	g.UpdateCell = func(id widget.TableCellID, o fyne.CanvasObject) {
		r := o.(*row)
		c := r.content.(*fyne.Container)
		// ponytail: refresh geometry only on resize; Fyne recursively reshapes every
		// label on Container.Refresh, including unchanged cells during selection.
		if c.Layout.MinSize(c.Objects) != g.cell {
			c.Layout = layout.NewGridWrapLayout(g.cell)
			c.Refresh()
		}
		index := g.entryID(id)
		if index >= len(p.entries) {
			r.id = -1
			r.Hide()
			return
		}
		r.Show()
		p.fillRow(index, r)
	}
	g.OnSelected = func(id widget.TableCellID) {
		if index := g.entryID(id); index < len(p.entries) {
			if p.cursor != index {
				p.cursor = index
				p.updateStatus()
			}
		} else {
			g.Select(p.cursor)
		}
	}
	g.OnHighlighted = func(id widget.TableCellID) {
		if g.entryID(id) >= len(p.entries) {
			g.Table.MouseOut()
		}
	}
	// Fyne also hides selection and hover markers when HideSeparators is true.
	g.ExtendBaseWidget(g)
	return g
}

type briefScroll struct{ *container.Scroll }

func (s *briefScroll) Scrolled(e *fyne.ScrollEvent) {
	event := *e
	if event.Scrolled.DX == 0 {
		event.Scrolled.DX = event.Scrolled.DY
	}
	event.Scrolled.DY = 0
	s.Scroll.Scrolled(&event)
}

func (g *keyGrid) CreateRenderer() fyne.WidgetRenderer {
	r := g.Table.CreateRenderer()
	for i, o := range r.Objects() {
		if scroll, ok := o.(*container.Scroll); ok {
			g.scroll = scroll
			scroll.Direction = container.ScrollHorizontalOnly
			// ponytail: wrap Fyne's inner scroller until horizontal-only scrolling maps wheel input itself.
			r.Objects()[i] = &briefScroll{scroll}
		}
	}
	return r
}

func (g *keyGrid) AcceptsTab() bool { return true }

func (g *keyGrid) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary && g.p.ui.passwordWindow == nil {
		g.p.ui.activate(g.p)
	}
}
func (g *keyGrid) Tapped(*fyne.PointEvent) {
	g.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
}

// Keyboard navigation uses our selection marker; Fyne's focused-table hover
// otherwise stays pinned to currentHighlight instead of following the mouse.
func (g *keyGrid) FocusGained()              {}
func (g *keyGrid) TypedKey(e *fyne.KeyEvent) { g.p.ui.key(g.p, e, g.rows*g.cols) }
func (g *keyGrid) TypedRune(r rune)          { g.p.typeSearch(r) }

func (g *keyGrid) slot(id int) widget.TableCellID {
	return widget.TableCellID{Row: id % g.rows, Col: id / g.rows}
}

func (g *keyGrid) entryID(id widget.TableCellID) int { return id.Col*g.rows + id.Row }
func (g *keyGrid) Select(id int)                     { g.Table.Select(g.slot(id)) }
func (g *keyGrid) Highlight(id int)                  { g.Table.Highlight(g.slot(id)) }
func (g *keyGrid) RefreshItem(id int)                { g.Table.RefreshItem(g.slot(id)) }

// Expanded cells must not prevent the pane from shrinking again.
func (g *keyGrid) MinSize() fyne.Size { return newRowWidget(g.p, false).MinSize() }

func (g *keyGrid) Resize(size fyne.Size) {
	g.BaseWidget.Resize(size)
	g.Refresh()
}

func (g *keyGrid) Refresh() {
	cell := g.MinSize()
	pad := g.Theme().Size(theme.SizeNamePadding)
	size := g.Size()
	limit := max(cell.Width, (size.Width-pad)/2)
	for _, e := range g.p.entries {
		if cell.Width >= limit {
			break
		}
		width := fyne.MeasureText(e.name, theme.TextSize(), fyne.TextStyle{}).Width
		cell.Width = max(cell.Width, width+2*theme.InnerPadding()+theme.IconInlineSize()+pad)
	}
	// ponytail: equal-width columns; truncate long names to keep two columns when space permits.
	cell.Width = min(cell.Width, limit)
	rows := max(1, int((size.Height+pad)/(cell.Height+pad)))
	available := max(1, int((size.Width+pad)/(cell.Width+pad)))
	cols := min(available, max(1, (len(g.p.entries)+rows-1)/rows))
	cell.Width = max(cell.Width, (size.Width+pad)/float32(cols)-pad-0.001)
	changed := rows != g.rows || cols != g.cols || cell != g.cell
	g.rows, g.cols = rows, cols
	g.cell = cell
	// ponytail: Fyne 2.8 attaches markers only after creating a divider.
	// Seed once after layout, then restore real data; remove after upstream fix.
	if !g.markersReady && size.Height >= 2*cell.Height+pad {
		length := g.Length
		g.Length = func() (int, int) { return 2, 1 }
		g.Table.Refresh()
		g.Length = length
		g.markersReady = true
	}
	g.Table.Refresh()
	if changed && g.p.ui.active == g.p && len(g.p.entries) > 0 {
		g.Select(g.p.cursor)
		g.Highlight(g.p.cursor)
	}
}

// cols: visible columns in brief view, 1 in detailed.
func (p *pane) cols() int {
	if p.brief {
		return p.grid.cols
	}
	return 1
}
