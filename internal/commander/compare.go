package commander

import (
	"fmt"
	"fyne.io/fyne/v2/lang"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dualcommander/dualcommander/pkg/desktop"
)

func (u *ui) comparisonSelected(folders bool) bool {
	for _, p := range []*pane{u.left, u.right} {
		e := p.current()
		if p.inArchive() || e.name == "" || e.name == ".." || e.dir != folders {
			return false
		}
	}
	return true
}

func (u *ui) updateCompareMenu() {
	if u.compareFilesItem == nil {
		return
	}
	filesDisabled, foldersDisabled := u.compareFilesItem.Disabled, u.compareFoldersItem.Disabled
	u.compareFilesItem.Disabled = u.passwordWindow != nil || !u.comparisonSelected(false) || strings.TrimSpace(u.app.Preferences().String(prefCompareFilesCmd)) == ""
	u.compareFoldersItem.Disabled = u.passwordWindow != nil || !u.comparisonSelected(true) || strings.TrimSpace(u.app.Preferences().String(prefCompareFoldersCmd)) == ""
	if (filesDisabled != u.compareFilesItem.Disabled || foldersDisabled != u.compareFoldersItem.Disabled) && u.win.MainMenu() != nil {
		u.win.MainMenu().Refresh()
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
	}
	if !u.comparisonSelected(folders) {
		if folders {
			return nil, fmt.Errorf(lang.L("select one folder under the cursor in each pane"))
		}
		return nil, fmt.Errorf(lang.L("select one file under the cursor in each pane"))
	}
	left = filepath.Join(left, u.left.current().name)
	right = filepath.Join(right, u.right.current().name)
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
