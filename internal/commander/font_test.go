package commander

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestFontSizeMenuAndPersistence(t *testing.T) {
	root := fixture(t)
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), root)
	u.right.setBrief(true)
	cursorTo(t, u.left, "readme.md")
	u.left.marked["readme.md"] = true
	base := theme.TextSize()
	previous := float32(0)
	if len(fontSizes) != 7 || u.fontSize != "medium" {
		t.Fatal("expected seven presets with medium default")
	}
	for i, size := range fontSizes {
		u.setDensity("standard")
		u.fontItems[i].Action()
		height := newRowWidget(u.left, true).MinSize().Height
		if height <= previous || u.fontSize != size || a.Preferences().String(prefFontSize) != size {
			t.Fatalf("preset %q: height=%v previous=%v", size, height, previous)
		}
		previous = height
		for j, item := range u.fontItems {
			if item.Label != fontSizes[j] {
				t.Fatal("font menu label differs from t-shirt size")
			}
			if item.Checked != (i == j) {
				t.Fatal("font menu not exclusive")
			}
		}
		if u.left.current().name != "readme.md" || !u.left.marked["readme.md"] || !u.right.brief || u.win.Canvas().Focused() != u.left.focusable() {
			t.Fatal("font change lost pane state or focus")
		}
		want := theme.TextSize()
		u.setTheme("dark")
		u.setDensity("compact")
		if theme.TextSize() != want {
			t.Fatal("appearance/density reset font size")
		}
		if size == "medium" && want != base {
			t.Fatal("default changed existing font size")
		}
		u = newUI(a)
		if u.fontSize != size || theme.TextSize() != want || !u.fontItems[i].Checked {
			t.Fatal("font preset not restored")
		}
		cursorTo(t, u.left, "readme.md")
		u.left.marked["readme.md"] = true
	}
	a.Preferences().SetString(prefFontSize, "invalid")
	u = newUI(a)
	if u.fontSize != "medium" || theme.TextSize() != base {
		t.Fatal("invalid preference did not fall back to default")
	}
	for old, size := range map[string]string{"very small": "x-small", "small": "small", "default": "medium", "large": "large", "very large": "x-large"} {
		a.Preferences().SetString(prefFontSize, old)
		u = newUI(a)
		if u.fontSize != size || a.Preferences().String(prefFontSize) != size {
			t.Fatalf("saved preset %q not migrated to %q", old, size)
		}
	}
}

func TestFontSizeRenderGolden(t *testing.T) {
	root := fixture(t)
	for _, size := range fontSizes {
		t.Run(size, func(t *testing.T) {
			a := test.NewTempApp(t)
			a.Preferences().SetString(prefFontSize, size)
			u := newTestUI(t, a, filepath.Join(root, "src"), root)
			u.setTheme("light")
			test.ApplyTheme(t, u.app.Settings().Theme())
			u.left.entries[0].attrs, u.left.entries[0].mod = "drwxr-xr-x", u.left.entries[1].mod
			cursorTo(t, u.left, "readme.md")
			// Isolate the render from the main window's minimum size and focus layout.
			w := u.win
			w.SetContent(u.left.body)
			w.Resize(fyne.NewSize(950, 300))
			w.Canvas().Capture()
			u.left.setDensity()
			w.Resize(fyne.NewSize(950, 300))
			w.Content().Refresh()
			u.left.list.Refresh()
			golden := iconGolden("font_" + size + ".png")
			if !assertNativeRender(t, golden, w.Canvas()) && os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.Rename(filepath.Join("testdata", "failed", golden), filepath.Join("testdata", golden)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
