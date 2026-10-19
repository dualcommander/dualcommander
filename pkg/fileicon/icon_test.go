//go:build darwin && cgo

package fileicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemIcons(t *testing.T) {
	for _, tc := range []struct {
		extension string
		folder    bool
	}{{"", true}, {"", false}, {"txt", false}, {"zip", false}, {"unknown-extension", false}} {
		data := PNG(tc.extension, tc.folder)
		image, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("extension=%q folder=%v: %v", tc.extension, tc.folder, err)
		}
		if image.Bounds().Dx() < 16 || image.Bounds().Dy() < 16 {
			t.Fatalf("icon too small: %v", image.Bounds())
		}
		if !bytes.Equal(data, PNG(tc.extension, tc.folder)) {
			t.Fatal("cached icon changed")
		}
	}
}

func TestFilePreview(t *testing.T) {
	name := filepath.Join(t.TempDir(), "preview.png")
	im := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			im.Set(x, y, color.RGBA{R: 220, G: 40, B: 70, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	data := Preview(name)
	preview, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("native preview: %v", err)
	}
	if preview.Bounds().Dx() < 16 || preview.Bounds().Dy() < 16 {
		t.Fatalf("preview too small: %v", preview.Bounds())
	}
	red := 0
	for y := preview.Bounds().Min.Y; y < preview.Bounds().Max.Y; y++ {
		for x := preview.Bounds().Min.X; x < preview.Bounds().Max.X; x++ {
			r, g, b, _ := preview.At(x, y).RGBA()
			if r > 40000 && g < 20000 && b < 25000 {
				red++
			}
		}
	}
	if red == 0 {
		t.Fatal("image preview lacks source image pixels")
	}
	if len(Preview(filepath.Join(t.TempDir(), "missing.png"))) != 0 {
		t.Fatal("missing file returned preview")
	}
}
