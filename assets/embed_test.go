package assets

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIconTransparency(t *testing.T) {
	icon, err := png.Decode(bytes.NewReader(IconPNG))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]int{{0, 0}, {100, 512}, {512, 100}, {512, 512}, {924, 512}, {512, 924}} {
		if _, _, _, alpha := icon.At(point[0], point[1]).RGBA(); alpha != 0 {
			t.Errorf("background at %v is not transparent", point)
		}
	}
	for _, x := range []int{300, 700} {
		if _, _, _, alpha := icon.At(x, 512).RGBA(); alpha != 0xffff {
			t.Errorf("pane at (%d, 512) is not opaque", x)
		}
	}
}
