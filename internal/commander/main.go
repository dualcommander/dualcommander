// Package commander implements a cross-platform two-pane file manager.
package commander

import (
	"image/color"
	"os/exec"
	"runtime"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
	"github.com/dualcommander/dualcommander/assets"
	"github.com/dualcommander/dualcommander/pkg/desktop"
	"github.com/dualcommander/dualcommander/pkg/fileicon"
)

const appName = "DualCommander"

// Keep the base version in sync with FyneApp.toml.
const appVersion = "0.1.0"

// ponytail: retain original app ID so existing preferences survive the rebrand.
const appID = "io.github.typekpb.gommander"

var appIcon = fyne.NewStaticResource("dualcommander.png", assets.IconPNG)

// appTheme: optional forced light/dark (force) + density paddings, on top of the default theme.
type appTheme struct {
	fyne.Theme
	v          fyne.ThemeVariant
	force      bool
	pad, inner float32
	fontScale  float32
}

func (t appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if t.force {
		v = t.v
	}
	if n == theme.ColorNameButton {
		if v == theme.VariantDark {
			return color.NRGBA{R: 62, G: 62, B: 66, A: 255}
		}
		return color.NRGBA{R: 238, G: 238, B: 240, A: 255}
	}
	return t.Theme.Color(n, v)
}

func (t appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius:
		return 6
	case theme.SizeNamePadding:
		return t.pad
	case theme.SizeNameInnerPadding:
		return t.inner
	case theme.SizeNameText, theme.SizeNameCaptionText, theme.SizeNameHeadingText, theme.SizeNameSubHeadingText, theme.SizeNameInlineIcon:
		return t.Theme.Size(n) * t.fontScale
	}
	return t.Theme.Size(n)
}

// Run starts DualCommander using native desktop integration.
func Run() {
	a := app.NewWithID(appID)
	desktop.SetAppIcon(assets.IconPNG)
	initIcons()
	u := newUI(a)
	u.filePreview = fileicon.Preview
	u.left.list.Refresh()
	u.left.grid.Refresh()
	u.right.list.Refresh()
	u.right.grid.Refresh()
	u.win.ShowAndRun()
}

// Theme modes; "system" follows OS appearance (incl. live changes).
var themeModes = []string{"system", "light", "dark"}

var densities = []string{"compact", "standard"}

var fontSizes = []string{"xx-small", "x-small", "small", "medium", "large", "x-large", "xx-large"}

// applyTheme sets appearance mode (system/light/dark) and density app-wide.
func applyTheme(a fyne.App, mode, density string) {
	t := appTheme{Theme: theme.DefaultTheme(), pad: 2, inner: 4, fontScale: 1}
	if i := slices.Index(fontSizes, a.Preferences().String(prefFontSize)); i >= 0 {
		t.fontScale = [...]float32{0.7, 0.8, 0.9, 1, 1.15, 1.3, 1.5}[i]
	}
	switch density {
	case "compact":
		t.pad, t.inner = 1, 2
	}
	switch mode {
	case "light":
		t.v, t.force = theme.VariantLight, true
	case "dark":
		t.v, t.force = theme.VariantDark, true
	}
	a.Settings().SetTheme(t)
}

// reveal shows path selected in the platform file manager.
// editCmd: OS default text editor (F4). No shell, so file names can't inject.
func editCmd(goos, path string) []string {
	switch goos {
	case "darwin":
		return []string{"open", "-t", path}
	case "windows":
		return []string{"notepad", path}
	}
	return []string{"xdg-open", path} // ponytail: Linux has no "default editor" API; add editorCmd pref when asked
}

func edit(path string) error {
	c := editCmd(runtime.GOOS, path)
	return exec.Command(c[0], c[1:]...).Start()
}
