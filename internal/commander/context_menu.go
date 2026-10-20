package commander

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/desktop"
)

func (p *pane) keyboardContextMenu() {
	if p.ui.passwordWindow != nil || p.cursor < 0 || p.cursor >= len(p.entries) {
		return
	}
	p.showCursor() // Scroll cursor into view before positioning menu.
	var view fyne.CanvasObject = p.list
	pad := p.list.Theme().Size(theme.SizeNamePadding)
	position := fyne.NewPos(0, float32(p.cursor)*(p.list.rowHeight+pad)-p.list.GetScrollOffset()+p.list.rowHeight)
	if p.brief {
		g := p.grid
		view = g
		pad = g.Theme().Size(theme.SizeNamePadding)
		position = fyne.NewPos(float32(p.cursor/g.rows)*(g.cell.Width+pad), float32(p.cursor%g.rows)*(g.cell.Height+pad)+g.cell.Height)
		if g.scroll != nil {
			position.X -= g.scroll.Offset.X
		}
	}
	position.X = max(0, min(position.X, view.Size().Width))
	position.Y = max(0, min(position.Y, view.Size().Height))
	p.showContextMenu(p.ui.app.Driver().AbsolutePositionForObject(view).Add(position))
}

func (u *ui) chooseApplication(path string) {
	window, ok := u.win.(driver.NativeWindow)
	if !ok {
		return
	}
	window.RunNative(func(context any) {
		mac, ok := context.(driver.MacWindowContext)
		if ok && mac.NSWindow != 0 {
			u.openWith(path, func() string { return desktop.ChooseApplication(mac.NSWindow) })
		}
	})
}

func (u *ui) openWith(path string, choose func() string) {
	unblock := u.blockParent(u.win)
	go func() {
		application := choose()
		post(func() {
			unblock()
			if application != "" {
				u.startExternal(exec.Command("open", "-a", application, "--", path), nil)
			}
		})
	}()
}

func (p *pane) showContextMenu(position fyne.Position) {
	menu := p.contextMenu()
	if window, ok := p.ui.win.(driver.NativeWindow); ok && runtime.GOOS == "darwin" {
		window.RunNative(func(context any) {
			mac, ok := context.(driver.MacWindowContext)
			if !ok || mac.NSWindow == 0 {
				return
			}
			items := make([]desktop.ContextItem, len(menu.Items))
			for i, item := range menu.Items {
				items[i] = desktop.ContextItem{Title: item.Label, Disabled: item.Disabled}
			}
			scale := p.ui.win.Canvas().Scale()
			go func() {
				index := desktop.ContextMenu(mac.NSWindow, items, position.X*scale, position.Y*scale)
				post(func() {
					if index >= 0 && index < len(menu.Items) && !menu.Items[index].Disabled {
						menu.Items[index].Action()
					}
				})
			}()
		})
		return
	}
	// ponytail: Fyne popup remains fallback outside macOS and in headless tests.
	widget.ShowPopUpMenuAtPosition(menu, p.ui.win.Canvas(), position)
}

// Context actions target the clicked item, not unrelated marked files.
func (p *pane) contextMenu() *fyne.Menu {
	e := p.current()
	path := filepath.Join(p.path, e.name)
	valid := e.name != "" && e.name != ".."
	native := valid && !p.inArchive()
	item := func(label string, enabled bool, action func()) *fyne.MenuItem {
		it := fyne.NewMenuItem(label, func() {
			if enabled && p.ui.passwordWindow == nil && p.path == filepath.Dir(path) {
				p.ui.activate(p)
				action()
			}
		})
		it.Disabled = !enabled
		return it
	}
	open := item("Open", valid, func() {
		for i, current := range p.entries {
			if current.name == e.name {
				p.moveTo(i)
				p.open()
				return
			}
		}
	})
	items := []*fyne.MenuItem{open}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		launch := func(action string) func() {
			return func() {
				if runtime.GOOS == "darwin" && action == "open-with" {
					p.ui.chooseApplication(path)
					return
				}
				cmd, err := desktop.ItemCommand(runtime.GOOS, action, path)
				p.ui.startExternal(cmd, err)
			}
		}
		label := "Properties"
		if runtime.GOOS == "darwin" {
			label = "Get Info"
		}
		items = append(items, item("Open With…", native && (!e.dir || runtime.GOOS == "darwin"), launch("open-with")),
			item(label, native, launch("properties")))
	}
	items = append(items, fyne.NewMenuItemSeparator(), item(revealLabel(), native, func() {
		reveal, _ := desktop.Defaults(runtime.GOOS)
		cmd, err := desktop.Command(reveal, path, p.path)
		p.ui.startExternal(cmd, err)
	}), item("Copy Path", native, func() { p.ui.win.Clipboard().SetContent(path) }))
	return fyne.NewMenu("", items...)
}
