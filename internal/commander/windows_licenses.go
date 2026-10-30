package commander

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	windowsruntime "github.com/dualcommander/dualcommander/packaging/windows"
)

func windowsLicenses(platform string, w fyne.Window) fyne.CanvasObject {
	if platform != "windows" {
		return nil
	}
	content := container.NewVBox()
	entries, _ := windowsruntime.Licenses.ReadDir("mesa") // Embedded, build-checked directory.
	for _, entry := range entries {
		text, _ := windowsruntime.Licenses.ReadFile("mesa/" + entry.Name())
		label := widget.NewLabel(string(text))
		label.Wrapping = fyne.TextWrapWord
		label.Hide()
		content.Add(windowButton(w, "Windows: "+strings.TrimSuffix(entry.Name(), ".txt"), func() {
			if label.Visible() {
				label.Hide()
			} else {
				label.Show()
			}
		}))
		content.Add(label)
	}
	return content
}
