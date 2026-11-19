package commander

import (
	"fmt"
	"fyne.io/fyne/v2/lang"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	for _, p := range []*pane{u.left, u.right} {
		if p != nil {
			p.updateFavourite()
		}
	}
}

// updateStarItem keeps Go > Star Folder in sync with the active pane's folder.
func (u *ui) updateStarItem() {
	if u.starItem == nil || u.active == nil {
		return
	}
	checked := slices.Contains(u.app.Preferences().StringList(prefFavourites), u.active.path)
	disabled := u.active.inArchive()
	if u.starItem.Checked == checked && u.starItem.Disabled == disabled {
		return
	}
	u.starItem.Checked, u.starItem.Disabled = checked, disabled
	if menu := u.win.MainMenu(); menu != nil {
		menu.Refresh()
	}
}

func (p *pane) updateFavourite() {
	p.favourite.SetText("☆")
	if slices.Contains(p.ui.app.Preferences().StringList(prefFavourites), p.path) {
		p.favourite.SetText("★")
	}
	// favourites target real folders only.
	if p.inArchive() {
		p.favourite.Disable()
	} else {
		p.favourite.Enable()
	}
	p.ui.updateStarItem()
}

// favouriteButton takes digit keys so the numbered rows open from the keyboard.
type favouriteButton struct {
	widget.Button
	digit func(rune)
	close func()
}

func (b *favouriteButton) TypedRune(r rune) { b.digit(r) }

func (b *favouriteButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		b.close()
	} else {
		b.Button.TypedKey(e)
	}
}

func (p *pane) showFavourites() {
	pop := widget.NewPopUp(container.NewVBox(), p.ui.win.Canvas())
	closePop := func() {
		pop.Hide()
		p.ui.win.Canvas().Focus(p.focusable())
	}
	open := func(path string) {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			err = fmt.Errorf(lang.L("not a folder: %s"), path)
		}
		if err == nil {
			err = p.cd(path, "")
		}
		pop.Hide()
		if err != nil {
			p.ui.showErr(err)
			return
		}
		p.ui.activate(p)
	}
	var rebuild func()
	rebuild = func() {
		paths := p.ui.app.Preferences().StringList(prefFavourites)
		button := func(text string, tapped func()) *favouriteButton {
			b := &favouriteButton{close: closePop}
			b.Text, b.OnTapped = text, tapped
			b.digit = func(r rune) {
				if i := int(r - '1'); r >= '1' && r <= '9' && i < len(paths) {
					open(paths[i])
				}
			}
			b.ExtendBaseWidget(b)
			return b
		}
		rows := container.NewVBox()
		var first *favouriteButton
		for i, path := range paths {
			index := button(strconv.Itoa(i+1), func() { open(path) })
			index.Alignment = widget.ButtonAlignLeading
			if first == nil {
				first = index
			}
			label := widget.NewLabel(path)
			label.Truncation = fyne.TextTruncateEllipsis
			remove := button("★", func() { p.ui.toggleFavourite(path); rebuild() })
			rows.Add(container.NewBorder(nil, nil, nil, remove, container.NewBorder(nil, nil, index, nil, label)))
		}
		if first == nil {
			first = button(lang.L("No favourites. Star a folder to add it."), closePop)
			rows.Add(first)
		}
		scroll := container.NewVScroll(rows)
		scroll.SetMinSize(fyne.NewSize(min(480, p.ui.win.Canvas().Size().Width-32), min(280, rows.MinSize().Height)))
		pop.Content = scroll
		pop.Refresh()
		pop.Resize(scroll.MinSize())
		pop.ShowAtRelativePosition(fyne.NewPos(p.favourites.Size().Width-pop.Size().Width, p.favourites.Size().Height), p.favourites)
		p.ui.win.Canvas().Focus(first)
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
		format, size := lang.L("%s used / %s total"), used
		if p.ui.app.Preferences().BoolWithFallback(prefShowFreeSpace, true) {
			format, size = lang.L("%s free / %s total"), total-used
		}
		text = fmt.Sprintf(format, humanSize(int64(size)), humanSize(int64(total)))
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
