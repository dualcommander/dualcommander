package commander

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestWindowsLicenses(t *testing.T) {
	if windowsLicenses("darwin", nil) != nil || windowsLicenses("linux", nil) != nil {
		t.Fatal("Windows runtime notices must be Windows-specific")
	}
	w := test.NewWindow(nil)
	defer w.Close()
	a := windowsLicenses("windows", w).(*fyne.Container)
	if len(a.Objects) < 14 {
		t.Fatal("missing bundled license terms")
	}
	var terms string
	for i := 0; i < len(a.Objects); i += 2 {
		label := a.Objects[i+1].(*widget.Label)
		terms += label.Text
		if label.Visible() {
			t.Fatal("license should start collapsed")
		}
		test.Tap(a.Objects[i].(*dialogButton))
		if !label.Visible() {
			t.Fatal("license did not expand")
		}
		test.Tap(a.Objects[i].(*dialogButton))
		if label.Visible() {
			t.Fatal("license did not collapse")
		}
	}
	for _, required := range []string{"Mesa", "LLVM", "Permission is hereby granted", "Apache License", "SGI", "Boost Software License"} {
		if !strings.Contains(terms, required) {
			t.Errorf("missing license text: %s", required)
		}
	}
}

func TestWindowsLicensesEscape(t *testing.T) {
	u := newTestUI(t, test.NewTempApp(t), t.TempDir(), t.TempDir())
	open := func() fyne.Window {
		w := u.dialogWindow("Windows licenses")
		w.SetContent(windowsLicenses("windows", w))
		w.Show()
		return w
	}
	w := open()
	count := len(dialogFocusables(w))
	w.Close()
	if count == 0 {
		t.Fatal("missing license controls")
	}
	for i := range count {
		w = open()
		controls := dialogFocusables(w)
		w.Canvas().Focus(controls[i])
		controls[i].TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		if hasWindow(u.app, "Windows licenses") {
			w.Close()
			t.Errorf("license control %d (%T) did not close dialog on Escape", i, controls[i])
		}
	}
}
