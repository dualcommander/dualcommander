package commander

import (
	"fmt"
	"fyne.io/fyne/v2/lang"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/dualcommander/dualcommander/pkg/desktop"
)

func (u *ui) updateCompareMenu() {
	if u.compareFilesItem == nil {
		return
	}
	for key, item := range map[string]*fyne.MenuItem{
		prefCompareFilesCmd: u.compareFilesItem, prefCompareFoldersCmd: u.compareFoldersItem,
	} {
		item.Disabled = u.passwordWindow != nil || strings.TrimSpace(u.app.Preferences().String(key)) == ""
	}
}

func (u *ui) comparisonCommand(folders bool) (*exec.Cmd, error) {
	if u.left.inArchive() || u.right.inArchive() {
		return nil, fmt.Errorf(lang.L("leave archive folders before comparing"))
	}
	key := prefCompareFilesCmd
	left, right := u.left.path, u.right.path
	if folders {
		key = prefCompareFoldersCmd
	} else {
		for _, p := range []*pane{u.left, u.right} {
			e := p.current()
			if e.name == "" || e.name == ".." || e.dir {
				return nil, fmt.Errorf(lang.L("select one file under the cursor in each pane"))
			}
		}
		left = filepath.Join(left, u.left.current().name)
		right = filepath.Join(right, u.right.current().name)
	}
	command := u.app.Preferences().String(key)
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf(lang.L("configure comparison command in Settings > External applications"))
	}
	return desktop.ComparisonCommand(command, left, right, u.left.path)
}

func (u *ui) compare(folders bool) {
	if u.passwordWindow != nil {
		return
	}
	cmd, err := u.comparisonCommand(folders)
	// Diff tools conventionally return 1 when differences exist, not a launch failure.
	u.startExternal(cmd, err, 1)
}
