package commander

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

type variantSettings struct {
	fyne.Settings
	variant fyne.ThemeVariant
}

func (s variantSettings) ThemeVariant() fyne.ThemeVariant { return s.variant }

type variantApp struct {
	fyne.App
	settings fyne.Settings
}

func (a variantApp) Settings() fyne.Settings { return a.settings }

func TestSettingsInfoIconThemeSwitches(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(string(rune('0'+variant)), func(t *testing.T) {
			base := test.NewTempApp(t)
			a := variantApp{App: base, settings: variantSettings{base.Settings(), variant}}
			fyne.SetCurrentApp(a)
			root := fixture(t)
			u := newTestUI(t, a, root, root)
			u.settings()
			w := windowByTitle(t, a, "Settings")
			var icons []*dialogInfoIcon
			var choice *dialogSelect
			for _, object := range test.LaidOutObjects(w.Content()) {
				if i, ok := object.(*dialogInfoIcon); ok {
					icons = append(icons, i)
				}
				if s, ok := object.(*dialogSelect); ok && choice == nil {
					choice = s
				}
			}
			if len(icons) != 2 || choice == nil {
				t.Fatal("missing settings guidance")
			}
			for _, mode := range []string{"light", "dark", "system", "dark", "light", "system"} {
				previous := make([]fyne.Resource, len(icons))
				for n, icon := range icons {
					previous[n] = icon.Resource
				}
				choice.SetSelected(mode)
				v := variant
				if mode == "light" {
					v = theme.VariantLight
				}
				if mode == "dark" {
					v = theme.VariantDark
				}
				want := color.NRGBAModel.Convert(theme.DefaultTheme().Color(theme.ColorNameForeground, v))
				for n, icon := range icons {
					if previous[n] == icon.Resource {
						t.Fatal("icon resource not invalidated")
					}
					res, ok := icon.Resource.(fyne.ThemedResource)
					if !ok || res.ThemeColorName() != theme.ColorNameForeground {
						t.Fatal("icon must use readable foreground tint")
					}
					got := color.NRGBAModel.Convert(icon.Theme().Color(res.ThemeColorName(), variant))
					if got != want {
						t.Fatalf("%s icon color %v, want %v", mode, got, want)
					}
					raster := test.WidgetRenderer(icon).Objects()[0].(*canvas.Image)
					if raster.Resource != icon.Resource {
						t.Fatal("renderer retained stale icon resource")
					}
				}
			}
			w.Close()
		})
	}
}
