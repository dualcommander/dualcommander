package commander

import (
	"path/filepath"

	"fyne.io/fyne/v2"
)

func (u *ui) dropPane(position fyne.Position) *pane {
	for _, p := range []*pane{u.left, u.right} {
		origin := u.split.Position().Add(p.box.Position())
		size := p.box.Size()
		if position.X >= origin.X && position.Y >= origin.Y && position.X < origin.X+size.Width && position.Y < origin.Y+size.Height {
			return p
		}
	}
	return nil
}

func (u *ui) dropURIs(position fyne.Position, uris []fyne.URI) {
	if u.passwordWindow != nil {
		return
	}
	dst := u.dropPane(position)
	if dst == nil || dst.inArchive() || len(uris) == 0 {
		return
	}
	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		// Only native absolute file paths; never fetch remote URLs on drop.
		if uri == nil || uri.Scheme() != "file" || !filepath.IsAbs(uri.Path()) {
			return
		}
		paths = append(paths, filepath.Clean(uri.Path()))
	}
	// ponytail: dropping anywhere in a pane targets its current folder, not hovered subfolders.
	u.transferSelection(dst, dst, paths, false)
}
