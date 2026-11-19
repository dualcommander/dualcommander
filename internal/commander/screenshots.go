//go:build screenshots

package commander

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

// CaptureManual renders the manual's Settings figures into dir.
// It uses Fyne's headless test driver, so no display is needed; fonts are Fyne's
// bundled ones, not the macOS system look.
func CaptureManual(dir string) error {
	root, err := os.MkdirTemp("", "dualcommander-screenshots")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	for _, d := range []string{src, dst} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	for _, name := range []string{"IMG_0001.jpg", "IMG_0002.jpg", "IMG_0003.jpg"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte("x"), 0o644); err != nil {
			return err
		}
	}
	a := test.NewApp()
	a.Preferences().SetString(prefLeft, src)
	a.Preferences().SetString(prefRight, dst)
	u := newUI(a)
	u.win.Resize(fyne.NewSize(1200, 560))
	window := func(title string) (fyne.Window, error) {
		for _, w := range a.Driver().AllWindows() {
			if w.Title() == title {
				return w, nil
			}
		}
		return nil, fmt.Errorf("window %q not found", title)
	}
	write := func(name string, w fyne.Window) error {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			return err
		}
		err = png.Encode(f, w.Canvas().Capture())
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	}
	for _, mode := range []string{"light", "dark"} {
		u.setTheme(mode)
		u.settings()
		w, err := window("Settings")
		if err != nil {
			return err
		}
		// Size the window to the whole form so nothing needs scrolling.
		scroll := w.Content().(*fyne.Container).Objects[0].(*container.Scroll)
		w.Resize(fyne.NewSize(900, 800))
		w.Resize(fyne.NewSize(900, scroll.Content.MinSize().Height+8))
		name := "manual-settings"
		if mode == "dark" {
			name += "-dark"
		}
		err = write(name, w)
		w.Close()
		if err != nil {
			return err
		}
	}
	return captureMultiRename(a, u, window, write)
}

// walk visits o and its children through the renderers already on screen.
// test.LaidOutObjects is unsuitable here: it calls CreateRenderer, which resets
// List.scroller and leaves the visible list empty.
func walk(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(o)
	switch c := o.(type) {
	case *fyne.Container:
		for _, child := range c.Objects {
			walk(child, visit)
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(c).Objects() {
			walk(child, visit)
		}
	}
}

// captureMultiRename marks the sample JPGs and previews a three-digit counter.
func captureMultiRename(a fyne.App, u *ui, window func(string) (fyne.Window, error), write func(string, fyne.Window) error) error {
	for i, e := range u.left.entries {
		if strings.HasSuffix(e.name, ".jpg") {
			u.left.click(i, fyne.KeyModifierShortcutDefault)
		}
	}
	u.activate(u.left)
	// Hold the debounced preview so it runs once, with the final template.
	var preview func()
	u.renameAfter = func(_ time.Duration, f func()) func() { preview = f; return func() { preview = nil } }
	for _, mode := range []string{"light", "dark"} {
		u.setTheme(mode)
		preview = nil
		u.renameMulti()
		w, err := window("Multi-Rename")
		if err != nil {
			return err
		}
		var templates []*escEntry
		var list *dialogList
		walk(w.Content(), func(o fyne.CanvasObject) {
			switch c := o.(type) {
			case *dialogList:
				list = c
			case *escEntry:
				templates = append(templates, c)
			}
		})
		if len(templates) != 2 || list == nil {
			return fmt.Errorf("multi-rename: found %d templates, list %v", len(templates), list != nil)
		}
		templates[1].SetText("{{.name}}_{{index 3}}.{{.ext}}")
		if preview == nil {
			return fmt.Errorf("multi-rename: preview was not scheduled")
		}
		preview()
		list.Refresh()
		name := "manual-multi-rename"
		if mode == "dark" {
			name += "-dark"
		}
		err = write(name, w)
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
