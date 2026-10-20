package commander

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/dualcommander/dualcommander/pkg/fileicon"
)

// Material icons remain the fallback where system icons are unavailable.
var icUp, icDir, icFile, icImage, icAudio, icVideo, icText, icArchive, icApp fyne.Resource

var extIcons = map[string]fyne.Resource{}

// initIcons must run after the app is created (theme lookups need it).
func initIcons() {
	icUp = theme.NewColoredResource(theme.MoveUpIcon(), theme.ColorNamePrimary)
	icDir = theme.NewColoredResource(theme.FolderIcon(), theme.ColorNameWarning)
	icFile = theme.FileIcon()
	icImage = theme.NewColoredResource(theme.FileImageIcon(), theme.ColorNameSuccess)
	icAudio = theme.NewColoredResource(theme.FileAudioIcon(), theme.ColorNamePrimary)
	icVideo = theme.NewColoredResource(theme.FileVideoIcon(), theme.ColorNameError)
	icText = theme.NewColoredResource(theme.FileTextIcon(), theme.ColorNamePrimary)
	icArchive = theme.NewColoredResource(theme.StorageIcon(), theme.ColorNameWarning)
	icApp = theme.NewColoredResource(theme.FileApplicationIcon(), theme.ColorNameSuccess)
	native := func(extension string, folder bool, fallback fyne.Resource) fyne.Resource {
		if data := fileicon.PNG(extension, folder); len(data) > 0 {
			return fyne.NewStaticResource("system-"+extension+".png", data)
		}
		return fallback
	}
	icDir = native("folder", true, icDir)
	icFile = native("", false, icFile)
	for res, exts := range map[fyne.Resource]string{
		icImage:   "png jpg jpeg gif bmp svg webp ico tiff heic",
		icAudio:   "mp3 wav flac ogg m4a aac",
		icVideo:   "mp4 mkv avi mov webm wmv",
		icText:    "txt md go js ts py java c h cpp rs json yaml yml toml xml html css sh log csv ini cfg",
		icArchive: "zip tar gz tgz bz2 xz 7z rar dmg iso",
		icApp:     "exe msi app bat cmd apk deb rpm",
	} {
		for _, e := range strings.Fields(exts) {
			extIcons["."+e] = native(e, false, res)
		}
	}
}

// ponytail: cache lives with the directory listing; refresh reloads previews so
// changed files cannot retain stale thumbnails. No persistent thumbnail cache.
var previewWorkers = make(chan struct{}, 2)

func (p *pane) entryIcon(i int) fyne.Resource {
	e := &p.entries[i]
	if e.icon != nil {
		return e.icon
	}
	if !e.iconRequested && !e.dir && e.name != ".." && p.ui.filePreview != nil && !p.inArchive() && p.previewVisible(i) {
		e.iconRequested = true
		name := filepath.Join(p.path, e.name)
		generation := p.iconGeneration
		load := p.ui.filePreview
		go func() {
			previewWorkers <- struct{}{}
			post(func() {
				if p.iconGeneration != generation {
					<-previewWorkers
					return
				}
				for i := range p.entries {
					if p.entries[i].name == filepath.Base(name) {
						if !p.previewVisible(i) {
							p.entries[i].iconRequested = false
							<-previewWorkers
							return
						}
						go func() {
							data := load(name)
							<-previewWorkers
							post(func() {
								if p.iconGeneration != generation || len(data) == 0 {
									return
								}
								// Sorting may move rows while a preview loads.
								for i := range p.entries {
									if p.entries[i].name == filepath.Base(name) {
										p.entries[i].icon = fyne.NewStaticResource(fmt.Sprintf("preview-%s-%d.png", name, generation), data)
										p.list.RefreshItem(i)
										p.grid.RefreshItem(i)
										break
									}
								}
							})
						}()
						return
					}
				}
				<-previewWorkers
			})
		}()
	}
	return iconFor(*e)
}

// Fyne retains buffer rows outside the viewport; those must not trigger previews.
func (p *pane) previewVisible(i int) bool {
	height := p.list.rowHeight
	padding := p.list.Theme().Size(theme.SizeNamePadding)
	offset, size := p.list.GetScrollOffset(), p.list.Size()
	row := i
	if p.brief {
		g := p.grid
		if g.scroll == nil {
			return false
		}
		left := float32(i/g.rows) * (g.cell.Width + g.Theme().Size(theme.SizeNamePadding))
		return g.Size().Width > 0 && g.Size().Height > 0 &&
			left < g.scroll.Offset.X+g.Size().Width && left+g.cell.Width > g.scroll.Offset.X
	}
	top := float32(row) * (height + padding)
	return size.Width > 0 && size.Height > 0 && height > 0 && top < offset+size.Height && top+height > offset
}

func iconFor(e entry) fyne.Resource {
	switch {
	case e.name == "..":
		return icUp
	case e.dir:
		return icDir
	}
	if r, ok := extIcons[strings.ToLower(filepath.Ext(e.name))]; ok {
		return r
	}
	return icFile
}
