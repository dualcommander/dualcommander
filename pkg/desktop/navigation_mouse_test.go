//go:build !ci

package desktop

import (
	"reflect"
	"testing"

	"github.com/go-gl/glfw/v3.4/glfw"
)

func TestNavigationMouseCallback(t *testing.T) {
	var steps []int
	var passed []glfw.MouseButton
	previous := func(_ *glfw.Window, button glfw.MouseButton, _ glfw.Action, _ glfw.ModifierKey) {
		passed = append(passed, button)
	}
	callback := navigationMouseCallback(previous, func(step int) { steps = append(steps, step) })
	for _, button := range []glfw.MouseButton{glfw.MouseButton1, glfw.MouseButton2, glfw.MouseButton3, glfw.MouseButton4, glfw.MouseButton5, glfw.MouseButton6} {
		callback(nil, button, glfw.Press, glfw.ModControl)
		callback(nil, button, glfw.Release, glfw.ModControl)
	}
	if !reflect.DeepEqual(steps, []int{-1, 1}) {
		t.Fatalf("navigation must fire once on press: %v", steps)
	}
	want := []glfw.MouseButton{glfw.MouseButton1, glfw.MouseButton1, glfw.MouseButton2, glfw.MouseButton2, glfw.MouseButton3, glfw.MouseButton3, glfw.MouseButton6, glfw.MouseButton6}
	if !reflect.DeepEqual(passed, want) {
		t.Fatalf("normal mouse events changed: %v", passed)
	}
}
