//go:build !darwin || !cgo

package desktop

import "fyne.io/fyne/v2"

// FixMenuShortcuts is unnecessary outside native macOS menus.
func FixMenuShortcuts(menu *fyne.MainMenu) {}

// HideServicesMenu is unnecessary outside native macOS menus.
func HideServicesMenu() {}
