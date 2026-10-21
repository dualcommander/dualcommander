//go:build darwin && cgo

package desktop

/*
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
void hideServicesMenu(void);
void fixUnmodifiedMenuShortcuts(const char *titles);
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"fyne.io/fyne/v2"
)

// FixMenuShortcuts clears Cocoa's default Command modifier only for shortcuts
// explicitly defined without modifiers. Future native menu rebuilds are covered.
func FixMenuShortcuts(menu *fyne.MainMenu) {
	var titles []string
	var visit func(*fyne.Menu)
	visit = func(menu *fyne.Menu) {
		for _, item := range menu.Items {
			if shortcut, ok := item.Shortcut.(fyne.KeyboardShortcut); ok && shortcut.Mod() == 0 {
				titles = append(titles, item.Label)
			}
			if item.ChildMenu != nil {
				visit(item.ChildMenu)
			}
		}
	}
	for _, child := range menu.Items {
		visit(child)
	}
	data, _ := json.Marshal(titles)
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	C.fixUnmodifiedMenuShortcuts(text)
}

// HideServicesMenu removes unrelated macOS Services from the application menu.
// Call after the desktop driver has started and created its native menus.
func HideServicesMenu() { C.hideServicesMenu() }
