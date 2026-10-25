//go:build darwin && cgo

package desktop

import (
	"os"
	"strings"
	"testing"
)

func TestMacOSModalUsesFrameworkBehavior(t *testing.T) {
	source, err := os.ReadFile("modal_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), `import "C"`) {
		t.Fatal("macOS modal blocking must not override native window activation through cgo")
	}
	if _, err := os.Stat("modal_darwin.m"); !os.IsNotExist(err) {
		t.Fatal("macOS modal activation workaround must be removed")
	}
}
