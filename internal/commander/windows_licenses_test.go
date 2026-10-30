package commander

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

func TestWindowsLicenses(t *testing.T) {
	if windowsLicenses("darwin") != nil || windowsLicenses("linux") != nil {
		t.Fatal("Windows runtime notices must be Windows-specific")
	}
	a := windowsLicenses("windows").(*widget.Accordion)
	if len(a.Items) < 7 {
		t.Fatal("missing bundled license terms")
	}
	var terms string
	for _, item := range a.Items {
		terms += item.Detail.(*widget.Label).Text
	}
	for _, required := range []string{"Mesa", "LLVM", "Permission is hereby granted", "Apache License", "SGI", "Boost Software License"} {
		if !strings.Contains(terms, required) {
			t.Errorf("missing license text: %s", required)
		}
	}
}
