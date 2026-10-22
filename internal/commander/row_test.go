package commander

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type measuringDriver struct {
	fyne.Driver
	measurements int
}

func (d *measuringDriver) RenderedTextSize(text string, size float32, style fyne.TextStyle, source fyne.Resource) (fyne.Size, float32) {
	d.measurements++
	return d.Driver.RenderedTextSize(text, size, style, source)
}

type measuringApp struct {
	fyne.App
	driver *measuringDriver
}

func (a *measuringApp) Driver() fyne.Driver { return a.driver }

func TestUnchangedRowDoesNotMeasureText(t *testing.T) {
	a := test.NewTempApp(t)
	d := &measuringDriver{Driver: a.Driver()}
	fyne.SetCurrentApp(&measuringApp{App: a, driver: d})
	t.Cleanup(func() { fyne.SetCurrentApp(a) })
	p := &pane{marked: map[string]bool{}, entries: []entry{{
		name: "readme.md", size: 42, attrs: "-rw-r--r--",
		mod: time.Date(2024, 1, 2, 3, 4, 0, 0, time.UTC), icon: theme.FileIcon(),
	}}}
	for _, detailed := range []bool{true, false} {
		r := newRowWidget(p, detailed)
		r.Resize(fyne.NewSize(600, 30))
		p.fillRow(0, r)
		d.measurements = 0
		p.fillRow(0, r)
		if d.measurements != 0 {
			t.Fatalf("detailed=%v: unchanged row measured text %d times", detailed, d.measurements)
		}
		p.marked["readme.md"] = true
		p.fillRow(0, r)
		assertUIText(t, r, "readme.md")
		content := r.content.(*fyne.Container)
		if !detailed {
			content = content.Objects[0].(*fyne.Container)
		}
		name := content.Objects[0].(*widget.Label)
		if name.Importance != widget.DangerImportance {
			t.Fatal("mark did not update row styling")
		}
		p.marked = map[string]bool{}
		p.fillRow(0, r)
		if name.Importance != widget.MediumImportance {
			t.Fatal("unmark did not restore row styling")
		}
		if detailed {
			assertUIText(t, r, "42 B", "2024-01-02 03:04", "-rw-r--r--")
			p.entries[0].dir, p.entries[0].mod = true, time.Time{}
			p.fillRow(0, r)
			assertUIText(t, r, "<DIR>")
			right := content.Objects[2].(*fyne.Container)
			if right.Objects[1].(*fyne.Container).Objects[0].(*widget.Label).Text != "" {
				t.Fatal("recycled row retained old modification date")
			}
		}
	}
}
