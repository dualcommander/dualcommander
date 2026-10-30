package commander

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	windowsruntime "github.com/dualcommander/dualcommander/packaging/windows"
)

func windowsLicenses(platform string) fyne.CanvasObject {
	if platform != "windows" {
		return nil
	}
	items := []*widget.AccordionItem{}
	entries, _ := windowsruntime.Licenses.ReadDir("mesa") // Embedded, build-checked directory.
	for _, entry := range entries {
		text, _ := windowsruntime.Licenses.ReadFile("mesa/" + entry.Name())
		label := widget.NewLabel(string(text))
		label.Wrapping = fyne.TextWrapWord
		items = append(items, widget.NewAccordionItem("Windows: "+strings.TrimSuffix(entry.Name(), ".txt"), label))
	}
	return widget.NewAccordion(items...)
}
