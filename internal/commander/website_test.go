package commander

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// MANUAL_SCREENSHOTS writes full app-rendered captures for the online manual.
func TestManualScreenshots(t *testing.T) {
	output := os.Getenv("MANUAL_SCREENSHOTS")
	if output == "" {
		t.Skip("set MANUAL_SCREENSHOTS to the website screenshots directory")
	}
	root := fixture(t)
	if err := os.Rename(filepath.Join(root, "src", "song.mp3"), filepath.Join(root, "src", "notes.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dst", "report.csv"), []byte("Sample report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := test.NewTempApp(t)
	u := newTestUI(t, a, filepath.Join(root, "src"), filepath.Join(root, "dst"))
	applyTheme(a, "light", u.density)
	u.left.setBrief(true)
	u.right.setBrief(false)
	u.win.Resize(fyne.NewSize(1200, 560))
	cursorTo(t, u.left, "readme.md")
	u.left.toggleMark()
	u.left.pathBar.SetText("/Users/demo/Documents")
	u.right.pathBar.SetText("/Users/demo/Backup")
	for _, p := range []*pane{u.left, u.right} {
		p.spaceGeneration++ // discard pending disk-space updates for fixture paths
		p.space.SetText("128 GiB free")
	}
	write := func(name string, w fyne.Window) {
		t.Helper()
		f, err := os.Create(filepath.Join(output, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, w.Canvas().Capture())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("write screenshot: %v; close: %v", err, closeErr)
		}
	}
	write("manual-panes", u.win)
	u.settings()
	windows := a.Driver().AllWindows()
	w := windows[len(windows)-1]
	if w.Title() != "Settings" {
		t.Fatalf("expected Settings window, got %q", w.Title())
	}
	// Show the whole scrollable form; normal desktop windows may need scrolling.
	w.Resize(fyne.NewSize(900, 800))
	assertUIText(t, w.Content(), "Appearance", "File views", "File deletion", "Archives", "External applications")
	write("manual-settings", w)
}

// WEBSITE_SCREENSHOT writes overlapping full canvases in both themes.
func TestWebsiteScreenshot(t *testing.T) {
	root := fixture(t)
	dir := filepath.Join(root, "src")
	if err := os.Remove(filepath.Join(dir, "song.mp3")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"design.pdf", "notes.txt", "report.csv", "backup.tar.gz", "presentation.pdf", "budget.csv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Sample document\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Date(2024, 1, 2, 3, 4, 0, 0, time.Local)
	if err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, stamp, stamp)
	}); err != nil {
		t.Fatal(err)
	}
	a := test.NewTempApp(t)
	u := newTestUI(t, a, dir, dir)
	u.left.setBrief(true)
	u.right.setBrief(false)
	for _, p := range []*pane{u.left, u.right} {
		for _, entry := range p.entries {
			if filepath.Ext(entry.name) == ".mp3" {
				t.Fatal("website preview must not contain MP3 files")
			}
		}
	}
	u.win.Resize(fyne.NewSize(1200, 560))
	cursorTo(t, u.left, "readme.md")
	u.left.toggleMark()
	cursorTo(t, u.right, "report.csv")
	// Only displayed paths change; sample files remain isolated in the test directory.
	u.left.pathBar.SetText("/Users/demo/Projects")
	u.right.pathBar.SetText("/Users/demo/Projects")
	u.left.space.SetText("128 GiB free")
	u.right.space.SetText("128 GiB free")
	var combined *image.RGBA
	for i, mode := range []string{"light", "dark"} {
		applyTheme(a, mode, u.density)
		capture := u.win.Canvas().Capture()
		bounds := capture.Bounds()
		if bounds.Dx() != 1200 || bounds.Dy() != 560 || !u.left.brief || u.right.brief {
			t.Fatal("preview must include full window and mixed pane views")
		}
		if i == 0 {
			combined = image.NewRGBA(image.Rect(0, 0, 1380, 660))
		}
		position := image.Pt(20+i*140, 20+i*60)
		frame := bounds.Add(position)
		shadow := image.NewUniform(color.NRGBA{A: 24})
		for spread := 12; spread > 0; spread -= 3 {
			draw.Draw(combined, frame.Add(image.Pt(0, 6)).Inset(-spread), shadow, image.Point{}, draw.Over)
		}
		draw.Draw(combined, frame, capture, bounds.Min, draw.Src)
	}
	if combined.Bounds().Dx() != 1380 || combined.Bounds().Dy() != 660 || combined.RGBAAt(0, 0).A != 0 {
		t.Fatal("preview must preserve overlapping composition and transparent margins")
	}
	if output := os.Getenv("WEBSITE_SCREENSHOT"); output != "" {
		f, err := os.Create(output)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, combined); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
