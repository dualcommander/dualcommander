package commander

import (
	"fmt"
	"fyne.io/fyne/v2/lang"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/dualcommander/dualcommander/pkg/fsops"
)

const prefFavourites = "favourites"

func (u *ui) toggleFavourite(path string) {
	paths := slices.Clone(u.app.Preferences().StringList(prefFavourites))
	if i := slices.Index(paths, path); i >= 0 {
		paths = slices.Delete(paths, i, i+1)
	} else {
		paths = append(paths, path)
	}
	u.app.Preferences().SetStringList(prefFavourites, paths)
	u.updateStarredMenu()
	for _, p := range []*pane{u.left, u.right} {
		if p != nil {
			p.updateFavourite()
		}
	}
}

func (u *ui) updateStarredMenu() {
	if u.starredItem == nil {
		return
	}
	var items []*fyne.MenuItem
	for _, path := range u.app.Preferences().StringList(prefFavourites) {
		items = append(items, fyne.NewMenuItem(path, func() {
			if u.passwordWindow != nil {
				return
			}
			u.showErr(u.active.cd(path, ""))
			u.activate(u.active)
		}))
	}
	if len(items) == 0 {
		empty := fyne.NewMenuItem(lang.L("No starred paths"), nil)
		empty.Disabled = true
		items = append(items, empty)
	}
	u.starredItem.ChildMenu = fyne.NewMenu("", items...)
	if menu := u.win.MainMenu(); menu != nil {
		menu.Refresh()
	}
}

func (p *pane) updateFavourite() {
	p.favourite.SetText("☆")
	if slices.Contains(p.ui.app.Preferences().StringList(prefFavourites), p.path) {
		p.favourite.SetText("★")
	}
	// ponytail: favourites target real folders only; archive bookmarks can follow if requested.
	if p.inArchive() {
		p.favourite.Disable()
	} else {
		p.favourite.Enable()
	}
}

func (p *pane) showFavourites() {
	pop := widget.NewPopUp(container.NewVBox(), p.ui.win.Canvas())
	var rebuild func()
	rebuild = func() {
		rows := container.NewVBox()
		for _, path := range p.ui.app.Preferences().StringList(prefFavourites) {
			open := widget.NewButton(filepath.Base(path), func() {
				info, err := os.Stat(path)
				if err == nil && !info.IsDir() {
					err = fmt.Errorf(lang.L("not a folder: %s"), path)
				}
				if err == nil {
					err = p.cd(path, "")
				}
				if err != nil {
					pop.Hide()
					p.ui.showErr(err)
					return
				}
				pop.Hide()
				p.ui.activate(p)
			})
			open.Alignment = widget.ButtonAlignLeading
			label := widget.NewLabel(path)
			label.Truncation = fyne.TextTruncateEllipsis
			remove := widget.NewButton("★", func() { p.ui.toggleFavourite(path); rebuild() })
			rows.Add(container.NewBorder(nil, nil, nil, remove, container.NewBorder(nil, nil, open, nil, label)))
		}
		if len(rows.Objects) == 0 {
			rows.Add(widget.NewLabel(lang.L("No favourites. Star a folder to add it.")))
		}
		scroll := container.NewVScroll(rows)
		scroll.SetMinSize(fyne.NewSize(min(480, p.ui.win.Canvas().Size().Width-32), min(280, rows.MinSize().Height)))
		pop.Content = scroll
		pop.Refresh()
		pop.Resize(scroll.MinSize())
		pop.ShowAtRelativePosition(fyne.NewPos(p.favourites.Size().Width-pop.Size().Width, p.favourites.Size().Height), p.favourites)
	}
	rebuild()
}

func (p *pane) updateSpace() {
	p.spaceGeneration++
	version := p.spaceGeneration
	path := p.path
	if p.inArchive() {
		path = filepath.Dir(p.archiveRoot)
	}
	p.setSpace(0, 0, fmt.Errorf("loading"))
	load := p.mounts
	go func() {
		used, total, err := fsops.Space(path)
		paths, _ := load()
		mount := currentMount(path, paths)
		post(func() {
			if p.spaceGeneration == version {
				p.setSpace(used, total, err)
				p.mountButton.SetText(mount)
			}
		})
	}()
}

func currentMount(path string, mounts []string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	root := filepath.VolumeName(path) + string(filepath.Separator)
	longest := len(root)
	for _, mount := range mounts {
		mount = filepath.Clean(mount)
		resolved := mount
		if target, err := filepath.EvalSymlinks(mount); err == nil {
			resolved = target
		}
		rel, err := filepath.Rel(resolved, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && len(resolved) > longest {
			root = mount
			longest = len(resolved)
		}
	}
	return root
}

func (p *pane) setSpace(used, total uint64, err error) {
	text := lang.L("Disk usage unavailable")
	if err == nil && total > 0 && used <= total {
		text = fmt.Sprintf(lang.L("%s used / %s total"), humanSize(int64(used)), humanSize(int64(total)))
	}
	p.space.SetText(text)
}

func (p *pane) showMounts() {
	p.mountButton.Disable()
	load := p.mounts
	go func() {
		paths, err := load()
		post(func() {
			p.mountButton.Enable()
			if err != nil {
				p.ui.showErr(err)
				return
			}
			rows := container.NewVBox()
			pop := widget.NewPopUp(rows, p.ui.win.Canvas())
			for _, path := range paths {
				button := widget.NewButton(path, func() {
					pop.Hide()
					if p.cd(path, "") == nil {
						p.ui.activate(p)
					}
				})
				button.Alignment = widget.ButtonAlignLeading
				rows.Add(button)
			}
			if len(paths) == 0 {
				rows.Add(widget.NewLabel(lang.L("No mounted volumes")))
			}
			scroll := container.NewVScroll(rows)
			scroll.SetMinSize(fyne.NewSize(min(480, p.ui.win.Canvas().Size().Width-32), min(280, rows.MinSize().Height)))
			pop.Content = scroll
			pop.ShowAtRelativePosition(fyne.NewPos(0, p.mountButton.Size().Height), p.mountButton)
		})
	}()
}
