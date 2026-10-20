//go:build !darwin || !cgo

package desktop

func ContextMenu(window uintptr, items []ContextItem, x, y float32) int { return -1 }

func ChooseApplication(window uintptr) string { return "" }
